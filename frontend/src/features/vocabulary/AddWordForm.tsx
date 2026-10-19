import { useState, type SubmitEvent } from "react";
import { Button } from "../../components/ui/Button";
import { cx } from "../../components/ui/cx";
import { DUPLICATE_MESSAGE, readEntryFields, validateEntry, type EntryErrors, type EntryFields } from "./entryFields";
import { EntryInputs } from "./EntryInputs";

type AddWordFormProps = {
  /** Returns false when the word and translation already exist. */
  onAdd: (fields: EntryFields) => boolean;
};

/**
 * Always open on wide screens. On narrow ones it starts collapsed behind a
 * button, so the list is not pushed below a full form.
 */
export function AddWordForm({ onAdd }: AddWordFormProps) {
  const [open, setOpen] = useState(false);
  const [errors, setErrors] = useState<EntryErrors>({});
  const [duplicate, setDuplicate] = useState(false);
  const [added, setAdded] = useState<EntryFields | null>(null);
  // A new key after each add remounts the inputs: every field, the part of
  // speech included, starts empty again and Serbian gets the focus.
  const [formKey, setFormKey] = useState(0);

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = readEntryFields(new FormData(event.currentTarget));
    const found = validateEntry(fields);
    setErrors(found);
    if (Object.keys(found).length > 0) return;

    const ok = onAdd(fields);
    setDuplicate(!ok);
    if (!ok) return;
    setAdded(fields);
    setFormKey(formKey + 1);
  }

  // Any typing replaces the last outcome; the confirmation is for the word just added.
  function clearOutcome() {
    setDuplicate(false);
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
        onSubmit={handleSubmit}
        onInput={clearOutcome}
        className={cx("mt-5 gap-4 lg:grid", open ? "grid" : "hidden")}
      >
        <EntryInputs key={formKey} idPrefix="vocabulary-add" errors={errors} autoFocus={formKey > 0} />
        {duplicate && (
          <p role="alert" className="text-sm text-danger">
            {DUPLICATE_MESSAGE}
          </p>
        )}
        <Button type="submit" className="mt-1 w-full">
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
