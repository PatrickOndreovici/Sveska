import { describe, expect, it } from "vitest";
import { distanceToStroke, farEnough, strokePath, toPagePoint } from "./ink";

// A 600×800 page whose top-left corner is at (100, 50) on screen.
const page = new DOMRect(100, 50, 600, 800);
const size = { width: 600, height: 800 };

describe("toPagePoint", () => {
  it("converts a screen position to page fractions", () => {
    expect(toPagePoint(250, 450, page)).toEqual({ x: 0.25, y: 0.5 });
  });

  it("rounds to four decimals", () => {
    expect(toPagePoint(300, 50, page)).toEqual({ x: 0.3333, y: 0 });
  });

  it("keeps a pointer dragged off the page on its edge", () => {
    expect(toPagePoint(0, 900, page)).toEqual({ x: 0, y: 1 });
  });
});

describe("farEnough", () => {
  it("drops points less than a pixel and a half apart", () => {
    expect(farEnough({ x: 0.5, y: 0.5 }, { x: 0.5 + 1 / 600, y: 0.5 }, size)).toBe(false);
    expect(farEnough({ x: 0.5, y: 0.5 }, { x: 0.5, y: 0.5 + 2 / 800 }, size)).toBe(true);
  });
});

describe("strokePath", () => {
  it("draws a dot for one point", () => {
    expect(strokePath([{ x: 0.1, y: 0.2 }])).toBe("M 0.1 0.2 L 0.1 0.2");
  });

  it("draws a straight line between two points", () => {
    expect(
      strokePath([
        { x: 0.1, y: 0.2 },
        { x: 0.3, y: 0.4 },
      ]),
    ).toBe("M 0.1 0.2 L 0.3 0.4");
  });

  it("curves through the midpoints of longer strokes", () => {
    const path = strokePath([
      { x: 0, y: 0 },
      { x: 0.2, y: 0 },
      { x: 0.2, y: 0.2 },
      { x: 0, y: 0.2 },
    ]);
    expect(path).toBe("M 0 0 Q 0.2 0 0.2 0.1 Q 0.2 0.2 0.1 0.2 L 0 0.2");
  });

  it("is empty without points", () => {
    expect(strokePath([])).toBe("");
  });
});

describe("distanceToStroke", () => {
  // A horizontal line across the middle of the page, from x 0.25 to 0.75.
  const line = [
    { x: 0.25, y: 0.5 },
    { x: 0.75, y: 0.5 },
  ];

  it("measures straight across to the line, in pixels", () => {
    expect(distanceToStroke(line, { x: 0.5, y: 0.51 }, size)).toBeCloseTo(8);
  });

  it("measures to the nearest end beyond it", () => {
    expect(distanceToStroke(line, { x: 0.8, y: 0.5 }, size)).toBeCloseTo(30);
  });

  it("measures to a dot", () => {
    expect(distanceToStroke([{ x: 0.5, y: 0.5 }], { x: 0.5, y: 0.505 }, size)).toBeCloseTo(4);
  });
});
