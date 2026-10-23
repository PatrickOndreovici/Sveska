import { describe, expect, it } from "vitest";
import { containsPoint, normalizeQuote, toPageRects } from "./selection";

// A 600×800 page whose top-left corner is at (100, 50) on screen.
const page = new DOMRect(100, 50, 600, 800);

describe("toPageRects", () => {
  it("converts screen rectangles to page fractions", () => {
    expect(toPageRects([new DOMRect(160, 210, 300, 16)], page)).toEqual([
      { x: 0.1, y: 0.2, width: 0.5, height: 0.02 },
    ]);
  });

  it("merges the spans of one line and keeps lines apart", () => {
    const rects = toPageRects(
      [
        new DOMRect(160, 210, 100, 16), // "Dobar "
        new DOMRect(262, 211, 80, 15), // "dan," a pixel lower, as fonts make it
        new DOMRect(100, 230, 200, 16), // the next line
      ],
      page,
    );
    expect(rects).toEqual([
      { x: 0.1, y: 0.2, width: 0.3033, height: 0.02 },
      { x: 0, y: 0.225, width: 0.3333, height: 0.02 },
    ]);
  });

  it("keeps two columns of the same line apart", () => {
    const rects = toPageRects([new DOMRect(110, 210, 100, 16), new DOMRect(500, 210, 100, 16)], page);
    expect(rects).toHaveLength(2);
  });

  it("drops empty rectangles and clips what spills off the page", () => {
    const rects = toPageRects(
      [
        new DOMRect(300, 300, 0, 16), // a line break
        new DOMRect(650, 840, 100, 20), // runs past the bottom-right corner
      ],
      page,
    );
    expect(rects).toEqual([{ x: 0.9167, y: 0.9875, width: 0.0833, height: 0.0125 }]);
    for (const r of rects) {
      expect(r.x + r.width).toBeLessThanOrEqual(1);
      expect(r.y + r.height).toBeLessThanOrEqual(1);
    }
  });

  it("gives nothing for a page that has no size yet", () => {
    expect(toPageRects([new DOMRect(0, 0, 10, 10)], new DOMRect(0, 0, 0, 0))).toEqual([]);
  });
});

describe("normalizeQuote", () => {
  it("joins lines and trims", () => {
    expect(normalizeQuote("  Dobar\n dan,\t\tkako  si? ")).toBe("Dobar dan, kako si?");
  });
});

describe("containsPoint", () => {
  const rects = [{ x: 0.1, y: 0.2, width: 0.5, height: 0.02 }];
  it("finds a point on the highlight, edges included", () => {
    expect(containsPoint(rects, 0.3, 0.21)).toBe(true);
    expect(containsPoint(rects, 0.6, 0.22)).toBe(true);
  });
  it("misses a point beside it", () => {
    expect(containsPoint(rects, 0.3, 0.25)).toBe(false);
  });
});
