/**
 * A minimal PDF writer: each page is one JPEG stretched over it. PDF
 * readers decode JPEG themselves (the DCTDecode filter), so the image's
 * bytes go in unchanged and nothing has to be compressed here.
 */

/** One page: a baseline JPEG in RGB, and the page's size in PDF points (1/72 inch). */
export type JpegPage = {
  jpeg: Uint8Array;
  /** The image's size in pixels, as encoded in the JPEG. */
  pixelWidth: number;
  pixelHeight: number;
  pageWidth: number;
  pageHeight: number;
};

const encoder = new TextEncoder();

/** Two decimals are far finer than a printer's dot. */
const num = (n: number) => String(Math.round(n * 100) / 100);

export function jpegsToPdf(pages: readonly JpegPage[]): Blob {
  if (pages.length === 0) throw new Error("a PDF needs at least one page");
  const chunks: Uint8Array[] = [];
  const offsets: number[] = [];
  let length = 0;

  const write = (part: string | Uint8Array) => {
    const bytes = typeof part === "string" ? encoder.encode(part) : part;
    chunks.push(bytes);
    length += bytes.length;
  };
  /** Objects are numbered from 1 in the order they are written. */
  const object = (body: string, stream?: Uint8Array) => {
    offsets.push(length);
    write(`${String(offsets.length)} 0 obj\n${body}\n`);
    if (stream) {
      write("stream\n");
      write(stream);
      write("\nendstream\n");
    }
    write("endobj\n");
  };

  // The second line's bytes above 127 tell tools the file is binary.
  write("%PDF-1.4\n%âãÏÓ\n");
  // 1 is the catalog and 2 the page tree; each page then takes three objects.
  const pageId = (i: number) => 3 + i * 3;
  object("<< /Type /Catalog /Pages 2 0 R >>");
  object(`<< /Type /Pages /Kids [${pages.map((_, i) => `${String(pageId(i))} 0 R`).join(" ")}] /Count ${String(pages.length)} >>`);

  pages.forEach((page, i) => {
    const [w, h] = [num(page.pageWidth), num(page.pageHeight)];
    const contentsId = pageId(i) + 1;
    const imageId = pageId(i) + 2;
    object(
      `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${w} ${h}] ` +
        `/Resources << /XObject << /Im0 ${String(imageId)} 0 R >> >> /Contents ${String(contentsId)} 0 R >>`,
    );
    // Scales the 1×1 image space to the page and draws the image.
    const contents = encoder.encode(`q ${w} 0 0 ${h} 0 0 cm /Im0 Do Q`);
    object(`<< /Length ${String(contents.length)} >>`, contents);
    object(
      `<< /Type /XObject /Subtype /Image /Width ${String(page.pixelWidth)} /Height ${String(page.pixelHeight)} ` +
        `/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${String(page.jpeg.length)} >>`,
      page.jpeg,
    );
  });

  const xref = length;
  const size = offsets.length + 1;
  write(`xref\n0 ${String(size)}\n0000000000 65535 f \n`);
  for (const offset of offsets) write(`${String(offset).padStart(10, "0")} 00000 n \n`);
  write(`trailer\n<< /Size ${String(size)} /Root 1 0 R >>\nstartxref\n${String(xref)}\n%%EOF\n`);

  return new Blob(chunks as BlobPart[], { type: "application/pdf" });
}
