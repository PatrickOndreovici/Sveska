import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadPdf, type PdfDocument } from "../../lib/pdf/pdf";
import { mockApi, student } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

// PDF.js cannot run in jsdom; the viewer is tested against a fake document
// that records which page was drawn at which zoom.
vi.mock("../../lib/pdf/pdf", () => ({ loadPdf: vi.fn() }));

const renders: { page: number; scale: number }[] = [];
const destroy = vi.fn();

function fakePdf(pageCount: number): PdfDocument {
  return {
    pageCount,
    renderPage: (page, _canvas, scale) => {
      renders.push({ page, scale });
      return { promise: Promise.resolve(), cancel: () => undefined };
    },
    destroy,
  };
}

const lastRender = () => renders.at(-1);

beforeEach(() => {
  renders.length = 0;
  destroy.mockClear();
  vi.mocked(loadPdf).mockReturnValue({ promise: Promise.resolve(fakePdf(3)), cancel: () => undefined });
});

function renderDocument() {
  mockApi({
    "GET /api/me": { status: 200, body: { user: student } },
    "GET /api/documents/d1": {
      status: 200,
      body: {
        document: { id: "d1", lessonId: "l1", title: "Čitanka", byteSize: 100, createdAt: "2026-10-23T10:00:00Z" },
        lesson: { id: "l1", title: "Lekcija 1" },
      },
    },
  });
  return renderApp("/documents/d1");
}

describe("DocumentPage", () => {
  it("shows the title, the way back and the first page", async () => {
    renderDocument();

    expect(await screen.findByRole("heading", { level: 1, name: "Čitanka" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "← Lekcija 1" })).toHaveAttribute("href", "/lessons/l1");
    expect(screen.getByRole("link", { name: "Download" })).toHaveAttribute("href", "/api/documents/d1/file");
    expect(await screen.findByRole("region", { name: "Čitanka, page 1 of 3" })).toBeInTheDocument();
    expect(loadPdf).toHaveBeenCalledWith("/api/documents/d1/file");
    expect(lastRender()).toEqual({ page: 1, scale: 1 });
  });

  it("turns pages with the buttons, the page box and the arrow keys", async () => {
    renderDocument();
    const next = await screen.findByRole("button", { name: "Next" });
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();

    await userEvent.click(next);
    expect(lastRender()?.page).toBe(2);
    expect(screen.getByLabelText("Page number")).toHaveValue("2");

    await userEvent.clear(screen.getByLabelText("Page number"));
    await userEvent.type(screen.getByLabelText("Page number"), "3");
    await waitFor(() => {
      expect(lastRender()?.page).toBe(3);
    });
    expect(next).toBeDisabled();

    screen.getByRole("region", { name: /page 3 of 3/ }).focus();
    await userEvent.keyboard("{ArrowLeft}");
    expect(lastRender()?.page).toBe(2);
  });

  it("ignores page numbers outside the document", async () => {
    renderDocument();
    const box = await screen.findByLabelText("Page number");

    await userEvent.clear(box);
    await userEvent.type(box, "9");
    expect(lastRender()?.page).toBe(1);

    await userEvent.tab();
    expect(box).toHaveValue("1");
  });

  it("zooms in and out in steps", async () => {
    renderDocument();
    const zoomIn = await screen.findByRole("button", { name: "Zoom in" });

    await userEvent.click(zoomIn);
    expect(lastRender()?.scale).toBe(1.25);
    expect(screen.getByLabelText("Zoom")).toHaveTextContent("125%");

    await userEvent.click(screen.getByRole("button", { name: "Zoom out" }));
    await userEvent.click(screen.getByRole("button", { name: "Zoom out" }));
    expect(lastRender()?.scale).toBe(0.75);
    expect(screen.getByLabelText("Zoom")).toHaveTextContent("75%");
  });

  it("explains when the PDF cannot be opened", async () => {
    // Created on call, so the rejection is never left unhandled while the page loads.
    vi.mocked(loadPdf).mockImplementation(() => ({
      promise: Promise.reject(new Error("Invalid PDF structure")),
      cancel: () => undefined,
    }));
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    renderDocument();

    expect(await screen.findByRole("alert")).toHaveTextContent("This PDF could not be opened.");
  });

  it("frees the PDF when leaving the page", async () => {
    const router = renderDocument();
    await screen.findByRole("region", { name: /page 1 of 3/ });

    await router.navigate("/");

    await waitFor(() => {
      expect(destroy).toHaveBeenCalled();
    });
  });
});
