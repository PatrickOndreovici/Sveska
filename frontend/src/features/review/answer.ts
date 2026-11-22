import type { ReviewRating } from "../../lib/api/types";

/**
 * How a typed answer compares with the Serbian word: the same, the same
 * but for a č, ć, š, ž or đ, or something else.
 */
export type AnswerCheck = "right" | "letters" | "wrong";

/** The rating each outcome suggests; the student can still pick another. */
export const SUGGESTED_RATING: Record<AnswerCheck, ReviewRating> = {
  right: "good",
  letters: "hard",
  wrong: "again",
};

/**
 * Compares what the student typed with the word. Case, punctuation, extra
 * spaces and a part in brackets ("pas (psa)") do not count, and a word
 * saved as "kuća / dom" accepts either. Typing c for č or ć, s for š, z for
 * ž, or d or dj for đ is "letters": the word was known, the spelling not
 * quite.
 */
export function checkAnswer(typed: string, term: string): AnswerCheck {
  const answer = normalize(typed);
  if (answer === "") return "wrong";
  const accepted = [term, ...term.split(/[/,;]/)].map(normalize).filter(Boolean);
  if (accepted.includes(answer)) return "right";
  const plain = withoutDiacritics(answer);
  if (accepted.some((a) => withoutDiacritics(a) === plain)) return "letters";
  return "wrong";
}

function normalize(text: string): string {
  return (
    text
      // A keyboard may send č as c and a combining caron.
      .normalize("NFC")
      .replace(/\([^)]*\)/g, " ")
      .replace(/[^\p{L}\p{N}\s-]/gu, " ")
      .toLocaleLowerCase("sr")
      .replace(/\s+/g, " ")
      .trim()
  );
}

function withoutDiacritics(text: string): string {
  // đ has no combining form, so it is replaced first; dj is how it is often
  // typed without the letter.
  return text
    .replace(/dj|đ/g, "d")
    .normalize("NFD")
    .replace(/\p{M}/gu, "");
}
