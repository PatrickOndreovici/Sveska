import { useState } from "react";
import { ApiError } from "../../lib/api/client";
import { validateEntry, type EntryErrors, type EntryFields } from "./entryFields";

/**
 * Submit handling shared by the add and edit forms: checks the required
 * fields, tracks the pending request, and turns API errors into messages.
 * Field problems go next to their inputs; anything else, such as a
 * duplicate word, becomes one message for the whole form.
 */
export function useEntryForm() {
  const [errors, setErrors] = useState<EntryErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  /** Resolves true when the request succeeded. */
  async function submit(fields: EntryFields, send: (fields: EntryFields) => Promise<void>): Promise<boolean> {
    setFormError(null);
    const found = validateEntry(fields);
    setErrors(found);
    if (Object.keys(found).length > 0) return false;

    setSubmitting(true);
    try {
      await send(fields);
      return true;
    } catch (error) {
      if (error instanceof ApiError && Object.keys(error.fields).length > 0) {
        setErrors(error.fields);
      } else if (error instanceof ApiError) {
        setFormError(error.message);
      } else {
        console.error("unexpected error while saving a word", error);
        setFormError("Something went wrong. Try again.");
      }
      return false;
    } finally {
      setSubmitting(false);
    }
  }

  function reset() {
    setErrors({});
    setFormError(null);
  }

  return { errors, formError, submitting, submit, reset };
}
