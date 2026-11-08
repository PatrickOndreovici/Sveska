import { useRef, useState, type FocusEvent } from "react";
import { SerbianLetters } from "../../components/SerbianLetters";
import { SelectField } from "../../components/ui/SelectField";
import { cx } from "../../components/ui/cx";
import { TextField } from "../../components/ui/TextField";
import {
  ASPECTS,
  aspectLabels,
  GENDERS,
  genderLabels,
  PARTS_OF_SPEECH,
  partOfSpeechLabels,
  type EntryErrors,
  type EntryFields,
  type PartOfSpeech,
} from "./entryFields";

type EntryInputsProps = {
  /** Makes every input id unique when several forms are on the page. */
  idPrefix: string;
  initial?: EntryFields;
  errors: EntryErrors;
  autoFocus?: boolean;
  /** Two columns from the sm breakpoint (the edit row); one column otherwise (the sidebar). */
  twoColumns?: boolean;
};

/**
 * The inputs of the add and edit forms, rendered without a wrapper so the
 * parent form's grid lays them out. Grammar inputs follow the part of speech.
 */
export function EntryInputs({ idPrefix, initial, errors, autoFocus = false, twoColumns = false }: EntryInputsProps) {
  const [partOfSpeech, setPartOfSpeech] = useState<PartOfSpeech>(initial?.partOfSpeech ?? "");
  const id = (name: string) => `${idPrefix}-${name}`;
  // Spanning is only applied in a two-column grid: in a one-column grid
  // col-span-2 would create a second column.
  const fullWidth = twoColumns ? "sm:col-span-2" : undefined;
  // The Serbian field used last, which the letter buttons type into: the
  // word itself until another is used, or once that one is gone.
  const lastSerbianField = useRef<HTMLInputElement | null>(null);
  const rememberField = (event: FocusEvent<HTMLInputElement>) => {
    lastSerbianField.current = event.currentTarget;
  };
  const serbianField = () => {
    const field = lastSerbianField.current;
    if (field?.isConnected) return field;
    const term = document.getElementById(id("term"));
    return term instanceof HTMLInputElement ? term : null;
  };

  return (
    <>
      <TextField
        id={id("term")}
        name="term"
        label="Serbian"
        lang="sr"
        spellCheck={false}
        defaultValue={initial?.term}
        autoFocus={autoFocus}
        error={errors.term}
        onFocus={rememberField}
      />
      <TextField
        id={id("translation")}
        name="translation"
        label="Translation"
        defaultValue={initial?.translation}
        error={errors.translation}
      />
      <div className={cx("flex flex-wrap items-center gap-x-3 gap-y-1.5", fullWidth)}>
        <span className="text-sm text-ink-muted">Letters for the Serbian fields</span>
        <SerbianLetters target={serbianField} />
      </div>

      <div className={fullWidth}>
        <SelectField
          id={id("partOfSpeech")}
          name="partOfSpeech"
          label="Part of speech"
          value={partOfSpeech}
          onChange={(event) => {
            setPartOfSpeech(event.target.value as PartOfSpeech);
          }}
        >
          <option value="">Not set</option>
          {PARTS_OF_SPEECH.map((value) => (
            <option key={value} value={value}>
              {partOfSpeechLabels[value]}
            </option>
          ))}
        </SelectField>
      </div>

      {partOfSpeech === "noun" && (
        <>
          <SelectField id={id("gender")} name="gender" label="Gender" defaultValue={initial?.gender ?? ""}>
            <option value="">Not set</option>
            {GENDERS.map((value) => (
              <option key={value} value={value}>
                {genderLabels[value]}
              </option>
            ))}
          </SelectField>
          <TextField
            id={id("plural")}
            name="plural"
            label="Plural"
            lang="sr"
            spellCheck={false}
            placeholder="e.g. kuće, ljudi"
            defaultValue={initial?.plural}
            error={errors.plural}
            onFocus={rememberField}
          />
        </>
      )}

      {partOfSpeech === "verb" && (
        <>
          <SelectField id={id("aspect")} name="aspect" label="Aspect" defaultValue={initial?.aspect ?? ""}>
            <option value="">Not set</option>
            {ASPECTS.map((value) => (
              <option key={value} value={value}>
                {aspectLabels[value]}
              </option>
            ))}
          </SelectField>
          <TextField
            id={id("aspectPair")}
            name="aspectPair"
            label="Aspect pair"
            lang="sr"
            spellCheck={false}
            placeholder="e.g. pročitati"
            defaultValue={initial?.aspectPair}
            error={errors.aspectPair}
            onFocus={rememberField}
          />
        </>
      )}

      {partOfSpeech === "adjective" && (
        <>
          <TextField
            id={id("feminineForm")}
            name="feminineForm"
            label="Feminine form"
            lang="sr"
            spellCheck={false}
            placeholder="e.g. dobra"
            defaultValue={initial?.feminineForm}
            error={errors.feminineForm}
            onFocus={rememberField}
          />
          <TextField
            id={id("neuterForm")}
            name="neuterForm"
            label="Neuter form"
            lang="sr"
            spellCheck={false}
            placeholder="e.g. dobro"
            defaultValue={initial?.neuterForm}
            error={errors.neuterForm}
            onFocus={rememberField}
          />
        </>
      )}

      <div className={fullWidth}>
        <TextField
          id={id("example")}
          name="example"
          label="Example sentence"
          lang="sr"
          placeholder="e.g. Čitam knjigu."
          defaultValue={initial?.example}
          error={errors.example}
          onFocus={rememberField}
        />
      </div>
      <div className={fullWidth}>
        <TextField id={id("notes")} name="notes" label="Notes" defaultValue={initial?.notes} error={errors.notes} />
      </div>
    </>
  );
}
