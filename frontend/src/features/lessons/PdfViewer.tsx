import { useEffect, useRef, useState } from "react";
import { Button } from "../../components/ui/Button";
import { loadPdf, type PdfDocument } from "../../lib/pdf/pdf";

/** Zoom levels the buttons step through; 1 is the PDF's own size. */
export const ZOOM_STEPS = [0.5, 0.75, 1, 1.25, 1.5, 2, 3] as const;
const DEFAULT_ZOOM_INDEX = ZOOM_STEPS.indexOf(1);

/**
 * Shows one page at a time on a canvas, with page and zoom controls. The
 * arrow keys turn pages while the page area has focus.
 */
export function PdfViewer({ url, title }: { url: string; title: string }) {
  const [doc, setDoc] = useState<PdfDocument | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  // The page box's own text, so it can be empty while someone types.
  const [pageText, setPageText] = useState("1");
  const [zoomIndex, setZoomIndex] = useState(DEFAULT_ZOOM_INDEX);
  const canvasRef = useRef<HTMLCanvasElement>(null);

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
        setPage(1);
        setPageText("1");
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

  const scale = ZOOM_STEPS[zoomIndex] ?? 1;

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!doc || !canvas) return;
    let active = true;
    const task = doc.renderPage(page, canvas, scale);
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
  }, [doc, page, scale]);

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

  const pageCount = doc.pageCount;

  function goTo(n: number) {
    const clamped = Math.min(Math.max(1, n), pageCount);
    setPage(clamped);
    setPageText(String(clamped));
  }

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
            disabled={page === 1}
            onClick={() => {
              goTo(page - 1);
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
              value={pageText}
              onChange={(event) => {
                setPageText(event.target.value);
                const n = Number(event.target.value);
                if (Number.isInteger(n) && n >= 1 && n <= pageCount) setPage(n);
              }}
              onBlur={() => {
                setPageText(String(page));
              }}
              className="h-8 w-12 rounded-sm border border-rule-strong bg-paper-raised px-2 text-center text-ink"
            />
            of {pageCount}
          </label>
          <Button
            variant="secondary"
            size="sm"
            disabled={page === pageCount}
            onClick={() => {
              goTo(page + 1);
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
        aria-label={`${title}, page ${String(page)} of ${String(pageCount)}`}
        role="region"
        onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "PageDown") goTo(page + 1);
          if (event.key === "ArrowLeft" || event.key === "PageUp") goTo(page - 1);
        }}
        className="max-h-[80vh] overflow-auto rounded-sm border border-rule bg-paper-sunken p-2 sm:p-4"
      >
        <canvas ref={canvasRef} className="mx-auto block bg-white shadow-sm" />
      </div>
    </div>
  );
}
