/**
 * Turning a text selection on a rendered page into something that can be
 * stored: the selected text plus rectangles in 0–1 page coordinates, which
 * fit the page again at any zoom.
 */

/** A rectangle as fractions of the page's width and height, from its top-left corner. */
export type PageRect = { x: number; y: number; width: number; height: number };

export type PageSelection = { quote: string; rects: PageRect[] };

type Box = { left: number; top: number; right: number; bottom: number };

/**
 * The current selection, if it lies on the given page element; null when
 * nothing is selected there. Kept small so tests can replace it: jsdom has
 * no layout, so Range.getClientRects cannot work there.
 */
export function readSelection(page: HTMLElement): PageSelection | null {
  const selection = window.getSelection();
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) return null;
  const range = selection.getRangeAt(0);
  if (!page.contains(range.commonAncestorContainer)) return null;
  const quote = normalizeQuote(selection.toString());
  const rects = toPageRects(Array.from(range.getClientRects()), page.getBoundingClientRect());
  if (quote === "" || rects.length === 0) return null;
  return { quote, rects };
}

export function clearSelection() {
  window.getSelection()?.removeAllRanges();
}

/** Collapses the line breaks and runs of spaces a selection across lines brings. */
export function normalizeQuote(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

/**
 * Converts on-screen rectangles to page fractions. A selection gives one
 * rectangle per text span, often several per line, so pieces of the same
 * line that touch are merged into one; empty ones and anything outside the
 * page are dropped.
 */
export function toPageRects(clientRects: readonly DOMRectReadOnly[], page: DOMRectReadOnly): PageRect[] {
  if (page.width <= 0 || page.height <= 0) return [];
  const boxes: Box[] = [];
  for (const r of clientRects) {
    const box = {
      left: Math.max(r.left, page.left),
      top: Math.max(r.top, page.top),
      right: Math.min(r.right, page.right),
      bottom: Math.min(r.bottom, page.bottom),
    };
    // Less than a pixel either way is a line break or a gap, not text.
    if (box.right - box.left >= 1 && box.bottom - box.top >= 1) boxes.push(box);
  }
  boxes.sort((a, b) => a.top - b.top || a.left - b.left);

  const lines: Box[] = [];
  for (const box of boxes) {
    const line = lines.find((l) => sameLine(l, box) && touches(l, box));
    if (line) {
      line.left = Math.min(line.left, box.left);
      line.top = Math.min(line.top, box.top);
      line.right = Math.max(line.right, box.right);
      line.bottom = Math.max(line.bottom, box.bottom);
    } else {
      lines.push({ ...box });
    }
  }

  return lines.map((l) => {
    const x0 = round((l.left - page.left) / page.width);
    const y0 = round((l.top - page.top) / page.height);
    const x1 = round((l.right - page.left) / page.width);
    const y1 = round((l.bottom - page.top) / page.height);
    return { x: x0, y: y0, width: round(x1 - x0), height: round(y1 - y0) };
  });
}

/** Most of the shorter box's height is shared. */
function sameLine(a: Box, b: Box): boolean {
  const overlap = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
  return overlap >= 0.5 * Math.min(a.bottom - a.top, b.bottom - b.top);
}

/** Overlapping, or separated by no more than a wide space. */
function touches(a: Box, b: Box): boolean {
  const gap = Math.max(a.left, b.left) - Math.min(a.right, b.right);
  return gap <= 0.75 * Math.max(a.bottom - a.top, b.bottom - b.top);
}

/** Four decimals is well under a pixel on any screen and keeps the request small. */
function round(n: number): number {
  return Math.round(n * 10_000) / 10_000;
}

/** Whether a point, in page fractions, falls inside any of the rectangles. */
export function containsPoint(rects: readonly PageRect[], x: number, y: number): boolean {
  return rects.some((r) => x >= r.x && x <= r.x + r.width && y >= r.y && y <= r.y + r.height);
}
