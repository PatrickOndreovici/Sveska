import { textValue } from "../auth/useAuthForm";

export const PARTS_OF_SPEECH = ["noun", "verb", "adjective", "other"] as const;
export const GENDERS = ["masculine", "feminine", "neuter"] as const;
export const ASPECTS = ["imperfective", "perfective", "biaspectual"] as const;

// "" means not set: every grammar field is optional.
export type PartOfSpeech = (typeof PARTS_OF_SPEECH)[number] | "";
export type Gender = (typeof GENDERS)[number] | "";
export type Aspect = (typeof ASPECTS)[number] | "";

/** The editable fields of a vocabulary entry, shared by the add and edit forms. */
export type EntryFields = {
  term: string;
  translation: string;
  notes: string;
  example: string;
  partOfSpeech: PartOfSpeech;
  // Nouns only.
  gender: Gender;
  plural: string;
  // Verbs only.
  aspect: Aspect;
  aspectPair: string;
  // Adjectives only; the term itself is the masculine form.
  feminineForm: string;
  neuterForm: string;
};

export type EntryErrors = Partial<Record<"term" | "translation", string>>;

export const DUPLICATE_MESSAGE = "You already have this word with this translation.";

export const partOfSpeechLabels: Record<Exclude<PartOfSpeech, "">, string> = {
  noun: "Noun · imenica",
  verb: "Verb · glagol",
  adjective: "Adjective · pridev",
  other: "Other",
};

export const genderLabels: Record<Exclude<Gender, "">, string> = {
  masculine: "Masculine · muški rod",
  feminine: "Feminine · ženski rod",
  neuter: "Neuter · srednji rod",
};

export const aspectLabels: Record<Exclude<Aspect, "">, string> = {
  imperfective: "Imperfective · nesvršeni",
  perfective: "Perfective · svršeni",
  biaspectual: "Both · dvovidski",
};

/** Short forms shown next to a word in the list, as a dictionary prints them. */
export const genderAbbreviations: Record<Exclude<Gender, "">, string> = {
  masculine: "m.",
  feminine: "f.",
  neuter: "n.",
};

export const aspectAbbreviations: Record<Exclude<Aspect, "">, string> = {
  imperfective: "impf.",
  perfective: "pf.",
  biaspectual: "impf./pf.",
};

function oneOf<T extends string>(allowed: readonly T[], value: string): T | "" {
  return (allowed as readonly string[]).includes(value) ? (value as T) : "";
}

/**
 * Reads the form and drops fields that do not belong to the chosen part of
 * speech, so switching a noun to a verb does not keep a stale gender.
 */
export function readEntryFields(data: FormData): EntryFields {
  const text = (name: string) => textValue(data, name).trim();
  const partOfSpeech = oneOf(PARTS_OF_SPEECH, text("partOfSpeech"));
  const isNoun = partOfSpeech === "noun";
  const isVerb = partOfSpeech === "verb";
  const isAdjective = partOfSpeech === "adjective";

  return {
    term: text("term"),
    translation: text("translation"),
    notes: text("notes"),
    example: text("example"),
    partOfSpeech,
    gender: isNoun ? oneOf(GENDERS, text("gender")) : "",
    plural: isNoun ? text("plural") : "",
    aspect: isVerb ? oneOf(ASPECTS, text("aspect")) : "",
    aspectPair: isVerb ? text("aspectPair") : "",
    feminineForm: isAdjective ? text("feminineForm") : "",
    neuterForm: isAdjective ? text("neuterForm") : "",
  };
}

/** Required-field checks only; the server repeats them along with the length limits. */
export function validateEntry(fields: EntryFields): EntryErrors {
  const errors: EntryErrors = {};
  if (fields.term === "") errors.term = "Enter the Serbian word.";
  if (fields.translation === "") errors.translation = "Enter a translation.";
  return errors;
}
