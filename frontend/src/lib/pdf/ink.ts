/**
 * Geometry for drawing on a page: pointer positions become points in 0–1
 * page coordinates, points become an SVG path, and the eraser asks which
 * strokes pass near the pointer.
 */
import type { Point } from "../api/types";

type Size = { width: number; height: number };

/** Four decimals place a point within a tenth of a pixel on a 2000px page. */
const round = (n: number) => Math.round(n * 10_000) / 10_000;
const clamp = (n: number) => Math.min(Math.max(n, 0), 1);

/** A pointer position on screen as a point on the page, kept on the page. */
export function toPagePoint(clientX: number, clientY: number, page: DOMRectReadOnly): Point {
  return {
    x: round(clamp((clientX - page.left) / page.width)),
    y: round(clamp((clientY - page.top) / page.height)),
  };
}

/** The distance between two page points in screen pixels. */
function pixelDistance(a: Point, b: Point, size: Size): number {
  return Math.hypot((a.x - b.x) * size.width, (a.y - b.y) * size.height);
}

/**
 * Whether a new point is worth keeping: a pen reports dozens of points a
 * second, and ones closer than this add nothing but weight.
 */
export function farEnough(last: Point, next: Point, size: Size, minPixels = 1.5): boolean {
  return pixelDistance(last, next, size) >= minPixels;
}

/**
 * An SVG path through the points in 0–1 coordinates. The corners between
 * points are smoothed with curves through their midpoints; one point is a
 * dot, which a round line cap draws.
 */
export function strokePath(points: readonly Point[]): string {
  const [first, ...rest] = points;
  if (!first) return "";
  const at = (p: Point) => `${String(p.x)} ${String(p.y)}`;
  const last = rest.at(-1);
  if (!last) return `M ${at(first)} L ${at(first)}`;
  let path = `M ${at(first)}`;
  for (let i = 0; i < rest.length - 1; i++) {
    const p = rest[i] as Point;
    const q = rest[i + 1] as Point;
    path += ` Q ${at(p)} ${at({ x: round((p.x + q.x) / 2), y: round((p.y + q.y) / 2) })}`;
  }
  return `${path} L ${at(last)}`;
}

/** How far, in screen pixels, a point is from the nearest part of a stroke. */
export function distanceToStroke(points: readonly Point[], p: Point, size: Size): number {
  const toPixels = (q: Point) => ({
    x: q.x * size.width,
    y: q.y * size.height,
  });
  const target = toPixels(p);
  let best = Infinity;
  for (let i = 0; i < points.length; i++) {
    const a = toPixels(points[i] as Point);
    const b = toPixels(points[i + 1] ?? (points[i] as Point));
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const lengthSquared = dx * dx + dy * dy;
    // Where the target projects onto segment a–b, kept within the segment.
    const t = lengthSquared === 0 ? 0 : clamp(((target.x - a.x) * dx + (target.y - a.y) * dy) / lengthSquared);
    best = Math.min(best, Math.hypot(target.x - (a.x + t * dx), target.y - (a.y + t * dy)));
  }
  return best;
}
