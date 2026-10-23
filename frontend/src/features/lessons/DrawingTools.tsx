import { Button } from "../../components/ui/Button";
import { cx } from "../../components/ui/cx";
import type { InkColor } from "../../lib/api/types";
import { PEN_CLASSES, type Tool } from "./DrawingLayer";

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
  canUndo: boolean;
  onUndo: () => void;
  /** Drawing waits until what is already drawn has loaded. */
  drawingReady: boolean;
};

/** Chooses what a pointer on the page does, and the colour to write in. */
export function DrawingTools({
  tool,
  onToolChange,
  color,
  onColorChange,
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

        <Button variant="secondary" size="sm" disabled={!canUndo} onClick={onUndo} aria-keyshortcuts="Control+Z Meta+Z">
          Undo
        </Button>
      </div>
      <p className="text-sm text-ink-muted">{hint}</p>
    </div>
  );
}
