/**
 * Pictures of pages: canvases saved as JPEG or PNG, gathered into a PDF,
 * and photos or screenshots turned into a one-page PDF so they can be
 * opened, written on and exported like any other worksheet.
 */
import { jpegsToPdf, type JpegPage } from "./jpegPdf";

/** JPEG quality: high enough that small print in a scan stays sharp. */
const JPEG_QUALITY = 0.92;
/** An image's longer side is cut to this, which keeps a phone photo well under the upload limit. */
const MAX_IMAGE_SIDE = 3000;
/** Pages made from images are as wide as A4, in PDF points. */
const IMAGE_PAGE_WIDTH = 595.28;

/** The kinds of picture that can be uploaded instead of a PDF. HEIC opens only where the browser can decode it. */
export const IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp", "image/heic", "image/heif"] as const;
const IMAGE_EXTENSIONS = /\.(jpe?g|png|webp|heic|heif)$/i;

export function isImageFile(file: File): boolean {
  return (IMAGE_TYPES as readonly string[]).includes(file.type) || IMAGE_EXTENSIONS.test(file.name);
}

export function canvasToBlob(canvas: HTMLCanvasElement, type: "image/jpeg" | "image/png"): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (blob) resolve(blob);
        else reject(new Error(`could not encode the canvas as ${type}`));
      },
      type,
      JPEG_QUALITY,
    );
  });
}

/** A rendered page and its size in PDF points. */
export type PagePicture = { canvas: HTMLCanvasElement; width: number; height: number };

/** One PDF page per picture, each the size it is given. */
export async function picturesToPdf(pictures: readonly PagePicture[]): Promise<Blob> {
  const pages: JpegPage[] = [];
  for (const { canvas, width, height } of pictures) {
    const jpeg = new Uint8Array(await (await canvasToBlob(canvas, "image/jpeg")).arrayBuffer());
    pages.push({ jpeg, pixelWidth: canvas.width, pixelHeight: canvas.height, pageWidth: width, pageHeight: height });
  }
  return jpegsToPdf(pages);
}

/** "IMG_2265 (1).jpeg" becomes "IMG_2265 (1).pdf", which the API turns into the title. */
export function pdfFileName(name: string): string {
  const base = name.replace(/\.[^./\\]*$/, "");
  return `${base || "Image"}.pdf`;
}

/**
 * A photo or screenshot as a one-page PDF. The image is drawn on a white
 * canvas, which also turns a phone photo the right way up and gives a PNG's
 * transparent parts a paper background. Rejects when the browser cannot
 * decode the image.
 */
export async function imageFileToPdf(file: File): Promise<File> {
  const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  try {
    const shrink = Math.min(1, MAX_IMAGE_SIDE / Math.max(bitmap.width, bitmap.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.max(1, Math.round(bitmap.width * shrink));
    canvas.height = Math.max(1, Math.round(bitmap.height * shrink));
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("no 2D canvas");
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
    const pdf = await picturesToPdf([
      { canvas, width: IMAGE_PAGE_WIDTH, height: (IMAGE_PAGE_WIDTH * canvas.height) / canvas.width },
    ]);
    return new File([pdf], pdfFileName(file.name), { type: "application/pdf" });
  } finally {
    bitmap.close();
  }
}

/** Saves a file through the browser's download, as a link with `download` would. */
export function saveFile(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.append(link);
  link.click();
  link.remove();
  // Revoked later, as some browsers read the file after click returns.
  setTimeout(() => {
    URL.revokeObjectURL(url);
  }, 60_000);
}
