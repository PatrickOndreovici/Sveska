import type { Stroke, TextBox } from "../../lib/api/types";
import { paintInk } from "../../lib/pdf/paintInk";
import { loadPdf } from "../../lib/pdf/pdf";
import { canvasToBlob, picturesToPdf, saveFile, type PagePicture } from "../../lib/pdf/raster";

export type ExportFormat = "pdf" | "png" | "jpeg";

/**
 * Canvas pixels per point: 2 is 144 dots per inch, sharp enough to read
 * handwriting and small print without making a many-page PDF heavy.
 */
const EXPORT_ZOOM = 2;

export type ExportRequest = {
  /** Where the PDF's bytes are. */
  url: string;
  /** The document's title, which names the saved file. */
  title: string;
  format: ExportFormat;
  /** 1-based, in order, or every page. PNG and JPEG take only the first. */
  pages: readonly number[] | "all";
  strokes: readonly Stroke[];
  textBoxes: readonly TextBox[];
};

/** The pages that have something written or drawn on them, in order. */
export function pagesWithInk(strokes: readonly Stroke[], textBoxes: readonly TextBox[]): number[] {
  const pages = new Set([...strokes, ...textBoxes].map((x) => x.pageNumber));
  return [...pages].sort((a, b) => a - b);
}

/** A file name from the title, without characters that file systems refuse. */
export function exportFileName(title: string, format: ExportFormat, page?: number): string {
  const base = title.replace(/[\\/:*?"<>|\p{Cc}]/gu, "").trim() || "Worksheet";
  const suffix = page === undefined ? "" : ` - page ${String(page)}`;
  return `${base}${suffix}.${format === "jpeg" ? "jpg" : format}`;
}

/**
 * Saves pages of a PDF with what the student wrote and drew on them, as a
 * PDF or as a picture of one page. Highlights and notes stay out: the file
 * is the worksheet as it would be handed in.
 */
export async function exportDocument(request: ExportRequest): Promise<void> {
  const { url, title, format, pages, strokes, textBoxes } = request;
  const doc = await loadPdf(url).promise;
  try {
    const all = pages === "all" ? Array.from({ length: doc.pageCount }, (_, i) => i + 1) : pages;
    const wanted = format === "pdf" ? all : all.slice(0, 1);
    if (wanted.length === 0) throw new Error("no pages to export");
    const pictures: PagePicture[] = [];
    for (const n of wanted) {
      const picture = await doc.rasterizePage(n, EXPORT_ZOOM);
      await paintInk(
        picture.canvas,
        EXPORT_ZOOM,
        strokes.filter((s) => s.pageNumber === n),
        textBoxes.filter((b) => b.pageNumber === n),
      );
      pictures.push(picture);
    }

    if (format === "pdf") {
      const single = wanted.length === 1 ? wanted[0] : undefined;
      saveFile(await picturesToPdf(pictures), exportFileName(title, format, single));
      return;
    }
    const [picture] = pictures;
    if (!picture) return;
    const blob = await canvasToBlob(picture.canvas, format === "png" ? "image/png" : "image/jpeg");
    saveFile(blob, exportFileName(title, format, wanted[0]));
  } finally {
    doc.destroy();
  }
}
