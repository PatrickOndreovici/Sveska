import { useState, type SubmitEvent } from "react";
import { Button } from "../../components/ui/Button";
import { cx } from "../../components/ui/cx";
import { readEntryFields, type EntryFields } from "./entryFields";
import { EntryInputs } from "./EntryInputs";
import { useEntryForm } from "./useEntryForm";

type AddWordFormProps = {
  /** Saves the word; rejects with an ApiError when the API refuses it. */
  onAdd: (fields: EntryFields) => Promise<void>;
};

/**
 * Always open on wide screens. On narrow ones it starts collapsed behind a
 * button, so the list is not pushed below a full form.
 */
export function AddWordForm({ onAdd }: AddWordFormProps) {
  const [open, setOpen] = useState(false);
  const { errors, formError, submitting, submit, reset } = useEntryForm();
  const [added, setAdded] = useState<EntryFields | null>(null);
  // A new key after each add remounts the inputs: every field, the part of
  // speech included, starts empty again and Serbian gets the focus.
  const [formKey, setFormKey] = useState(0);

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = readEntryFields(new FormData(event.currentTarget));
    setAdded(null);
    if (!(await submit(fields, onAdd))) return;
    setAdded(fields);
    setFormKey(formKey + 1);
  }

  // Any typing replaces the last outcome; the confirmation is for the word just added.
  function clearOutcome() {
    if (formError !== null) reset();
    setAdded(null);
  }

  return (
    <section
      aria-labelledby="vocabulary-add-heading"
      className={cx(
        "lg:sticky lg:top-6 lg:rounded-md lg:border lg:border-rule lg:bg-paper-raised lg:p-6",
        // Collapsed on a phone it is just the button, without an empty card around it.
        open && "rounded-md border border-rule bg-paper-raised p-4 sm:p-6",
      )}
    >
      <h2 id="vocabulary-add-heading" className="hidden font-serif text-xl text-ink lg:block">
        Add a word
      </h2>
      <Button
        variant={open ? "secondary" : "primary"}
        aria-expanded={open}
        aria-controls="vocabulary-add-form"
        className="w-full lg:hidden"
        onClick={() => {
          setOpen(!open);
        }}
      >
        {open ? "Close" : "Add a word"}
      </Button>

      <form
        id="vocabulary-add-form"
        noValidate
        onSubmit={(event) => {
          void handleSubmit(event);
        }}
        onInput={clearOutcome}
        className={cx("mt-5 gap-4 lg:grid", open ? "grid" : "hidden")}
      >
        <EntryInputs key={formKey} idPrefix="vocabulary-add" errors={errors} autoFocus={formKey > 0} />
        {formError && (
          <p role="alert" className="text-sm text-danger">
            {formError}
          </p>
        )}
        <Button type="submit" className="mt-1 w-full" disabled={submitting}>
          Add word
        </Button>
        {/* Always rendered so screen readers announce the text when it appears. */}
        <p role="status" className="text-sm text-success empty:hidden">
          {added && (
            <>
              Added{" "}
              <span lang="sr" className="font-medium">
                {added.term}
              </span>{" "}
              ({added.translation}).
            </>
          )}
        </p>
      </form>
    </section>
  );
}
