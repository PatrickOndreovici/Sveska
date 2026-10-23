/**
 * The app's only contact with PDF.js. Pages and tests depend on this small
 * interface instead of the library, which cannot run in jsdom.
 *
 * PDF.js is loaded on first use with a dynamic import, so its few hundred
 * kilobytes are only downloaded by someone who opens a PDF.
 */
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";

export interface PdfDocument {
  readonly pageCount: number;
  /**
   * Draws one page (1-based) into the canvas. `scale` 1 is the PDF's own
   * size in CSS pixels; the canvas is sharpened for high-density screens.
   */
  renderPage(pageNumber: number, canvas: HTMLCanvasElement, scale: number): Task<void>;
  destroy(): void;
}

/** A piece of work that can be abandoned, such as a page render that a newer one replaces. */
export interface Task<T> {
  promise: Promise<T>;
  cancel(): void;
}

export function loadPdf(url: string): Task<PdfDocument> {
  let destroyLoading: (() => void) | null = null;
  let cancelled = false;

  const promise = import("pdfjs-dist").then(async (pdfjs) => {
    pdfjs.GlobalWorkerOptions.workerSrc = workerUrl;
    // Same-origin request, so the session cookie is sent. Range requests let
    // a large PDF show its first page before the rest has downloaded.
    const loadingTask = pdfjs.getDocument({ url });
    destroyLoading = () => void loadingTask.destroy();
    if (cancelled) destroyLoading();
    const doc = await loadingTask.promise;

    return {
      pageCount: doc.numPages,
      renderPage(pageNumber, canvas, scale) {
        let renderTask: { promise: Promise<void>; cancel(): void } | null = null;
        let pageCancelled = false;
        const rendered = doc.getPage(pageNumber).then((page) => {
          if (pageCancelled) return;
          const pixelRatio = window.devicePixelRatio || 1;
          const viewport = page.getViewport({ scale: scale * pixelRatio });
          canvas.width = Math.floor(viewport.width);
          canvas.height = Math.floor(viewport.height);
          canvas.style.width = `${String(Math.floor(viewport.width / pixelRatio))}px`;
          canvas.style.height = `${String(Math.floor(viewport.height / pixelRatio))}px`;
          renderTask = page.render({ canvas, viewport });
          return renderTask.promise;
        });
        return {
          promise: rendered,
          cancel() {
            pageCancelled = true;
            renderTask?.cancel();
          },
        };
      },
      destroy() {
        // Frees the worker's copy of the document too.
        void loadingTask.destroy();
      },
    } satisfies PdfDocument;
  });

  return {
    promise,
    cancel() {
      cancelled = true;
      destroyLoading?.();
    },
  };
}
