import type { ComponentProps } from "react";
import { cx } from "./cx";

export type TextFieldProps = Omit<ComponentProps<"input">, "id" | "className"> & {
  id: string;
  label: string;
  hint?: string;
  error?: string | undefined;
};

/** Labelled text input with an optional hint and an inline error tied to it for screen readers. */
export function TextField({ id, label, hint, error, ...inputProps }: TextFieldProps) {
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(" ") || undefined;

  return (
    <div className="grid gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-ink">
        {label}
      </label>
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={cx(
          "h-10 w-full rounded-sm border bg-paper-raised px-3 text-ink placeholder:text-ink-subtle",
          error ? "border-danger" : "border-rule-strong hover:border-ink-muted",
        )}
        {...inputProps}
      />
      {hint && !error && (
        <p id={hintId} className="text-sm text-ink-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  );
}
