import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { LessonDocument } from "../../lib/api/types";
import { imageFileToPdf } from "../../lib/pdf/raster";
import { mockApi, student, type MockHandler } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";
import { lesson } from "./LessonsPage.test";

// jsdom cannot decode images or draw on a canvas; the conversion has its own tests.
vi.mock("../../lib/pdf/raster", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/pdf/raster")>()),
  imageFileToPdf: vi.fn(),
}));

function doc(id: string, title: string, byteSize = 1_258_291): LessonDocument {
  return { id, lessonId: "l1", title, byteSize, createdAt: "2026-10-23T10:00:00Z" };
}

const citanka = doc("d1", "Čitanka");

/**
 * A fake API for one lesson: GET answers with the current documents, and
 * upload and delete change them, so reloads show the effect.
 */
function renderLesson(initial: LessonDocument[], overrides: Record<string, MockHandler> = {}) {
  let documents = [...initial];
  const fetchSpy = mockApi({
    "GET /api/me": { status: 200, body: { user: student } },
    "GET /api/lessons/l1": () => ({
      status: 200,
      body: { lesson: lesson("l1", "Lekcija 1", { description: "Padeži", documentCount: documents.length }), documents },
    }),
    "POST /api/lessons/l1/documents": () => {
      const added = doc("d2", "Glagoli", 2048);
      documents = [...documents, added];
      return { status: 201, body: { document: added } };
    },
    "DELETE /api/documents/d1": () => {
      documents = documents.filter((d) => d.id !== "d1");
      return { status: 204 };
    },
    ...overrides,
  });
  const router = renderApp("/lessons/l1");
  return { fetchSpy, router };
}

function pdf(name: string, size = 100): File {
  const file = new File(["%PDF-1.7"], name, { type: "application/pdf" });
  Object.defineProperty(file, "size", { value: size });
  return file;
}

describe("LessonPage", () => {
  it("shows the lesson and its PDFs", async () => {
    renderLesson([citanka]);

    expect(await screen.findByRole("heading", { level: 1, name: "Lekcija 1" })).toBeInTheDocument();
    expect(screen.getByText("Padeži")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "Čitanka" });
    expect(link).toHaveAttribute("href", "/documents/d1");
    expect(screen.getByText("1.2 MB · 23 Oct 2026")).toBeInTheDocument();
  });

  it("uploads a chosen PDF and lists it", async () => {
    const { fetchSpy } = renderLesson([citanka]);
    const input = await screen.findByLabelText("Add a PDF or a picture");

    await userEvent.upload(input, pdf("Glagoli.pdf"));

    expect(await screen.findByRole("link", { name: "Glagoli" })).toBeInTheDocument();
    expect(screen.getByText("Added Glagoli.")).toHaveAttribute("role", "status");
    const call = fetchSpy.mock.calls.find(([url]) => url === "/api/lessons/l1/documents");
    const body = call?.[1]?.body;
    expect(body).toBeInstanceOf(FormData);
    expect((body as FormData).get("file")).toBeInstanceOf(File);
  });

  it("makes a photo into a PDF before uploading it", async () => {
    const converted = pdf("IMG_2265.pdf");
    vi.mocked(imageFileToPdf).mockResolvedValueOnce(converted);
    const { fetchSpy } = renderLesson([]);
    const photo = new File(["jpeg bytes"], "IMG_2265.jpeg", { type: "image/jpeg" });

    await userEvent.upload(await screen.findByLabelText("Add a PDF or a picture"), photo);

    expect(await screen.findByText("Added Glagoli.")).toBeInTheDocument();
    expect(imageFileToPdf).toHaveBeenCalledWith(photo);
    const call = fetchSpy.mock.calls.find(([url]) => url === "/api/lessons/l1/documents");
    expect((call?.[1]?.body as FormData).get("file")).toBe(converted);
  });

  it("says when a picture cannot be opened, without uploading", async () => {
    vi.mocked(imageFileToPdf).mockRejectedValueOnce(new Error("cannot decode"));
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    const { fetchSpy } = renderLesson([]);

    await userEvent.upload(
      await screen.findByLabelText("Add a PDF or a picture"),
      new File(["?"], "scan.heic", { type: "image/heic" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent("This picture could not be opened. Try a JPEG or PNG.");
    expect(fetchSpy.mock.calls.some(([url]) => url === "/api/lessons/l1/documents")).toBe(false);
  });

  it("refuses a PDF over 20 MB without uploading it", async () => {
    const { fetchSpy } = renderLesson([]);
    const input = await screen.findByLabelText("Add a PDF or a picture");

    await userEvent.upload(input, pdf("huge.pdf", 21 * 1024 * 1024));

    expect(await screen.findByRole("alert")).toHaveTextContent("Choose a PDF of at most 20 MB.");
    expect(fetchSpy.mock.calls.some(([url]) => url === "/api/lessons/l1/documents")).toBe(false);
  });

  it("shows the API's reason when an upload is refused", async () => {
    renderLesson([], {
      "POST /api/lessons/l1/documents": {
        status: 422,
        body: {
          error: { code: "validation_failed", message: "Some fields are missing or invalid.", fields: { file: "Choose a PDF file." } },
        },
      },
    });

    await userEvent.upload(await screen.findByLabelText("Add a PDF or a picture"), pdf("fake.pdf"));

    expect(await screen.findByRole("alert")).toHaveTextContent("Choose a PDF file.");
    expect(screen.getByLabelText("Add a PDF or a picture")).toHaveAttribute("aria-invalid", "true");
  });

  it("deletes a PDF after confirming", async () => {
    renderLesson([citanka]);

    await userEvent.click(await screen.findByRole("button", { name: "Delete Čitanka" }));
    const dialog = within(screen.getByRole("dialog", { name: "Delete this PDF?" }));
    await userEvent.click(dialog.getByRole("button", { name: "Delete" }));

    expect(await screen.findByText("No PDFs in this lesson yet.")).toBeInTheDocument();
  });

  it("deletes the lesson and returns to the list", async () => {
    const { router } = renderLesson([citanka], {
      "DELETE /api/lessons/l1": { status: 204 },
      "GET /api/lessons": { status: 200, body: { lessons: [] } },
    });

    await userEvent.click(await screen.findByRole("button", { name: "Delete lesson" }));
    const dialog = within(screen.getByRole("dialog", { name: "Delete this lesson?" }));
    expect(dialog.getByText(/and its 1 PDF will be deleted/)).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/lessons");
    });
  });

  it("edits the title and description", async () => {
    let title = "Lekcija 1";
    const { fetchSpy } = renderLesson([], {
      "GET /api/lessons/l1": () => ({ status: 200, body: { lesson: lesson("l1", title), documents: [] } }),
      "PUT /api/lessons/l1": (body) => {
        title = (body as { title: string }).title;
        return { status: 200, body: { lesson: lesson("l1", title) } };
      },
    });

    await userEvent.click(await screen.findByRole("button", { name: "Edit lesson" }));
    const form = within(screen.getByRole("form", { name: "Edit lesson" }));
    await userEvent.clear(form.getByLabelText("Title"));
    await userEvent.type(form.getByLabelText("Title"), "Lekcija 2");
    await userEvent.click(form.getByRole("button", { name: "Save" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Lekcija 2" })).toBeInTheDocument();
    const call = fetchSpy.mock.calls.find(([, init]) => init?.method === "PUT");
    expect(JSON.parse(call?.[1]?.body as string)).toEqual({ title: "Lekcija 2", description: "" });
  });

  it("says when the lesson does not exist", async () => {
    mockApi({
      "GET /api/me": { status: 200, body: { user: student } },
      "GET /api/lessons/missing": {
        status: 404,
        body: { error: { code: "lesson_not_found", message: "This lesson does not exist." } },
      },
    });
    renderApp("/lessons/missing");

    expect(await screen.findByRole("alert")).toHaveTextContent("This lesson does not exist.");
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "← All lessons" })).toHaveAttribute("href", "/lessons");
  });
});
