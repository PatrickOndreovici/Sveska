import { TextField } from "../../components/ui/TextField";
import type { LessonInput } from "../../lib/api/types";

export type LessonErrors = Partial<Record<keyof LessonInput, string>>;

/** Reads the lesson form and checks the one required field before the API does. */
export function readLessonForm(form: HTMLFormElement): { input: LessonInput; errors: LessonErrors } {
  const data = new FormData(form);
  const text = (name: string) => {
    const value = data.get(name);
    return typeof value === "string" ? value.trim() : "";
  };
  const input = { title: text("title"), description: text("description") };
  return { input, errors: input.title === "" ? { title: "Enter a title." } : {} };
}

/** The inputs shared by the new-lesson form and the edit form. */
export function LessonFields({
  idPrefix,
  initial,
  errors,
  autoFocus = false,
}: {
  idPrefix: string;
  initial?: LessonInput;
  errors: LessonErrors;
  autoFocus?: boolean;
}) {
  return (
    <>
      <TextField
        id={`${idPrefix}-title`}
        name="title"
        label="Title"
        placeholder="e.g. Lekcija 3 – Padeži"
        defaultValue={initial?.title}
        error={errors.title}
        autoFocus={autoFocus}
        maxLength={200}
      />
      <TextField
        id={`${idPrefix}-description`}
        name="description"
        label="Description"
        hint="Optional."
        defaultValue={initial?.description}
        error={errors.description}
        maxLength={2000}
      />
    </>
  );
}
