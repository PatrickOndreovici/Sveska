import { useState, type SubmitEvent } from "react";
import { Button } from "../../components/ui/Button";
import { cx } from "../../components/ui/cx";
import { ApiError, errorMessage } from "../../lib/api/client";
import { MAX_NOTE_LENGTH } from "../../lib/api/annotations";
import type { Annotation } from "../../lib/api/types";

type AnnotationPanelProps = {
  /** null while loading. */
  annotations: readonly Annotation[] | null;
  loadError: string | null;
  onRetry: () => void;
  activeId: string | null;
  editingId: string | null;
  /** Shows the highlight on its page. */
  onSelect: (annotation: Annotation) => void;
  /** Opens the note form for an annotation, or closes it with null. */
  onEditNote: (id: string | null) => void;
  /** Rejects with an ApiError when the API refuses the note. */
  onSaveNote: (id: string, note: string) => Promise<void>;
  onDelete: (annotation: Annotation) => void;
};

/** A quote short enough for a button's accessible name. */
function shortQuote(quote: string): string {
  return quote.length > 40 ? `${quote.slice(0, 39)}…` : quote;
}

/**
 * The document's highlights in reading order, grouped by page. Each can be
 * shown on its page, given a note or deleted. This is also how the
 * highlights are reached from the keyboard and by screen readers, since the
 * ones drawn on the page are only visual.
 */
export function AnnotationPanel(props: AnnotationPanelProps) {
  const { annotations, loadError, onRetry } = props;

  let body;
  if (loadError) {
    body = (
      <div role="alert" className="mt-3 grid justify-items-start gap-3">
        <p className="text-sm text-danger">{loadError}</p>
        <Button variant="secondary" size="sm" onClick={onRetry}>
          Try again
        </Button>
      </div>
    );
  } else if (!annotations) {
    body = (
      <p role="status" className="mt-3 text-sm text-ink-muted">
        Loading highlights…
      </p>
    );
  } else if (annotations.length === 0) {
    body = (
      <p className="mt-3 text-sm text-ink-muted">
        Select text on the page to highlight it, write a note on it or save a word to your vocabulary.
      </p>
    );
  } else {
    const pages = [...new Set(annotations.map((a) => a.pageNumber))];
    body = (
      <div className="mt-4 grid gap-5">
        {pages.map((page) => (
          <section key={page} aria-labelledby={`annotations-page-${String(page)}`}>
            <h3
              id={`annotations-page-${String(page)}`}
              className="text-eyebrow font-medium tracking-[0.12em] text-ink-subtle uppercase"
            >
              Page {page}
            </h3>
            <ul className="mt-2 grid gap-2">
              {annotations
                .filter((a) => a.pageNumber === page)
                .map((a) => (
                  <AnnotationItem key={a.id} annotation={a} {...props} />
                ))}
            </ul>
          </section>
        ))}
      </div>
    );
  }

  return (
    <section aria-labelledby="annotations-heading">
      <h2 id="annotations-heading" className="font-serif text-xl text-ink">
        Highlights
      </h2>
      {body}
    </section>
  );
}

function AnnotationItem({
  annotation: a,
  activeId,
  editingId,
  onSelect,
  onEditNote,
  onSaveNote,
  onDelete,
}: AnnotationPanelProps & { annotation: Annotation }) {
  const active = a.id === activeId;
  const editing = a.id === editingId;
  const quote = shortQuote(a.quote);

  return (
    <li
      className={cx("rounded-sm border p-3", active ? "border-accent bg-accent-soft" : "border-rule bg-paper-raised")}
    >
      <button
        type="button"
        aria-current={active ? "true" : undefined}
        onClick={() => {
          onSelect(a);
        }}
        className="block w-full text-left"
      >
        {/* The space is its own text node: one at the end of the hidden span would be trimmed from the name. */}
        <span className="sr-only">Show on page {a.pageNumber}:</span>{" "}
        <span lang="sr" className="line-clamp-3 font-serif text-ink">
          <mark className="bg-highlight text-ink [box-decoration-break:clone]">{a.quote}</mark>
        </span>
      </button>

      {editing ? (
        <NoteForm
          annotation={a}
          onSave={(note) => onSaveNote(a.id, note)}
          onCancel={() => {
            onEditNote(null);
          }}
        />
      ) : (
        <>
          {a.note && <p className="mt-2 text-sm whitespace-pre-line text-ink-muted">{a.note}</p>}
          <div className="mt-2 flex gap-4">
            <Button
              variant="quiet"
              size="sm"
              className="h-auto px-0"
              aria-label={a.note ? `Edit the note on “${quote}”` : `Add a note to “${quote}”`}
              onClick={() => {
                onEditNote(a.id);
              }}
            >
              {a.note ? "Edit note" : "Add note"}
            </Button>
            <Button
              variant="quiet"
              size="sm"
              className="h-auto px-0"
              aria-label={`Delete the highlight “${quote}”`}
              onClick={() => {
                onDelete(a);
              }}
            >
              Delete
            </Button>
          </div>
        </>
      )}
    </li>
  );
}

function NoteForm({
  annotation,
  onSave,
  onCancel,
}: {
  annotation: Annotation;
  onSave: (note: string) => Promise<void>;
  onCancel: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const fieldId = `note-${annotation.id}`;

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const value = new FormData(event.currentTarget).get("note");
    setError(null);
    setSaving(true);
    try {
      await onSave(typeof value === "string" ? value.trim() : "");
    } catch (err) {
      setError(
        err instanceof ApiError && err.fields["note"]
          ? err.fields["note"]
          : errorMessage(err, "Could not save the note. Try again."),
      );
      setSaving(false);
    }
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        void handleSubmit(event);
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape") onCancel();
      }}
      className="mt-3 grid gap-2"
    >
      <label htmlFor={fieldId} className="text-sm font-medium text-ink">
        Note
      </label>
      <textarea
        id={fieldId}
        name="note"
        rows={3}
        autoFocus
        maxLength={MAX_NOTE_LENGTH}
        defaultValue={annotation.note}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${fieldId}-error` : undefined}
        className={cx(
          "w-full rounded-sm border bg-paper-raised px-3 py-2 text-sm text-ink",
          error ? "border-danger" : "border-rule-strong hover:border-ink-muted",
        )}
      />
      {error && (
        <p id={`${fieldId}-error`} role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
      <div className="flex gap-3">
        <Button type="submit" size="sm" disabled={saving}>
          Save note
        </Button>
        <Button variant="secondary" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
