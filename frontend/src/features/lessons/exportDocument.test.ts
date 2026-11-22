import { describe, expect, it } from "vitest";
import type { Stroke, TextBox } from "../../lib/api/types";
import { pdfFileName } from "../../lib/pdf/raster";
import { exportFileName, pagesWithInk } from "./exportDocument";

describe("pagesWithInk", () => {
  it("lists each page written or drawn on once, in order", () => {
    const strokes = [{ pageNumber: 4 }, { pageNumber: 1 }] as Stroke[];
    const textBoxes = [{ pageNumber: 4 }, { pageNumber: 2 }] as TextBox[];
    expect(pagesWithInk(strokes, textBoxes)).toEqual([1, 2, 4]);
  });
});

describe("exportFileName", () => {
  it("names the file after the document and the page", () => {
    expect(exportFileName("Lekcija 8", "png", 3)).toBe("Lekcija 8 - page 3.png");
    expect(exportFileName("Lekcija 8", "jpeg", 1)).toBe("Lekcija 8 - page 1.jpg");
    expect(exportFileName("Lekcija 8", "pdf")).toBe("Lekcija 8.pdf");
  });

  it("drops characters file systems refuse", () => {
    expect(exportFileName('Unit 2: "U kafeu"', "pdf")).toBe("Unit 2 U kafeu.pdf");
    expect(exportFileName("///", "pdf")).toBe("Worksheet.pdf");
  });
});

describe("pdfFileName", () => {
  it("gives a picture's name the PDF extension", () => {
    expect(pdfFileName("IMG_2265 (1).jpeg")).toBe("IMG_2265 (1).pdf");
    expect(pdfFileName("scan")).toBe("scan.pdf");
    expect(pdfFileName(".png")).toBe("Image.pdf");
  });
});
