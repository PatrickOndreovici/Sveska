import type { ReviewRating } from "../../lib/api/types";

/** The four answers in button order, each with its key. */
export const RATINGS: readonly { rating: ReviewRating; label: string; key: string }[] = [
  { rating: "again", label: "Again", key: "1" },
  { rating: "hard", label: "Hard", key: "2" },
  { rating: "good", label: "Good", key: "3" },
  { rating: "easy", label: "Easy", key: "4" },
];

/** "today", "1 day", "12 days", "3 months", "1 year": when a word would come back. */
export function formatInterval(days: number): string {
  if (days === 0) return "today";
  if (days < 30) return days === 1 ? "1 day" : `${String(days)} days`;
  if (days < 365) {
    const months = Math.round(days / 30);
    return months === 1 ? "1 month" : `${String(months)} months`;
  }
  return "1 year";
}
