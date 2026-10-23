import { useEffect, useId, useRef, type SubmitEvent } from "react";
import { Button } from "../../components/ui/Button";
import { readEntryFields, type EntryFields } from "../vocabulary/entryFields";
import { EntryInputs } from "../vocabulary/EntryInputs";
import { useEntryForm } from "../vocabulary/useEntryForm";

type SaveWordDialogProps = {
  /** The selected text, which starts as the Serbian word; null keeps the dialog closed. */
  quote: string | null;
  /** Saves the word; rejects with an ApiError when the API refuses it. */
  onSave: (fields: EntryFields) => Promise<void>;
  onClose: () => void;
};

/** The vocabulary page's add form in a modal, for a word selected in a PDF. */
export function SaveWordDialog({ quote, onSave, onClose }: SaveWordDialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const { errors, formError, submitting, submit, reset } = useEntryForm();

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (quote !== null && !dialog.open) dialog.showModal();
    if (quote === null && dialog.open) dialog.close();
  }, [quote]);

  function close() {
    reset();
    onClose();
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (await submit(readEntryFields(new FormData(event.currentTarget)), onSave)) close();
  }

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onClose={close}
      className="m-auto max-h-[calc(100%-2rem)] w-[calc(100%-2rem)] max-w-md overflow-auto rounded-md border border-rule bg-paper-raised p-6 text-ink backdrop:bg-[rgb(0_0_0/0.5)]"
    >
      <h2 id={titleId} className="font-serif text-xl">
        Save to vocabulary
      </h2>
      <p className="mt-2 text-sm text-ink-muted">The word stays highlighted in the PDF and links back to this page.</p>
      {quote !== null && (
        <form
          noValidate
          onSubmit={(event) => {
            void handleSubmit(event);
          }}
          className="mt-5 grid gap-4"
        >
          {/* Keyed by the quote, so each selection starts from a fresh form. */}
          <EntryInputs
            key={quote}
            idPrefix="save-word"
            initial={{
              term: quote,
              translation: "",
              notes: "",
              example: "",
              partOfSpeech: "",
              gender: "",
              plural: "",
              aspect: "",
              aspectPair: "",
              feminineForm: "",
              neuterForm: "",
            }}
            errors={errors}
            autoFocus
          />
          {formError && (
            <p role="alert" className="text-sm text-danger">
              {formError}
            </p>
          )}
          <div className="mt-1 flex justify-end gap-3">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              Save word
            </Button>
          </div>
        </form>
      )}
    </dialog>
  );
}
