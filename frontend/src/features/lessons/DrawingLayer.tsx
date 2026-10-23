import { useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import { cx } from "../../components/ui/cx";
import { MAX_STROKE_POINTS, MAX_TEXT_BOX_LENGTH } from "../../lib/api/drawings";
import type { InkColor, Point, Stroke, TextBox, TextBoxInput } from "../../lib/api/types";
import { distanceToStroke, farEnough, strokePath, toPagePoint } from "../../lib/pdf/ink";
import type { Erased } from "./useDrawings";

/** What a pointer on the page does: select text, draw, type or erase. */
export type Tool = "select" | "pen" | "text" | "eraser";

/** Text size and line width at 100% zoom, in CSS pixels; both grow with the zoom. */
const FONT_SIZE = 14;
const LINE_HEIGHT = 1.25;
const STROKE_WIDTH = 2;
/** How close, in screen pixels, the eraser must pass to a stroke's line. */
const ERASER_REACH = 8;
/** How far, in screen pixels, text must be dragged before it moves rather than opens. */
const DRAG_THRESHOLD = 4;
/** Text starts left of and above this, so it stays on the page as the API requires. */
const MAX_START = 0.99;

/** Full class names, so Tailwind finds them. */
export const PEN_CLASSES: Record<InkColor, { text: string; stroke: string; fill: string }> = {
  black: {
    text: "text-pen-black",
    stroke: "stroke-pen-black",
    fill: "bg-pen-black",
  },
  blue: {
    text: "text-pen-blue",
    stroke: "stroke-pen-blue",
    fill: "bg-pen-blue",
  },
  red: { text: "text-pen-red", stroke: "stroke-pen-red", fill: "bg-pen-red" },
};

const round = (n: number) => Math.round(n * 10_000) / 10_000;
const clampStart = (n: number) => round(Math.min(Math.max(n, 0), MAX_START));

/** Text being typed: a new text box when id is null. */
type Editor = {
  id: string | null;
  x: number;
  y: number;
  text: string;
  color: InkColor;
};

/** A press on a text box with the text tool: a click opens it, a drag moves it. */
type Press = {
  box: TextBox;
  page: DOMRect;
  startX: number;
  startY: number;
  x: number;
  y: number;
  moved: boolean;
};

type DrawingLayerProps = {
  scale: number;
  tool: Tool;
  color: InkColor;
  /** This page's strokes and text boxes. */
  strokes: readonly Stroke[];
  textBoxes: readonly TextBox[];
  onDraw: (points: Point[]) => void;
  onErase: (erased: Erased) => void;
  onAddText: (fields: TextBoxInput) => void;
  /** Called with the new fields when text is edited or moved. */
  onChangeText: (id: string, fields: TextBoxInput) => void;
};

/**
 * What the student wrote and drew on one page, laid over the PDF. With the
 * pen, a drag draws a stroke; with the eraser, a drag removes every stroke
 * and text box it passes over; with the text tool, a click starts typing,
 * a click on text edits it and a drag moves it. With the select tool it
 * lets every pointer through to the PDF's text.
 */
export function DrawingLayer(props: DrawingLayerProps) {
  const { scale, tool, color, strokes, textBoxes, onDraw, onErase, onAddText, onChangeText } = props;
  const surfaceRef = useRef<HTMLDivElement>(null);
  const pen = useRef<{ page: DOMRect; points: Point[] } | null>(null);
  const [draft, setDraft] = useState<readonly Point[] | null>(null);
  const eraser = useRef<{
    page: DOMRect;
    strokes: Set<string>;
    textBoxes: Set<string>;
  } | null>(null);
  // Shown as gone while the eraser is still moving; erased together when it lifts.
  const [erasing, setErasing] = useState<Erased | null>(null);
  const press = useRef<Press | null>(null);
  const [moving, setMoving] = useState<{
    id: string;
    x: number;
    y: number;
  } | null>(null);
  // Mirrors the editor state, so a commit from Enter and one from the blur that follows count once.
  const editorRef = useRef<Editor | null>(null);
  const [editor, setEditorState] = useState<Editor | null>(null);
  const editorWasOpen = useRef(false);

  function setEditor(next: Editor | null) {
    editorRef.current = next;
    setEditorState(next);
  }

  function commitEditor() {
    const open = editorRef.current;
    if (!open) return;
    setEditor(null);
    const text = open.text.trim();
    const fields = { x: open.x, y: open.y, text, color: open.color };
    if (open.id === null) {
      if (text) onAddText(fields);
      return;
    }
    const box = textBoxes.find((b) => b.id === open.id);
    if (!box) return;
    // Clearing the text deletes it, as a text box cannot be empty.
    if (!text) onErase({ strokes: [], textBoxes: [box.id] });
    else if (text !== box.text) onChangeText(box.id, fields);
  }

  function eraseAt(clientX: number, clientY: number) {
    const sweep = eraser.current;
    const surface = surfaceRef.current;
    if (!sweep || !surface) return;
    const at = toPagePoint(clientX, clientY, sweep.page);
    const reach = ERASER_REACH + (STROKE_WIDTH * scale) / 2;
    let changed = false;
    for (const s of strokes) {
      if (!sweep.strokes.has(s.id) && distanceToStroke(s.points, at, sweep.page) <= reach) {
        sweep.strokes.add(s.id);
        changed = true;
      }
    }
    for (const el of surface.querySelectorAll<HTMLElement>("[data-text-box]")) {
      const id = el.dataset["textBox"];
      const r = el.getBoundingClientRect();
      const inside = clientX >= r.left && clientX <= r.right && clientY >= r.top && clientY <= r.bottom;
      if (id && inside && !sweep.textBoxes.has(id)) {
        sweep.textBoxes.add(id);
        changed = true;
      }
    }
    if (changed)
      setErasing({
        strokes: [...sweep.strokes],
        textBoxes: [...sweep.textBoxes],
      });
  }

  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    const surface = surfaceRef.current;
    if (tool === "select" || !surface) return;
    if (event.pointerType === "mouse" && event.button !== 0) return;
    const page = surface.getBoundingClientRect();

    if (tool === "pen") {
      // Keeps the page from scrolling or selecting text under the pen.
      event.preventDefault();
      surface.setPointerCapture(event.pointerId);
      const start = toPagePoint(event.clientX, event.clientY, page);
      pen.current = { page, points: [start] };
      setDraft([start]);
      return;
    }
    if (tool === "eraser") {
      event.preventDefault();
      surface.setPointerCapture(event.pointerId);
      eraser.current = { page, strokes: new Set(), textBoxes: new Set() };
      eraseAt(event.clientX, event.clientY);
      return;
    }

    // A press on the page finishes the text being typed. It is done here
    // rather than left to the text field's blur because preventDefault, which
    // stops a tap's emulated mousedown from taking the focus off text that
    // has only just opened, also keeps a click from moving the focus.
    event.preventDefault();
    editorWasOpen.current = editorRef.current !== null;
    commitEditor();
    const target = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-text-box]") : null;
    const box = textBoxes.find((b) => b.id === target?.dataset["textBox"]);
    if (box) {
      surface.setPointerCapture(event.pointerId);
      press.current = {
        box,
        page,
        startX: event.clientX,
        startY: event.clientY,
        x: box.x,
        y: box.y,
        moved: false,
      };
    }
  }

  function handlePointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    const stroke = pen.current;
    if (stroke) {
      const next = toPagePoint(event.clientX, event.clientY, stroke.page);
      const last = stroke.points.at(-1);
      if (last && !farEnough(last, next, stroke.page)) return;
      stroke.points.push(next);
      // A very long stroke is saved in pieces, each within the API's limit.
      if (stroke.points.length >= MAX_STROKE_POINTS) {
        onDraw(stroke.points);
        stroke.points = [next];
      }
      setDraft([...stroke.points]);
      return;
    }
    if (eraser.current) {
      eraseAt(event.clientX, event.clientY);
      return;
    }
    const p = press.current;
    if (!p) return;
    const dx = event.clientX - p.startX;
    const dy = event.clientY - p.startY;
    if (!p.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
    p.moved = true;
    p.x = clampStart(p.box.x + dx / p.page.width);
    p.y = clampStart(p.box.y + dy / p.page.height);
    setMoving({ id: p.box.id, x: p.x, y: p.y });
  }

  function handlePointerUp(event: ReactPointerEvent<HTMLDivElement>) {
    const stroke = pen.current;
    if (stroke) {
      pen.current = null;
      setDraft(null);
      onDraw(stroke.points);
      return;
    }
    const sweep = eraser.current;
    if (sweep) {
      eraser.current = null;
      setErasing(null);
      if (sweep.strokes.size + sweep.textBoxes.size > 0) {
        onErase({
          strokes: [...sweep.strokes],
          textBoxes: [...sweep.textBoxes],
        });
      }
      return;
    }
    if (tool !== "text") return;

    const p = press.current;
    if (p) {
      press.current = null;
      setMoving(null);
      if (p.moved)
        onChangeText(p.box.id, {
          x: p.x,
          y: p.y,
          text: p.box.text,
          color: p.box.color,
        });
      else
        setEditor({
          id: p.box.id,
          x: p.box.x,
          y: p.box.y,
          text: p.box.text,
          color: p.box.color,
        });
      return;
    }
    // That click closed the text being typed; the next one starts new text.
    if (editorWasOpen.current || !surfaceRef.current) return;
    const page = surfaceRef.current.getBoundingClientRect();
    const at = toPagePoint(event.clientX, event.clientY, page);
    // Centres the first line on the click, as on a line to fill in.
    const halfLine = (FONT_SIZE * scale * LINE_HEIGHT) / 2 / page.height;
    setEditor({
      id: null,
      x: clampStart(at.x),
      y: clampStart(at.y - halfLine),
      text: "",
      color,
    });
  }

  function handlePointerCancel() {
    pen.current = null;
    setDraft(null);
    eraser.current = null;
    setErasing(null);
    press.current = null;
    setMoving(null);
  }

  const shownStrokes = erasing ? strokes.filter((s) => !erasing.strokes.includes(s.id)) : strokes;
  const shownTextBoxes = textBoxes.filter((b) => b.id !== editor?.id && !erasing?.textBoxes.includes(b.id));
  const lineWidth = { strokeWidth: STROKE_WIDTH * scale };
  const textStyle = {
    fontSize: `${String(FONT_SIZE * scale)}px`,
    lineHeight: LINE_HEIGHT,
  };
  const editorLines = editor?.text.split("\n") ?? [];

  return (
    <div
      ref={surfaceRef}
      data-testid="drawing-layer"
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerCancel}
      className={cx(
        "absolute inset-0 z-[1] select-none",
        tool === "select" && "pointer-events-none",
        tool === "pen" && "cursor-crosshair touch-none",
        tool === "eraser" && "cursor-cell touch-none",
        tool === "text" && "cursor-text",
      )}
    >
      <svg
        aria-hidden="true"
        viewBox="0 0 1 1"
        preserveAspectRatio="none"
        className="absolute inset-0 size-full overflow-visible"
      >
        {shownStrokes.map((s) => (
          <path
            key={s.id}
            data-stroke={s.id}
            d={strokePath(s.points)}
            fill="none"
            strokeLinecap="round"
            strokeLinejoin="round"
            // Keeps the line's width in screen pixels although the drawing is stretched to the page.
            vectorEffect="non-scaling-stroke"
            className={PEN_CLASSES[s.color].stroke}
            style={lineWidth}
          />
        ))}
        {draft && (
          <path
            d={strokePath(draft)}
            fill="none"
            strokeLinecap="round"
            strokeLinejoin="round"
            vectorEffect="non-scaling-stroke"
            className={PEN_CLASSES[color].stroke}
            style={lineWidth}
          />
        )}
      </svg>

      {shownTextBoxes.map((b) => {
        const at = moving?.id === b.id ? moving : b;
        return (
          <div
            key={b.id}
            data-text-box={b.id}
            className={cx(
              "absolute font-sans break-words whitespace-pre-wrap",
              PEN_CLASSES[b.color].text,
              tool === "text" && "cursor-move touch-none",
            )}
            style={{
              ...textStyle,
              left: `${String(at.x * 100)}%`,
              top: `${String(at.y * 100)}%`,
              maxWidth: `${String((1 - at.x) * 100)}%`,
            }}
          >
            {b.text}
          </div>
        );
      })}

      {editor && (
        <textarea
          aria-label="Text on the page"
          autoFocus
          rows={editorLines.length}
          maxLength={MAX_TEXT_BOX_LENGTH}
          value={editor.text}
          onChange={(event) => {
            setEditor({ ...editor, text: event.target.value });
          }}
          onKeyDown={(event) => {
            // Enter finishes, as on a line to fill in; Shift+Enter starts another line.
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              commitEditor();
            }
            if (event.key === "Escape") {
              event.preventDefault();
              setEditor(null);
            }
          }}
          onBlur={commitEditor}
          // Clicks inside it place the caret; they are not clicks on the page.
          onPointerDown={(event) => {
            event.stopPropagation();
          }}
          onPointerUp={(event) => {
            event.stopPropagation();
          }}
          className={cx(
            "absolute resize-none overflow-hidden rounded-sm bg-white/70 p-0 font-sans outline-1 outline-accent outline-dashed",
            PEN_CLASSES[editor.color].text,
          )}
          style={{
            ...textStyle,
            left: `${String(editor.x * 100)}%`,
            top: `${String(editor.y * 100)}%`,
            width: `${String(Math.max(...editorLines.map((l) => l.length), 2) + 1)}ch`,
            maxWidth: `${String((1 - editor.x) * 100)}%`,
          }}
        />
      )}
    </div>
  );
}
