/**
 * The app's only contact with PDF.js. Pages and tests depend on this small
 * interface instead of the library, which cannot run in jsdom.
 *
 * PDF.js is loaded on first use with a dynamic import, so its few hundred
 * kilobytes are only downloaded by someone who opens a PDF.
 */
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import type { PagePicture } from "./raster";
import "./textLayer.css";

/** The elements one page is drawn into. */
export interface PageLayers {
  /** Holds the other layers; sized to the page by renderPage. */
  page: HTMLElement;
  canvas: HTMLCanvasElement;
  /** Receives the page's text as invisible, selectable spans over the canvas. Needs the `textLayer` class. */
  textLayer: HTMLElement;
}

export interface PdfDocument {
  readonly pageCount: number;
  /**
   * Draws one page (1-based) and lays its text over it. `scale` 1 is the
   * PDF's own size in CSS pixels; the canvas is sharpened for high-density
   * screens.
   */
  renderPage(pageNumber: number, layers: PageLayers, scale: number): Task<void>;
  /**
   * Draws one page (1-based) on a new canvas, `zoom` canvas pixels to one
   * point, for exporting. Its width and height are the page's size in points.
   */
  rasterizePage(pageNumber: number, zoom: number): Promise<PagePicture>;
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
      renderPage(pageNumber, { page: box, canvas, textLayer }, scale) {
        let renderTask: { promise: Promise<void>; cancel(): void } | null = null;
        let textTask: { render(): Promise<unknown>; cancel(): void } | null = null;
        let pageCancelled = false;
        const rendered = doc.getPage(pageNumber).then(async (page) => {
          if (pageCancelled) return;
          const viewport = page.getViewport({ scale });
          const width = `${String(Math.floor(viewport.width))}px`;
          const height = `${String(Math.floor(viewport.height))}px`;
          box.style.width = width;
          box.style.height = height;
          // The text layer's CSS sizes and places its spans from these.
          box.style.setProperty("--scale-factor", String(viewport.scale));
          box.style.setProperty("--user-unit", String(viewport.userUnit));

          const pixelRatio = window.devicePixelRatio || 1;
          canvas.width = Math.floor(viewport.width * pixelRatio);
          canvas.height = Math.floor(viewport.height * pixelRatio);
          canvas.style.width = width;
          canvas.style.height = height;
          renderTask = page.render({
            canvas,
            viewport,
            ...(pixelRatio === 1 ? {} : { transform: [pixelRatio, 0, 0, pixelRatio, 0, 0] }),
          });

          textLayer.replaceChildren();
          textTask = new pdfjs.TextLayer({ textContentSource: page.streamTextContent(), container: textLayer, viewport });
          await Promise.all([renderTask.promise, textTask.render()]);
        });
        return {
          promise: rendered,
          cancel() {
            pageCancelled = true;
            renderTask?.cancel();
            textTask?.cancel();
          },
        };
      },
      async rasterizePage(pageNumber, zoom) {
        const page = await doc.getPage(pageNumber);
        const size = page.getViewport({ scale: 1 });
        const viewport = page.getViewport({ scale: zoom });
        const canvas = document.createElement("canvas");
        canvas.width = Math.floor(viewport.width);
        canvas.height = Math.floor(viewport.height);
        // White under the page, as a PDF without a background shows white on screen too.
        await page.render({ canvas, viewport, background: "#ffffff" }).promise;
        return { canvas, width: size.width, height: size.height };
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
