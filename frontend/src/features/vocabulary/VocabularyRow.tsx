import { useEffect, useRef, useState, type SubmitEvent } from "react";
import { Button } from "../../components/ui/Button";
import { TextLink } from "../../components/ui/TextLink";
import type { VocabularyEntry } from "../../lib/api/types";
import {
  aspectAbbreviations,
  aspectLabels,
  formsOf,
  genderAbbreviations,
  genderLabels,
  readEntryFields,
  type EntryFields,
} from "./entryFields";
import { EntryInputs } from "./EntryInputs";
import { useEntryForm } from "./useEntryForm";

const dateFormat = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short" });

type VocabularyRowProps = {
  entry: VocabularyEntry;
  /** Saves the edit; rejects with an ApiError when the API refuses it. */
  onSave: (fields: EntryFields) => Promise<void>;
  onDelete: () => void;
};

/** The dictionary-style tag after the word: gender for nouns, aspect for verbs. */
export function GrammarTag({ entry }: { entry: EntryFields }) {
  if (entry.gender) {
    return (
      <abbr title={genderLabels[entry.gender]} className="text-sm text-ink-subtle italic no-underline">
        {genderAbbreviations[entry.gender]}
      </abbr>
    );
  }
  if (entry.aspect) {
    return (
      <abbr title={aspectLabels[entry.aspect]} className="text-sm text-ink-subtle italic no-underline">
        {aspectAbbreviations[entry.aspect]}
      </abbr>
    );
  }
  return null;
}

/** One list item that switches in place between reading and an edit form. */
export function VocabularyRow({ entry, onSave, onDelete }: VocabularyRowProps) {
  const [editing, setEditing] = useState(false);
  const { errors, formError, submitting, submit, reset } = useEntryForm();
  const editButton = useRef<HTMLButtonElement>(null);
  const wasEditing = useRef(false);

  // After Save or Cancel, put focus back on Edit instead of losing it to <body>.
  useEffect(() => {
    if (!editing && wasEditing.current) editButton.current?.focus();
    wasEditing.current = editing;
  }, [editing]);

  function startEditing() {
    reset();
    setEditing(true);
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = readEntryFields(new FormData(event.currentTarget));
    if (await submit(fields, onSave)) setEditing(false);
  }

  if (editing) {
    return (
      <li className="border-b border-rule py-4">
        <form
          noValidate
          aria-label={`Edit ${entry.term}`}
          onSubmit={(event) => {
            void handleSubmit(event);
          }}
          onKeyDown={(event) => {
            if (event.key === "Escape") setEditing(false);
          }}
          className="grid gap-4 sm:grid-cols-2"
        >
          <EntryInputs idPrefix={`vocabulary-edit-${entry.id}`} initial={entry} errors={errors} autoFocus twoColumns />
          {formError && (
            <p role="alert" className="text-sm text-danger sm:col-span-2">
              {formError}
            </p>
          )}
          <div className="flex gap-3 sm:col-span-2">
            <Button type="submit" size="sm" disabled={submitting}>
              Save
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                setEditing(false);
              }}
            >
              Cancel
            </Button>
          </div>
        </form>
      </li>
    );
  }

  const forms = formsOf(entry);

  return (
    <li className="grid gap-2 border-b border-rule py-4 sm:grid-cols-[1fr_auto] sm:gap-6">
      <div className="min-w-0">
        <p className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="flex items-baseline gap-1.5">
            <span lang="sr" className="font-serif text-xl text-ink">
              {entry.term}
            </span>
            <GrammarTag entry={entry} />
          </span>
          <span className="text-ink-muted">{entry.translation}</span>
        </p>
        {forms && (
          <p lang="sr" className="mt-1 text-sm text-ink-muted">
            {forms}
          </p>
        )}
        {entry.example && (
          <p lang="sr" className="mt-1 font-serif text-ink-muted italic">
            {entry.example}
          </p>
        )}
        {entry.notes && <p className="mt-1 text-sm text-ink-subtle">{entry.notes}</p>}
        {entry.source && (
          <p className="mt-1 text-sm text-ink-subtle">
            From{" "}
            <TextLink to={`/documents/${entry.source.documentId}?page=${String(entry.source.pageNumber)}`}>
              {entry.source.documentTitle}, page {entry.source.pageNumber}
            </TextLink>
          </p>
        )}
      </div>
      <div className="flex items-center gap-4 sm:justify-end">
        <time dateTime={entry.createdAt} className="text-sm text-ink-subtle">
          {dateFormat.format(new Date(entry.createdAt))}
        </time>
        <Button
          ref={editButton}
          variant="quiet"
          size="sm"
          className="px-0"
          aria-label={`Edit ${entry.term} (${entry.translation})`}
          onClick={startEditing}
        >
          Edit
        </Button>
        <Button
          variant="quiet"
          size="sm"
          className="px-0"
          aria-label={`Delete ${entry.term} (${entry.translation})`}
          onClick={onDelete}
        >
          Delete
        </Button>
      </div>
    </li>
  );
}
