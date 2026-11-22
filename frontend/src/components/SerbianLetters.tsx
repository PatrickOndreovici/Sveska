import { cx } from "./ui/cx";

const LETTERS = ["č", "ć", "š", "ž", "đ"] as const;

type SerbianLettersProps = {
  /** The field a letter goes into: called on each click, so it can be the one used last. */
  target: () => HTMLInputElement | null;
  className?: string;
};

/**
 * Buttons that type č ć š ž đ into a field, for keyboards without them;
 * with Shift, the capital. They keep the focus in the field, and stay out of
 * the tab order so that they do not stand between one field and the next.
 */
export function SerbianLetters({ target, className }: SerbianLettersProps) {
  return (
    <div role="group" aria-label="Serbian letters" className={cx("flex flex-wrap gap-1.5", className)}>
      {LETTERS.map((letter) => (
        <button
          key={letter}
          type="button"
          tabIndex={-1}
          aria-label={`Type ${letter}`}
          lang="sr"
          className="h-8 min-w-8 rounded-sm border border-rule-strong bg-paper-raised px-2 text-ink hover:border-ink-muted hover:bg-paper-sunken"
          // Keeps the focus, and the cursor, in the field.
          onMouseDown={(event) => {
            event.preventDefault();
          }}
          onClick={(event) => {
            const field = target();
            if (field) typeInto(field, event.shiftKey ? letter.toLocaleUpperCase("sr") : letter);
          }}
        >
          {letter}
        </button>
      ))}
    </div>
  );
}

/** Puts text where the cursor is in field, over any selection, as typing would. */
function typeInto(field: HTMLInputElement, text: string) {
  const start = field.selectionStart ?? field.value.length;
  const end = field.selectionEnd ?? start;
  field.setRangeText(text, start, end, "end");
  // Lets React and anything else listening see the change.
  field.dispatchEvent(new Event("input", { bubbles: true }));
  field.focus();
}
