import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Annotation, Stroke, StrokeInput, TextBox, TextBoxInput } from "../../lib/api/types";
import { loadPdf, type PdfDocument } from "../../lib/pdf/pdf";
import { readSelection, type PageSelection } from "../../lib/pdf/selection";
import { mockApi, student, type MockHandler } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";
import { exportDocument } from "./exportDocument";

// PDF.js cannot run in jsdom; the viewer is tested against a fake document
// that records which page was drawn at which zoom.
vi.mock("../../lib/pdf/pdf", () => ({ loadPdf: vi.fn() }));
// jsdom has no layout, so a text selection has no rectangles. Tests set
// what readSelection reports instead; the geometry has its own tests.
vi.mock("../../lib/pdf/selection", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/pdf/selection")>()),
  readSelection: vi.fn(() => null),
  clearSelection: vi.fn(),
}));
// Exporting draws on canvases, which jsdom lacks; the page is tested up to
// the request it makes, and the drawing has its own tests.
vi.mock("./exportDocument", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./exportDocument")>()),
  exportDocument: vi.fn(),
}));

const renders: { page: number; scale: number }[] = [];
const destroy = vi.fn();

function fakePdf(pageCount: number): PdfDocument {
  return {
    pageCount,
    renderPage: (page, _layers, scale) => {
      renders.push({ page, scale });
      return { promise: Promise.resolve(), cancel: () => undefined };
    },
    rasterizePage: () => Promise.reject(new Error("not in jsdom")),
    destroy,
  };
}

const lastRender = () => renders.at(-1);

beforeEach(() => {
  renders.length = 0;
  destroy.mockClear();
  vi.mocked(loadPdf).mockReturnValue({
    promise: Promise.resolve(fakePdf(3)),
    cancel: () => undefined,
  });
});

function annotation(id: string, pageNumber: number, quote: string, note = ""): Annotation {
  return {
    id,
    documentId: "d1",
    pageNumber,
    rects: [{ x: 0.1, y: 0.2, width: 0.5, height: 0.02 }],
    quote,
    note,
    createdAt: `2026-10-23T10:00:0${String(pageNumber)}Z`,
    updatedAt: "2026-10-23T10:00:00Z",
  };
}

const kuci = annotation("a1", 2, "Idem kući.", "going home");
const zdravo = annotation("a2", 1, "Zdravo");

/**
 * A fake API for one document: annotations live in memory, so creating,
 * editing and deleting them shows in what the page renders next.
 */
function renderDocument(
  path = "/documents/d1",
  // In reading order, as the API lists them.
  initial: Annotation[] = [zdravo, kuci],
  overrides: Record<string, MockHandler> = {},
) {
  let annotations = [...initial];
  let created = 0;
  const fetchSpy = mockApi({
    "GET /api/me": { status: 200, body: { user: student } },
    "GET /api/documents/d1": {
      status: 200,
      body: {
        document: {
          id: "d1",
          lessonId: "l1",
          title: "Čitanka",
          byteSize: 100,
          createdAt: "2026-10-23T10:00:00Z",
        },
        lesson: { id: "l1", title: "Lekcija 1" },
      },
    },
    "GET /api/documents/d1/annotations": () => ({
      status: 200,
      body: { annotations },
    }),
    "POST /api/documents/d1/annotations": (body) => {
      created++;
      const input = body as Pick<Annotation, "pageNumber" | "rects" | "quote" | "note">;
      const a = {
        ...annotation(`new-${String(created)}`, input.pageNumber, input.quote, input.note),
        rects: input.rects,
      };
      annotations = [...annotations, a];
      return { status: 201, body: { annotation: a } };
    },
    "PUT /api/annotations/a1": (body) => {
      const updated = { ...kuci, note: (body as { note: string }).note };
      return { status: 200, body: { annotation: updated } };
    },
    "DELETE /api/annotations/a1": () => {
      annotations = annotations.filter((a) => a.id !== "a1");
      return { status: 204 };
    },
    "GET /api/documents/d1/strokes": { status: 200, body: { strokes: [] } },
    "GET /api/documents/d1/text-boxes": { status: 200, body: { textBoxes: [] } },
    ...overrides,
  });
  const router = renderApp(path);
  return { fetchSpy, router };
}

const selected: PageSelection = {
  quote: "kuća",
  rects: [{ x: 0.2, y: 0.4, width: 0.1, height: 0.02 }],
};

/** Makes as if the reader dragged across text on the page shown. */
async function selectText(selection = selected) {
  vi.mocked(readSelection).mockReturnValueOnce(selection);
  fireEvent.pointerUp(await screen.findByTestId("pdf-page"));
  return within(await screen.findByRole("toolbar", { name: "Selected text" }));
}

function panel() {
  return within(screen.getByRole("region", { name: "Highlights" }));
}

/** The panel once the page has loaded. */
async function findPanel() {
  return within(await screen.findByRole("region", { name: "Highlights" }));
}

function calls(fetchSpy: ReturnType<typeof mockApi>, method: string, url: string) {
  return fetchSpy.mock.calls.filter(([u, init]) => u === url && init?.method === method);
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
    const { router } = renderDocument();
    const next = await screen.findByRole("button", { name: "Next" });
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();

    await userEvent.click(next);
    expect(lastRender()?.page).toBe(2);
    expect(screen.getByLabelText("Page number")).toHaveValue("2");
    expect(router.state.location.search).toBe("?page=2");

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

  it("opens on the page in the link", async () => {
    renderDocument("/documents/d1?page=2");
    expect(await screen.findByRole("region", { name: "Čitanka, page 2 of 3" })).toBeInTheDocument();
  });

  it("moves a page past the end to the last page", async () => {
    const { router } = renderDocument("/documents/d1?page=9");
    expect(await screen.findByRole("region", { name: "Čitanka, page 3 of 3" })).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.search).toBe("?page=3");
    });
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
    const { router } = renderDocument();
    await screen.findByRole("region", { name: /page 1 of 3/ });

    await router.navigate("/");

    await waitFor(() => {
      expect(destroy).toHaveBeenCalled();
    });
  });

  it("lists the highlights by page and shows one on its page", async () => {
    const { router } = renderDocument();
    await screen.findByRole("heading", { level: 3, name: "Page 1" });

    const pages = panel().getAllByRole("heading", { level: 3 });
    expect(pages.map((h) => h.textContent)).toEqual(["Page 1", "Page 2"]);
    expect(panel().getByText("going home")).toBeInTheDocument();

    const show = panel().getByRole("button", {
      name: /Show on page 2: Idem kući\./,
    });
    await userEvent.click(show);

    expect(show).toHaveAttribute("aria-current", "true");
    expect(await screen.findByRole("region", { name: "Čitanka, page 2 of 3" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("?page=2");
  });

  it("invites a first highlight when there is none", async () => {
    renderDocument("/documents/d1", []);
    expect(await screen.findByText(/Select text on the page to highlight it/)).toBeInTheDocument();
  });

  it("highlights selected text", async () => {
    const { fetchSpy } = renderDocument();
    const toolbar = await selectText();

    await userEvent.click(toolbar.getByRole("button", { name: "Highlight" }));

    const show = await panel().findByRole("button", {
      name: /Show on page 1: kuća/,
    });
    expect(show).toHaveAttribute("aria-current", "true");
    expect(screen.queryByRole("toolbar", { name: "Selected text" })).not.toBeInTheDocument();
    const [call] = calls(fetchSpy, "POST", "/api/documents/d1/annotations");
    expect(JSON.parse(call?.[1]?.body as string)).toEqual({
      pageNumber: 1,
      ...selected,
      note: "",
    });
  });

  it("adds a note to new and existing highlights", async () => {
    const { fetchSpy } = renderDocument();
    const toolbar = await selectText();

    await userEvent.click(toolbar.getByRole("button", { name: "Add note" }));
    expect(await panel().findByLabelText("Note")).toHaveFocus();
    await userEvent.click(panel().getByRole("button", { name: "Cancel" }));

    await userEvent.click(panel().getByRole("button", { name: "Edit the note on “Idem kući.”" }));
    const note = panel().getByLabelText("Note");
    expect(note).toHaveValue("going home");
    await userEvent.clear(note);
    await userEvent.type(note, "I am going home");
    await userEvent.click(panel().getByRole("button", { name: "Save note" }));

    expect(await panel().findByText("I am going home")).toBeInTheDocument();
    const [call] = calls(fetchSpy, "PUT", "/api/annotations/a1");
    expect(JSON.parse(call?.[1]?.body as string)).toEqual({
      note: "I am going home",
    });
  });

  it("saves a selected word to the vocabulary, linked to a new highlight", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], {
      "POST /api/vocabulary": (body) => ({
        status: 201,
        body: { entry: body },
      }),
    });
    const toolbar = await selectText();

    await userEvent.click(toolbar.getByRole("button", { name: "Save word" }));
    const dialog = within(screen.getByRole("dialog", { name: "Save to vocabulary" }));
    expect(dialog.getByLabelText("Serbian")).toHaveValue("kuća");
    await userEvent.type(dialog.getByLabelText("Translation"), "house");
    await userEvent.click(dialog.getByRole("button", { name: "Save word" }));

    expect(await screen.findByText(/Saved/)).toHaveTextContent("Saved kuća to your vocabulary.");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(panel().getByRole("button", { name: /Show on page 1: kuća/ })).toBeInTheDocument();
    const [word] = calls(fetchSpy, "POST", "/api/vocabulary");
    expect(JSON.parse(word?.[1]?.body as string)).toMatchObject({
      term: "kuća",
      translation: "house",
      sourceAnnotationId: "new-1",
    });
  });

  it("keeps one highlight when a refused word is saved again", async () => {
    let attempts = 0;
    const { fetchSpy } = renderDocument("/documents/d1", [], {
      "POST /api/vocabulary": (body) =>
        attempts++ === 0
          ? {
              status: 409,
              body: {
                error: {
                  code: "duplicate_entry",
                  message: "You already have this word with this translation.",
                },
              },
            }
          : { status: 201, body: { entry: body } },
    });
    const toolbar = await selectText();
    await userEvent.click(toolbar.getByRole("button", { name: "Save word" }));
    const dialog = within(screen.getByRole("dialog", { name: "Save to vocabulary" }));
    await userEvent.type(dialog.getByLabelText("Translation"), "house");

    await userEvent.click(dialog.getByRole("button", { name: "Save word" }));
    expect(await dialog.findByRole("alert")).toHaveTextContent("You already have this word");
    await userEvent.clear(dialog.getByLabelText("Translation"));
    await userEvent.type(dialog.getByLabelText("Translation"), "home");
    await userEvent.click(dialog.getByRole("button", { name: "Save word" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    expect(calls(fetchSpy, "POST", "/api/documents/d1/annotations")).toHaveLength(1);
  });

  it("asks for a shorter passage when the selection is too long", async () => {
    const { fetchSpy } = renderDocument();
    const toolbar = await selectText({ ...selected, quote: "a".repeat(2001) });

    expect(toolbar.getByText("Select a shorter passage to highlight it.")).toBeInTheDocument();
    expect(toolbar.queryByRole("button")).not.toBeInTheDocument();
    expect(calls(fetchSpy, "POST", "/api/documents/d1/annotations")).toHaveLength(0);
  });

  it("selects a highlight clicked on the page", async () => {
    renderDocument("/documents/d1?page=2");
    const page = await screen.findByTestId("pdf-page");
    await (await findPanel()).findByRole("heading", { level: 3, name: "Page 2" });
    // A 600×800 page at the top-left corner; the highlight covers x 0.1–0.6, y 0.2–0.22.
    vi.spyOn(page, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 600, 800));

    fireEvent.pointerUp(page, { clientX: 120, clientY: 168 });
    expect(panel().getByRole("button", { name: /Show on page 2/ })).toHaveAttribute("aria-current", "true");

    fireEvent.pointerUp(page, { clientX: 120, clientY: 400 });
    expect(panel().getByRole("button", { name: /Show on page 2/ })).not.toHaveAttribute("aria-current");
  });

  it("deletes a highlight after confirming", async () => {
    renderDocument();
    await userEvent.click(
      await (await findPanel()).findByRole("button", { name: "Delete the highlight “Idem kući.”" }),
    );
    const dialog = within(screen.getByRole("dialog", { name: "Delete this highlight?" }));
    expect(dialog.getByText(/Words you saved from it stay in your vocabulary/)).toBeInTheDocument();

    await userEvent.click(dialog.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(panel().queryByText("Idem kući.")).not.toBeInTheDocument();
    });
    expect(panel().getByText("Zdravo")).toBeInTheDocument();
  });

  it("still shows the PDF when the highlights cannot be loaded", async () => {
    let failures = 1;
    renderDocument("/documents/d1", [kuci], {
      "GET /api/documents/d1/annotations": () =>
        failures-- > 0
          ? {
              status: 500,
              body: {
                error: {
                  code: "internal_error",
                  message: "Something went wrong on our side.",
                },
              },
            }
          : { status: 200, body: { annotations: [kuci] } },
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong on our side.");
    expect(await screen.findByRole("region", { name: /page 1 of 3/ })).toBeInTheDocument();
    await userEvent.click(panel().getByRole("button", { name: "Try again" }));
    expect(await panel().findByText("Idem kući.")).toBeInTheDocument();
  });
});

const circled: Stroke = {
  id: "s1",
  documentId: "d1",
  pageNumber: 1,
  color: "red",
  points: [
    { x: 0.1, y: 0.1 },
    { x: 0.2, y: 0.1 },
  ],
  createdAt: "2026-10-23T10:00:00Z",
};

const answered: TextBox = {
  id: "t1",
  documentId: "d1",
  pageNumber: 1,
  x: 0.5,
  y: 0.25,
  text: "kuća",
  color: "blue",
  fontSize: 14,
  createdAt: "2026-10-23T10:00:00Z",
  updatedAt: "2026-10-23T10:00:00Z",
};

/** The drawing endpoints, starting with the given strokes and text; new ones get ids stroke-1, text-1 and so on. */
function drawingApi(strokes: Stroke[] = [], textBoxes: TextBox[] = []): Record<string, MockHandler> {
  let created = 0;
  return {
    "GET /api/documents/d1/strokes": { status: 200, body: { strokes } },
    "GET /api/documents/d1/text-boxes": { status: 200, body: { textBoxes } },
    "POST /api/documents/d1/strokes": (body) => {
      created++;
      const stroke = { ...(body as StrokeInput), id: `stroke-${String(created)}`, documentId: "d1", createdAt: "" };
      return { status: 201, body: { stroke } };
    },
    "POST /api/documents/d1/text-boxes": (body) => {
      created++;
      const textBox = { ...(body as TextBoxInput), id: `text-${String(created)}`, documentId: "d1" };
      return { status: 201, body: { textBox } };
    },
    "DELETE /api/strokes/s1": { status: 204 },
    "DELETE /api/strokes/stroke-1": { status: 204 },
    "PUT /api/text-boxes/t1": (body) => ({ status: 200, body: { textBox: { ...answered, ...(body as TextBoxInput) } } }),
    "DELETE /api/text-boxes/t1": { status: 204 },
  };
}

/** Picks a drawing tool once what is already drawn has loaded. */
async function pickTool(name: "Select" | "Pen" | "Text" | "Eraser") {
  const button = await screen.findByRole("button", { name });
  await waitFor(() => {
    expect(button).toBeEnabled();
  });
  await userEvent.click(button);
}

/** The drawing layer of the page shown, laid out as a 600×800 page at the top-left corner. */
async function drawingLayer() {
  const layer = await screen.findByTestId("drawing-layer");
  vi.spyOn(layer, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 600, 800));
  return layer;
}

/** Presses on the first point, moves through the others and lifts on the last. */
function drag(target: Element, layer: Element, points: [number, number][]) {
  const [[x, y] = [0, 0], ...rest] = points;
  fireEvent.pointerDown(target, { clientX: x, clientY: y, pointerId: 1 });
  for (const [mx, my] of rest) fireEvent.pointerMove(layer, { clientX: mx, clientY: my, pointerId: 1 });
  const [ux, uy] = rest.at(-1) ?? [x, y];
  fireEvent.pointerUp(layer, { clientX: ux, clientY: uy, pointerId: 1 });
}

function sent(fetchSpy: ReturnType<typeof mockApi>, method: string, url: string): unknown[] {
  return calls(fetchSpy, method, url).map(([, init]) => JSON.parse(init?.body as string) as unknown);
}

describe("DocumentPage drawing", () => {
  it("draws with the pen and saves the stroke", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Pen");
    await userEvent.click(screen.getByRole("button", { name: "Red" }));
    const layer = await drawingLayer();

    drag(layer, layer, [
      [60, 80],
      [120, 80],
      [120, 160],
    ]);

    await waitFor(() => {
      expect(sent(fetchSpy, "POST", "/api/documents/d1/strokes")).toEqual([
        {
          pageNumber: 1,
          color: "red",
          points: [
            { x: 0.1, y: 0.1 },
            { x: 0.2, y: 0.1 },
            { x: 0.2, y: 0.2 },
          ],
        },
      ]);
    });
    expect(layer.querySelectorAll("path[data-stroke]")).toHaveLength(1);
  });

  it("does not select text while the pen is in use", async () => {
    renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Pen");

    vi.mocked(readSelection).mockReturnValueOnce(selected);
    fireEvent.pointerUp(await screen.findByTestId("pdf-page"));

    expect(screen.queryByRole("toolbar", { name: "Selected text" })).not.toBeInTheDocument();
  });

  it("erases the strokes and text it passes over, and undoes that", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi([circled], [answered]));
    await pickTool("Eraser");
    const layer = await drawingLayer();
    const text = await screen.findByText("kuća");
    vi.spyOn(text, "getBoundingClientRect").mockReturnValue(new DOMRect(300, 200, 40, 18));

    // From a point on the stroke to one on the text.
    drag(layer, layer, [
      [90, 82],
      [310, 210],
    ]);

    await waitFor(() => {
      expect(calls(fetchSpy, "DELETE", "/api/strokes/s1")).toHaveLength(1);
    });
    expect(calls(fetchSpy, "DELETE", "/api/text-boxes/t1")).toHaveLength(1);
    expect(layer.querySelectorAll("path[data-stroke]")).toHaveLength(0);
    expect(screen.queryByText("kuća")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Undo" }));

    expect(await screen.findByText("kuća")).toBeInTheDocument();
    expect(layer.querySelectorAll("path[data-stroke]")).toHaveLength(1);
    await waitFor(() => {
      expect(sent(fetchSpy, "POST", "/api/documents/d1/text-boxes")).toEqual([
        { pageNumber: 1, x: 0.5, y: 0.25, text: "kuća", color: "blue", fontSize: 14 },
      ]);
    });
    expect(sent(fetchSpy, "POST", "/api/documents/d1/strokes")).toEqual([
      { pageNumber: 1, color: "red", points: circled.points },
    ]);
    expect(screen.getByRole("button", { name: "Undo" })).toBeDisabled();
  });

  it("types text where the page is clicked", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Text");
    await userEvent.click(screen.getByRole("button", { name: "Blue" }));
    const layer = await drawingLayer();

    drag(layer, layer, [[300, 400]]);
    const input = await screen.findByRole("textbox", { name: "Text on the page" });
    expect(input).toHaveFocus();
    await userEvent.type(input, "kuća{Enter}");

    expect(await screen.findByText("kuća")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "Text on the page" })).not.toBeInTheDocument();
    await waitFor(() => {
      // The first line is centred on the click: half a 17.5px line above it.
      expect(sent(fetchSpy, "POST", "/api/documents/d1/text-boxes")).toEqual([
        { pageNumber: 1, x: 0.5, y: 0.4891, text: "kuća", color: "blue", fontSize: 14 },
      ]);
    });
  });

  it("finishes the text when the page is pressed elsewhere", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Text");
    const layer = await drawingLayer();

    drag(layer, layer, [[300, 400]]);
    await userEvent.type(await screen.findByRole("textbox", { name: "Text on the page" }), "kuća");
    drag(layer, layer, [[60, 80]]);

    expect(await screen.findByText("kuća")).toBeInTheDocument();
    // That press only finished the text; the next one starts new text.
    expect(screen.queryByRole("textbox", { name: "Text on the page" })).not.toBeInTheDocument();
    await waitFor(() => {
      expect(calls(fetchSpy, "POST", "/api/documents/d1/text-boxes")).toHaveLength(1);
    });
  });

  it("leaves nothing behind when no text is typed", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Text");
    const layer = await drawingLayer();

    drag(layer, layer, [[300, 400]]);
    await userEvent.type(await screen.findByRole("textbox", { name: "Text on the page" }), "   {Enter}");

    expect(screen.queryByRole("textbox", { name: "Text on the page" })).not.toBeInTheDocument();
    expect(calls(fetchSpy, "POST", "/api/documents/d1/text-boxes")).toHaveLength(0);
  });

  it("edits, moves and deletes typed text", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi([], [answered]));
    await pickTool("Text");
    const layer = await drawingLayer();

    drag(await screen.findByText("kuća"), layer, [[310, 210]]);
    const input = await screen.findByRole("textbox", { name: "Text on the page" });
    expect(input).toHaveValue("kuća");
    await userEvent.clear(input);
    await userEvent.type(input, "kuće{Enter}");
    expect(await screen.findByText("kuće")).toBeInTheDocument();

    // 60px right and 80px down on a 600×800 page.
    drag(screen.getByText("kuće"), layer, [
      [310, 210],
      [340, 250],
      [370, 290],
    ]);

    drag(screen.getByText("kuće"), layer, [[370, 290]]);
    await userEvent.clear(await screen.findByRole("textbox", { name: "Text on the page" }));
    await userEvent.keyboard("{Enter}");

    await waitFor(() => {
      expect(calls(fetchSpy, "DELETE", "/api/text-boxes/t1")).toHaveLength(1);
    });
    expect(sent(fetchSpy, "PUT", "/api/text-boxes/t1")).toEqual([
      { x: 0.5, y: 0.25, text: "kuće", color: "blue", fontSize: 14 },
      { x: 0.6, y: 0.35, text: "kuće", color: "blue", fontSize: 14 },
    ]);
    expect(screen.queryByText("kuće")).not.toBeInTheDocument();
  });

  it("types new text in the size chosen", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Text");
    const size = screen.getByRole("textbox", { name: "Text size in pixels" });
    expect(size).toHaveValue("14");
    await userEvent.clear(size);
    await userEvent.type(size, "20");
    const layer = await drawingLayer();

    drag(layer, layer, [[300, 400]]);
    const input = await screen.findByRole("textbox", { name: "Text on the page" });
    expect(input).toHaveStyle({ fontSize: "20px" });
    await userEvent.type(input, "slatko{Enter}");

    expect(await screen.findByText("slatko")).toHaveStyle({ fontSize: "20px" });
    await waitFor(() => {
      // Half a 25px line above the click.
      expect(sent(fetchSpy, "POST", "/api/documents/d1/text-boxes")).toEqual([
        { pageNumber: 1, x: 0.5, y: 0.4844, text: "slatko", color: "black", fontSize: 20 },
      ]);
    });
  });

  it("keeps sizes within 8 to 48 pixels", async () => {
    renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Text");
    const size = screen.getByRole("textbox", { name: "Text size in pixels" });

    await userEvent.clear(size);
    await userEvent.type(size, "99");
    await userEvent.tab();
    // "9" was a size on the way; "99" is not one.
    expect(size).toHaveValue("9");
    await userEvent.click(screen.getByRole("button", { name: "Smaller text" }));
    expect(size).toHaveValue("8");
    expect(screen.getByRole("button", { name: "Smaller text" })).toBeDisabled();
  });

  it("resizes text being edited without closing it", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi([], [{ ...answered, fontSize: 22 }]));
    await pickTool("Text");
    const layer = await drawingLayer();

    drag(await screen.findByText("kuća"), layer, [[310, 210]]);
    const input = await screen.findByRole("textbox", { name: "Text on the page" });
    const size = screen.getByRole("textbox", { name: "Text size in pixels" });
    // The size field shows the size of the text opened.
    expect(size).toHaveValue("22");

    await userEvent.click(screen.getByRole("button", { name: "Larger text" }));
    await userEvent.click(screen.getByRole("button", { name: "Larger text" }));
    expect(input).toHaveFocus();
    expect(input).toHaveStyle({ fontSize: "24px" });

    // Typing a size takes the focus but leaves the text open.
    await userEvent.clear(size);
    await userEvent.type(size, "30");
    expect(screen.getByRole("textbox", { name: "Text on the page" })).toHaveStyle({ fontSize: "30px" });

    // A press elsewhere on the page finishes it.
    drag(layer, layer, [[60, 700]]);
    await waitFor(() => {
      expect(sent(fetchSpy, "PUT", "/api/text-boxes/t1")).toEqual([
        { x: 0.5, y: 0.25, text: "kuća", color: "blue", fontSize: 30 },
      ]);
    });
    expect(screen.getByText("kuća")).toHaveStyle({ fontSize: "30px" });
  });

  it("finishes the text when something outside the page is pressed", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Text");
    const layer = await drawingLayer();

    drag(layer, layer, [[300, 400]]);
    await userEvent.type(await screen.findByRole("textbox", { name: "Text on the page" }), "med");
    await userEvent.click(screen.getByRole("textbox", { name: "Text size in pixels" }));
    expect(screen.getByRole("textbox", { name: "Text on the page" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("heading", { level: 1 }));

    expect(await screen.findByText("med")).toBeInTheDocument();
    await waitFor(() => {
      expect(calls(fetchSpy, "POST", "/api/documents/d1/text-boxes")).toHaveLength(1);
    });
  });

  it("undoes with Ctrl+Z", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], drawingApi());
    await pickTool("Pen");
    const layer = await drawingLayer();
    drag(layer, layer, [
      [60, 80],
      [120, 80],
    ]);
    expect(layer.querySelectorAll("path[data-stroke]")).toHaveLength(1);

    await userEvent.keyboard("{Control>}z{/Control}");

    expect(layer.querySelectorAll("path[data-stroke]")).toHaveLength(0);
    // The stroke is saved before it is erased, so the API is never asked to erase an unknown one.
    await waitFor(() => {
      expect(calls(fetchSpy, "DELETE", "/api/strokes/stroke-1")).toHaveLength(1);
    });
    expect(calls(fetchSpy, "POST", "/api/documents/d1/strokes")).toHaveLength(1);
  });

  it("shows what was saved when a change cannot be saved", async () => {
    const { fetchSpy } = renderDocument("/documents/d1", [], {
      ...drawingApi(),
      "POST /api/documents/d1/strokes": {
        status: 500,
        body: { error: { code: "internal_error", message: "Something went wrong on our side. Try again." } },
      },
    });
    await pickTool("Pen");
    const layer = await drawingLayer();
    drag(layer, layer, [
      [60, 80],
      [120, 80],
    ]);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Your last change was not saved. Something went wrong on our side. Try again.",
    );
    await waitFor(() => {
      expect(layer.querySelectorAll("path[data-stroke]")).toHaveLength(0);
    });
    expect(calls(fetchSpy, "GET", "/api/documents/d1/strokes")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Undo" })).toBeDisabled();
  });
});

describe("DocumentPage export", () => {
  const onPage3: TextBox = { ...answered, id: "t3", pageNumber: 3 };

  /** Opens the export dialog once the drawing has loaded. */
  async function openExport() {
    const button = await screen.findByRole("button", { name: "Export" });
    await waitFor(() => {
      expect(button).toBeEnabled();
    });
    await userEvent.click(button);
    return within(screen.getByRole("dialog", { name: "Export with your writing" }));
  }

  it("exports the pages written on as a PDF", async () => {
    vi.mocked(exportDocument).mockResolvedValueOnce();
    renderDocument("/documents/d1", [], drawingApi([circled], [onPage3]));

    const dialog = await openExport();
    await userEvent.click(dialog.getByLabelText("Pages you wrote on (2 pages)"));
    await userEvent.click(dialog.getByRole("button", { name: "Export" }));

    expect(exportDocument).toHaveBeenCalledWith({
      url: "/api/documents/d1/file",
      title: "Čitanka",
      format: "pdf",
      pages: [1, 3],
      strokes: [circled],
      textBoxes: [onPage3],
    });
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("exports a picture of the page being read", async () => {
    vi.mocked(exportDocument).mockResolvedValueOnce();
    renderDocument("/documents/d1?page=2", [], drawingApi());

    const dialog = await openExport();
    expect(dialog.getByLabelText("Pages you wrote on (none yet)")).toBeDisabled();
    await userEvent.click(dialog.getByLabelText(/^PNG/));
    expect(dialog.queryByLabelText("All pages")).not.toBeInTheDocument();
    expect(dialog.getByText("The picture is of the page you are on, page 2.")).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("button", { name: "Export" }));

    expect(exportDocument).toHaveBeenCalledWith(expect.objectContaining({ format: "png", pages: [2] }));
  });

  it("says when the file could not be made, and stays open", async () => {
    vi.mocked(exportDocument).mockRejectedValueOnce(new Error("canvas too large"));
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    renderDocument("/documents/d1", [], drawingApi());

    const dialog = await openExport();
    await userEvent.click(dialog.getByLabelText("All pages"));
    await userEvent.click(dialog.getByRole("button", { name: "Export" }));

    expect(await dialog.findByRole("alert")).toHaveTextContent("Could not export the pages. Try again.");
    expect(exportDocument).toHaveBeenCalledWith(expect.objectContaining({ format: "pdf", pages: "all" }));
    expect(screen.getByRole("dialog", { name: "Export with your writing" })).toBeInTheDocument();
  });
});
