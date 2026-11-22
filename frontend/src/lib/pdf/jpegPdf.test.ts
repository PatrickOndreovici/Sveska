import { describe, expect, it } from "vitest";
import { jpegsToPdf } from "./jpegPdf";

const decoder = new TextDecoder("latin1");

async function text(blob: Blob): Promise<string> {
  return decoder.decode(await blob.arrayBuffer());
}

const page = (jpeg: number[], width = 595.28, height = 841.89) => ({
  jpeg: new Uint8Array(jpeg),
  pixelWidth: 4,
  pixelHeight: 3,
  pageWidth: width,
  pageHeight: height,
});

describe("jpegsToPdf", () => {
  it("writes one page per image, each the size it is given", async () => {
    const pdf = await text(jpegsToPdf([page([0xff, 0xd8, 0xff, 0xd9]), page([1, 2], 300, 200)]));

    expect(pdf.startsWith("%PDF-1.4\n")).toBe(true);
    expect(pdf).toContain("/Type /Pages /Kids [3 0 R 6 0 R] /Count 2");
    expect(pdf).toContain("/MediaBox [0 0 595.28 841.89]");
    expect(pdf).toContain("/MediaBox [0 0 300 200]");
    expect(pdf).toContain("/Width 4 /Height 3 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length 4");
    expect(pdf).toContain("q 300 0 0 200 0 0 cm /Im0 Do Q");
    expect(pdf.trimEnd().endsWith("%%EOF")).toBe(true);
  });

  it("puts the image's bytes in unchanged", async () => {
    const bytes = new Uint8Array(await jpegsToPdf([page([0xff, 0xd8, 0x00, 0x80, 0xff, 0xd9])]).arrayBuffer());
    const at = decoder.decode(bytes).indexOf("stream\n", decoder.decode(bytes).indexOf("/DCTDecode")) + "stream\n".length;

    expect([...bytes.slice(at, at + 6)]).toEqual([0xff, 0xd8, 0x00, 0x80, 0xff, 0xd9]);
  });

  it("points the cross-reference table at each object", async () => {
    const pdf = await text(jpegsToPdf([page([1, 2, 3])]));
    const xref = Number(/startxref\n(\d+)/.exec(pdf)?.[1]);
    const table = pdf.slice(xref).split("\n");

    expect(table[0]).toBe("xref");
    expect(table[1]).toBe("0 6");
    for (let n = 1; n <= 5; n++) {
      const offset = Number(table[2 + n]?.slice(0, 10));
      expect(pdf.slice(offset).startsWith(`${String(n)} 0 obj`)).toBe(true);
    }
    expect(pdf).toContain("trailer\n<< /Size 6 /Root 1 0 R >>");
  });

  it("refuses to write a PDF without pages", () => {
    expect(() => jpegsToPdf([])).toThrow();
  });
});
