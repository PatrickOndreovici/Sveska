import { useEffect, useRef, useState } from "react";
import { ApiError } from "../../lib/api/client";

export type FieldErrors = Partial<Record<string, string>>;

/**
 * Shared submit handling for the auth forms: tracks the pending state, turns
 * API errors into field or form messages, and moves focus to the first
 * invalid field after a failed submit.
 */
export function useAuthForm() {
  const formRef = useRef<HTMLFormElement>(null);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  useEffect(() => {
    const firstInvalid = formRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]');
    firstInvalid?.focus();
  }, [fieldErrors]);

  async function submit(action: () => Promise<void>, clientErrors: FieldErrors = {}) {
    setFormError(null);
    if (Object.keys(clientErrors).length > 0) {
      setFieldErrors(clientErrors);
      return;
    }
    setFieldErrors({});
    setSubmitting(true);
    try {
      await action();
    } catch (error) {
      if (error instanceof ApiError && Object.keys(error.fields).length > 0) {
        setFieldErrors(error.fields);
      } else if (error instanceof ApiError && error.code === "email_taken") {
        setFieldErrors({ email: error.message });
      } else if (error instanceof ApiError) {
        setFormError(error.message);
      } else {
        console.error("unexpected error while submitting", error);
        setFormError("Something went wrong. Try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return { formRef, submitting, formError, fieldErrors, submit };
}

/** Reads a text value from FormData; file inputs and missing fields become "". */
export function textValue(data: FormData, name: string): string {
  const value = data.get(name);
  return typeof value === "string" ? value : "";
}
