import { useState, type MouseEvent as ReactMouseEvent } from "react";
import { Button } from "../../components/ui/Button";
import { cx } from "../../components/ui/cx";
import { MAX_FONT_SIZE, MIN_FONT_SIZE } from "../../lib/api/drawings";
import type { InkColor } from "../../lib/api/types";
import { KEEPS_TEXT_OPEN, PEN_CLASSES, type Tool } from "./DrawingLayer";

const TOOLS: { tool: Tool; label: string; hint: string }[] = [
  {
    tool: "select",
    label: "Select",
    hint: "Select text to highlight it, add a note or save a word.",
  },
  {
    tool: "pen",
    label: "Pen",
    hint: "Draw on the page, for example to circle an answer.",
  },
  {
    tool: "text",
    label: "Text",
    hint: "Click where you want to write. Drag text to move it; clear it to delete it.",
  },
  {
    tool: "eraser",
    label: "Eraser",
    hint: "Drag across a drawing or text to erase it.",
  },
];

const COLORS: { color: InkColor; label: string }[] = [
  { color: "black", label: "Black" },
  { color: "blue", label: "Blue" },
  { color: "red", label: "Red" },
];

type DrawingToolsProps = {
  tool: Tool;
  onToolChange: (tool: Tool) => void;
  color: InkColor;
  onColorChange: (color: InkColor) => void;
  /** The text size in CSS pixels at 100% zoom, for new text and the text being typed. */
  fontSize: number;
  onFontSizeChange: (size: number) => void;
  canUndo: boolean;
  onUndo: () => void;
  /** Drawing waits until what is already drawn has loaded. */
  drawingReady: boolean;
};

/** Chooses what a pointer on the page does, the colour to write in and the size of text. */
export function DrawingTools({
  tool,
  onToolChange,
  color,
  onColorChange,
  fontSize,
  onFontSizeChange,
  canUndo,
  onUndo,
  drawingReady,
}: DrawingToolsProps) {
  const hint = TOOLS.find((t) => t.tool === tool)?.hint;

  return (
    <div className="grid gap-2">
      <div role="toolbar" aria-label="Drawing tools" className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <div role="group" aria-label="Tool" className="flex rounded-sm border border-rule-strong p-0.5">
          {TOOLS.map((t) => (
            <button
              key={t.tool}
              type="button"
              aria-pressed={t.tool === tool}
              disabled={t.tool !== "select" && !drawingReady}
              onClick={() => {
                onToolChange(t.tool);
              }}
              className={cx(
                "h-7 rounded-sm px-3 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50",
                t.tool === tool ? "bg-accent text-on-accent" : "text-ink-muted hover:text-ink",
              )}
            >
              {t.label}
            </button>
          ))}
        </div>

        <div role="group" aria-label="Colour" className="flex items-center gap-1">
          {COLORS.map((c) => (
            <button
              key={c.color}
              type="button"
              aria-label={c.label}
              aria-pressed={c.color === color}
              onClick={() => {
                onColorChange(c.color);
              }}
              className={cx(
                "grid size-8 place-items-center rounded-full border-2",
                c.color === color ? "border-ink" : "border-transparent hover:border-rule-strong",
              )}
            >
              <span className={cx("size-4 rounded-full", PEN_CLASSES[c.color].fill)} />
            </button>
          ))}
        </div>

        {tool === "text" && <FontSizeField size={fontSize} onChange={onFontSizeChange} />}

        <Button variant="secondary" size="sm" disabled={!canUndo} onClick={onUndo} aria-keyshortcuts="Control+Z Meta+Z">
          Undo
        </Button>
      </div>
      <p className="text-sm text-ink-muted">{hint}</p>
    </div>
  );
}

const clampSize = (n: number) => Math.min(Math.max(Math.round(n), MIN_FONT_SIZE), MAX_FONT_SIZE);

/**
 * The text size in pixels: typed, or stepped with − and +. It changes the
 * text being typed too, so the buttons keep the focus in that text and the
 * field takes it without finishing the text.
 */
function FontSizeField({ size, onChange }: { size: number; onChange: (size: number) => void }) {
  // What is typed, or null to show the size; "1" on the way to "18" is not a size yet.
  const [draft, setDraft] = useState<string | null>(null);
  const keepFocus = (event: ReactMouseEvent) => {
    event.preventDefault();
  };
  const stepButton = "grid size-7 place-items-center rounded-sm text-ink-muted hover:text-ink disabled:opacity-50";

  return (
    <div
      role="group"
      aria-label="Text size"
      {...{ [KEEPS_TEXT_OPEN]: "" }}
      className="flex items-center gap-1 rounded-sm border border-rule-strong p-0.5"
    >
      <button
        type="button"
        aria-label="Smaller text"
        disabled={size <= MIN_FONT_SIZE}
        onMouseDown={keepFocus}
        onClick={() => {
          setDraft(null);
          onChange(clampSize(size - 1));
        }}
        className={stepButton}
      >
        −
      </button>
      <label className="flex items-center gap-1 text-sm text-ink-muted">
        <input
          type="text"
          inputMode="numeric"
          aria-label="Text size in pixels"
          value={draft ?? String(size)}
          onChange={(event) => {
            setDraft(event.target.value);
            const n = Number(event.target.value);
            if (Number.isInteger(n) && n >= MIN_FONT_SIZE && n <= MAX_FONT_SIZE) onChange(n);
          }}
          onKeyDown={(event) => {
            if (event.key === "ArrowUp" || event.key === "ArrowDown") {
              event.preventDefault();
              setDraft(null);
              onChange(clampSize(size + (event.key === "ArrowUp" ? 1 : -1)));
            }
          }}
          onBlur={() => {
            setDraft(null);
          }}
          className="h-7 w-10 rounded-sm border border-rule bg-paper-raised px-1 text-center text-ink"
        />
        px
      </label>
      <button
        type="button"
        aria-label="Larger text"
        disabled={size >= MAX_FONT_SIZE}
        onMouseDown={keepFocus}
        onClick={() => {
          setDraft(null);
          onChange(clampSize(size + 1));
        }}
        className={stepButton}
      >
        +
      </button>
    </div>
  );
}
