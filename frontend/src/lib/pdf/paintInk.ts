/**
 * Paints what was written and drawn on a page onto a canvas, matching how
 * DrawingLayer shows it on screen, for exporting the page as a picture.
 */
import type { InkColor, Stroke, TextBox } from "../api/types";
import { strokePath } from "./ink";

/** Shared with DrawingLayer: typed text's line height, and a stroke's width in CSS pixels at 100% zoom, which grows with the zoom. */
export const LINE_HEIGHT = 1.25;
export const STROKE_WIDTH = 2;

/** The pen tokens in styles/index.css; pages are white in both themes, so these never change. */
const INK: Record<InkColor, { token: string; fallback: string }> = {
  black: { token: "--color-pen-black", fallback: "#1b1f27" },
  blue: { token: "--color-pen-blue", fallback: "#1f4fd1" },
  red: { token: "--color-pen-red", fallback: "#d0312d" },
};
const FALLBACK_FONT = '"IBM Plex Sans", ui-sans-serif, system-ui, sans-serif';

function inkColor(color: InkColor): string {
  const { token, fallback } = INK[color];
  return getComputedStyle(document.documentElement).getPropertyValue(token).trim() || fallback;
}

function fontFamily(): string {
  return getComputedStyle(document.documentElement).getPropertyValue("--font-sans").trim() || FALLBACK_FONT;
}

/**
 * Breaks text into the lines the screen shows, as CSS does with
 * `white-space: pre-wrap` and `overflow-wrap: break-word`: at line breaks,
 * then between words, and inside a word only when it is wider than a line.
 */
export function wrapLines(text: string, maxWidth: number, measure: (s: string) => number): string[] {
  const lines: string[] = [];
  for (const paragraph of text.split("\n")) {
    let line = "";
    // Words keep the spaces after them, as pre-wrap does.
    for (const word of paragraph.match(/\S+\s*|\s+/g) ?? [""]) {
      if (measure(line + word.trimEnd()) <= maxWidth) {
        line += word;
        continue;
      }
      if (line) lines.push(line.trimEnd());
      // A word wider than a whole line is broken where it runs out of room.
      let rest = word;
      while (measure(rest.trimEnd()) > maxWidth && rest.trimEnd().length > 1) {
        let fit = 1;
        while (fit < rest.length && measure(rest.slice(0, fit + 1)) <= maxWidth) fit++;
        lines.push(rest.slice(0, fit));
        rest = rest.slice(fit);
      }
      line = rest;
    }
    lines.push(line.trimEnd());
  }
  return lines;
}

/**
 * Paints strokes and text boxes over a page already drawn on the canvas.
 * `zoom` is how many canvas pixels make one CSS pixel of the page at 100%.
 */
export async function paintInk(
  canvas: HTMLCanvasElement,
  zoom: number,
  strokes: readonly Stroke[],
  textBoxes: readonly TextBox[],
): Promise<void> {
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no 2D canvas");
  const { width, height } = canvas;

  ctx.save();
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.lineWidth = STROKE_WIDTH * zoom;
  // The path is in 0–1 page coordinates; it is stretched to the page while
  // the line keeps its width, as vector-effect: non-scaling-stroke does on screen.
  const toPage = new DOMMatrix([width, 0, 0, height, 0, 0]);
  for (const s of strokes) {
    const path = new Path2D();
    path.addPath(new Path2D(strokePath(s.points)), toPage);
    ctx.strokeStyle = inkColor(s.color);
    ctx.stroke(path);
  }

  const family = fontFamily();
  // The page's font may not have been needed yet; without it the text would be measured in another.
  try {
    await document.fonts.load(`16px ${family}`, "čćšžđ");
  } catch {
    // The fallback font is fine.
  }
  ctx.textBaseline = "middle";
  for (const b of textBoxes) {
    const fontSize = b.fontSize * zoom;
    const lineHeight = fontSize * LINE_HEIGHT;
    ctx.font = `${String(fontSize)}px ${family}`;
    ctx.fillStyle = inkColor(b.color);
    const left = b.x * width;
    const lines = wrapLines(b.text, (1 - b.x) * width, (s) => ctx.measureText(s).width);
    lines.forEach((line, i) => {
      ctx.fillText(line, left, b.y * height + (i + 0.5) * lineHeight);
    });
  }
  ctx.restore();
}
