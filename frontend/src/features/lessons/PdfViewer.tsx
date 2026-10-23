import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { Button } from "../../components/ui/Button";
import { cx } from "../../components/ui/cx";
import { loadPdf, type PdfDocument } from "../../lib/pdf/pdf";
import {
  clearSelection,
  containsPoint,
  readSelection,
  type PageRect,
  type PageSelection,
} from "../../lib/pdf/selection";

/** Zoom levels the buttons step through; 1 is the PDF's own size. */
export const ZOOM_STEPS = [0.5, 0.75, 1, 1.25, 1.5, 2, 3] as const;
const DEFAULT_ZOOM_INDEX = ZOOM_STEPS.indexOf(1);

/** A saved highlight to draw over its page. */
export type PdfHighlight = {
  id: string;
  pageNumber: number;
  rects: readonly PageRect[];
  hasNote: boolean;
};

/** Text the reader selected, with the page it is on. */
export type TextSelection = PageSelection & { pageNumber: number };

type PdfViewerProps = {
  url: string;
  title: string;
  /** The page shown, 1-based. A page past the end is moved to the last one. */
  page: number;
  onPageChange: (page: number) => void;
  highlights: readonly PdfHighlight[];
  activeHighlightId: string | null;
  /** A click on a highlight, or null for a click on the page beside them. */
  onHighlightClick: (id: string | null) => void;
  /** What to offer for selected text, shown just below it. `dismiss` drops the selection. */
  selectionActions: (selection: TextSelection, dismiss: () => void) => ReactNode;
};

/**
 * Shows one page at a time with page and zoom controls. The page's text can
 * be selected, and saved highlights are drawn over it. The arrow keys turn
 * pages while the page area has focus.
 */
export function PdfViewer(props: PdfViewerProps) {
  const { url, title, page, onPageChange, highlights, activeHighlightId, onHighlightClick, selectionActions } = props;
  const [doc, setDoc] = useState<PdfDocument | null>(null);
  const [error, setError] = useState<string | null>(null);
  // What is typed in the page box, or null to show the current page.
  const [pageDraft, setPageDraft] = useState<string | null>(null);
  const [zoomIndex, setZoomIndex] = useState(DEFAULT_ZOOM_INDEX);
  const [selection, setSelection] = useState<TextSelection | null>(null);
  const pageRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const textLayerRef = useRef<HTMLDivElement>(null);
  const pointerDown = useRef(false);

  useEffect(() => {
    let active = true;
    let loaded: PdfDocument | null = null;
    const task = loadPdf(url);
    task.promise.then(
      (next) => {
        if (!active) {
          next.destroy();
          return;
        }
        loaded = next;
        setDoc(next);
      },
      (err: unknown) => {
        if (!active) return;
        console.error("open PDF", err);
        setError("This PDF could not be opened. It may be damaged; try downloading it instead.");
      },
    );
    return () => {
      active = false;
      task.cancel();
      loaded?.destroy();
    };
  }, [url]);

  const pageCount = doc?.pageCount ?? 0;
  const scale = ZOOM_STEPS[zoomIndex] ?? 1;
  // A page number from a link may be past the end of the document.
  const shownPage = doc ? Math.min(Math.max(1, page), pageCount) : page;

  useEffect(() => {
    if (doc && shownPage !== page) onPageChange(shownPage);
  }, [doc, shownPage, page, onPageChange]);

  useEffect(() => {
    const [box, canvas, textLayer] = [pageRef.current, canvasRef.current, textLayerRef.current];
    if (!doc || !box || !canvas || !textLayer) return;
    let active = true;
    const task = doc.renderPage(shownPage, { page: box, canvas, textLayer }, scale);
    task.promise.catch((err: unknown) => {
      // A cancelled render rejects too; only a real failure is shown.
      if (!active) return;
      console.error("render PDF page", err);
      setError("This page could not be shown.");
    });
    return () => {
      active = false;
      task.cancel();
    };
  }, [doc, shownPage, scale]);

  // On touch screens the selection is adjusted with handles after the
  // finger lifts, so later changes are picked up here too.
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    function onSelectionChange() {
      clearTimeout(timer);
      timer = setTimeout(() => {
        const box = pageRef.current;
        if (pointerDown.current || !box) return;
        const found = readSelection(box);
        if (found) setSelection({ ...found, pageNumber: shownPage });
      }, 250);
    }
    document.addEventListener("selectionchange", onSelectionChange);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("selectionchange", onSelectionChange);
    };
  }, [shownPage]);

  if (error) {
    return (
      <p role="alert" className="text-danger">
        {error}
      </p>
    );
  }
  if (!doc) {
    return (
      <p role="status" className="text-ink-muted">
        Opening the PDF…
      </p>
    );
  }

  function goTo(n: number) {
    setPageDraft(null);
    setSelection(null);
    onPageChange(Math.min(Math.max(1, n), pageCount));
  }

  function dismissSelection() {
    setSelection(null);
    clearSelection();
  }

  /** After a drag or click on the page: either text was selected, or a highlight may have been clicked. */
  function handlePointerUp(event: ReactPointerEvent<HTMLDivElement>) {
    pointerDown.current = false;
    const box = pageRef.current;
    if (!box) return;
    const found = readSelection(box);
    if (found) {
      setSelection({ ...found, pageNumber: shownPage });
      return;
    }
    setSelection(null);
    const r = box.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return;
    const x = (event.clientX - r.left) / r.width;
    const y = (event.clientY - r.top) / r.height;
    // The newest highlight is drawn on top, so it wins where they overlap.
    const hit = highlights.findLast((h) => h.pageNumber === shownPage && containsPoint(h.rects, x, y));
    onHighlightClick(hit?.id ?? null);
  }

  const onPage = highlights.filter((h) => h.pageNumber === shownPage);
  const shownSelection = selection?.pageNumber === shownPage ? selection : null;
  const anchor = shownSelection?.rects.at(-1);

  return (
    <div className="grid gap-3">
      <div
        role="toolbar"
        aria-label="PDF controls"
        className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3"
      >
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            disabled={shownPage === 1}
            onClick={() => {
              goTo(shownPage - 1);
            }}
          >
            Previous
          </Button>
          <label className="flex items-center gap-2 text-sm text-ink-muted">
            Page
            <input
              type="text"
              inputMode="numeric"
              aria-label="Page number"
              value={pageDraft ?? String(shownPage)}
              onChange={(event) => {
                setPageDraft(event.target.value);
                const n = Number(event.target.value);
                if (Number.isInteger(n) && n >= 1 && n <= pageCount) {
                  setSelection(null);
                  onPageChange(n);
                }
              }}
              onBlur={() => {
                setPageDraft(null);
              }}
              className="h-8 w-12 rounded-sm border border-rule-strong bg-paper-raised px-2 text-center text-ink"
            />
            of {pageCount}
          </label>
          <Button
            variant="secondary"
            size="sm"
            disabled={shownPage === pageCount}
            onClick={() => {
              goTo(shownPage + 1);
            }}
          >
            Next
          </Button>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            aria-label="Zoom out"
            disabled={zoomIndex === 0}
            onClick={() => {
              setZoomIndex(zoomIndex - 1);
            }}
          >
            −
          </Button>
          <output aria-label="Zoom" className="w-12 text-center text-sm text-ink-muted">
            {Math.round(scale * 100)}%
          </output>
          <Button
            variant="secondary"
            size="sm"
            aria-label="Zoom in"
            disabled={zoomIndex === ZOOM_STEPS.length - 1}
            onClick={() => {
              setZoomIndex(zoomIndex + 1);
            }}
          >
            +
          </Button>
        </div>
      </div>

      {/* Zoomed pages scroll inside this box, never the whole page. */}
      <div
        tabIndex={0}
        aria-label={`${title}, page ${String(shownPage)} of ${String(pageCount)}`}
        role="region"
        onKeyDown={(event) => {
          if (event.key === "Escape") dismissSelection();
          // Shift+arrows extend a selection rather than turn the page.
          if (event.shiftKey) return;
          if (event.key === "ArrowRight" || event.key === "PageDown") goTo(shownPage + 1);
          if (event.key === "ArrowLeft" || event.key === "PageUp") goTo(shownPage - 1);
        }}
        className="max-h-[80vh] overflow-auto rounded-sm border border-rule bg-paper-sunken p-2 sm:p-4"
      >
        <div
          ref={pageRef}
          data-testid="pdf-page"
          className="pdfPage relative mx-auto bg-white shadow-sm"
          onPointerDown={() => {
            pointerDown.current = true;
          }}
          onPointerUp={handlePointerUp}
        >
          <canvas ref={canvasRef} className="block" />
          {/* Drawn under the text, so the text stays selectable through a highlight. */}
          <div aria-hidden="true" className="pointer-events-none absolute inset-0">
            {onPage.map((h) =>
              h.rects.map((r, i) => (
                <div
                  key={`${h.id}-${String(i)}`}
                  data-highlight={h.id}
                  className={cx(
                    "absolute mix-blend-multiply",
                    h.id === activeHighlightId ? "bg-page-highlight-active" : "bg-page-highlight",
                  )}
                  style={{
                    left: `${String(r.x * 100)}%`,
                    top: `${String(r.y * 100)}%`,
                    width: `${String(r.width * 100)}%`,
                    height: `${String(r.height * 100)}%`,
                  }}
                >
                  {/* A note is marked at the start of its highlight. */}
                  {h.hasNote && i === 0 && (
                    <span className="absolute -top-1 -left-1 size-2 rounded-full bg-accent mix-blend-normal" />
                  )}
                </div>
              )),
            )}
          </div>
          <div ref={textLayerRef} className="textLayer" />

          {shownSelection && anchor && (
            <div
              role="toolbar"
              aria-label="Selected text"
              // Its own clicks must not count as a new selection or a click on the page.
              onPointerDown={(event) => {
                event.stopPropagation();
              }}
              onPointerUp={(event) => {
                event.stopPropagation();
              }}
              className="absolute z-10 mt-1.5 flex flex-wrap gap-1 rounded-md border border-rule bg-paper-raised p-1 shadow-md"
              style={{
                top: `${String((anchor.y + anchor.height) * 100)}%`,
                // Near the right edge it is aligned to the selection's end instead, to stay on the page.
                ...(anchor.x > 0.55
                  ? { right: `${String((1 - anchor.x - anchor.width) * 100)}%` }
                  : { left: `${String(anchor.x * 100)}%` }),
              }}
            >
              {selectionActions(shownSelection, dismissSelection)}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
