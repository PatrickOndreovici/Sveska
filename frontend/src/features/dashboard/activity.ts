import type { ActivityDay } from "../../lib/api/types";

const DAY = 24 * 60 * 60 * 1000;

/** Everything done on a day: words added, highlights made and pages worked on. */
export function dayTotal(day: ActivityDay): number {
  return day.words + day.highlights + day.pages;
}

/** The calendar's colour steps, by how much was done on a day. */
export const LEVELS = [
  { level: 1, min: 1, label: "1–2" },
  { level: 2, min: 3, label: "3–5" },
  { level: 3, min: 6, label: "6–9" },
  { level: 4, min: 10, label: "10+" },
] as const;

export type Level = 0 | (typeof LEVELS)[number]["level"];

/** 0 for a day with nothing done, otherwise its step in LEVELS. */
export function level(total: number): Level {
  let found: Level = 0;
  for (const step of LEVELS) if (total >= step.min) found = step.level;
  return found;
}

/** One column of the calendar: a week from Monday, null for days after today. */
export type Week = {
  start: string;
  days: (ActivityDay | null)[];
};

// Dates are calendar dates (YYYY-MM-DD) in the student's time zone; doing the
// arithmetic in UTC keeps a day 24 hours long.
function parseDate(date: string): number {
  return Date.parse(`${date}T00:00:00Z`);
}

function formatDate(time: number): string {
  return new Date(time).toISOString().slice(0, 10);
}

/**
 * Lays the days out as the last `count` calendar weeks, Monday first: the
 * last column is the week of today. Days missing from activity count as
 * nothing done.
 */
export function calendarWeeks(activity: readonly ActivityDay[], today: string, count = 12): Week[] {
  const byDate = new Map(activity.map((d) => [d.date, d]));
  const todayTime = parseDate(today);
  const weekday = (new Date(todayTime).getUTCDay() + 6) % 7; // Monday 0 … Sunday 6
  const first = todayTime - (weekday + 7 * (count - 1)) * DAY;

  return Array.from({ length: count }, (_, w) => ({
    start: formatDate(first + w * 7 * DAY),
    days: Array.from({ length: 7 }, (_, d) => {
      const time = first + (w * 7 + d) * DAY;
      if (time > todayTime) return null;
      const date = formatDate(time);
      return byDate.get(date) ?? { date, words: 0, highlights: 0, pages: 0 };
    }),
  }));
}

/**
 * The month to print above each week: on the first week of a month, and on
 * the first column unless the second one starts a month already (the two
 * labels would overlap).
 */
export function monthLabels(weeks: readonly Week[]): (string | null)[] {
  const month = (w: Week) => monthFormat.format(new Date(parseDate(w.start)));
  return weeks.map((week, i) => {
    const previous = weeks[i - 1];
    if (previous) return month(previous) === month(week) ? null : month(week);
    const next = weeks[1];
    return next && month(next) !== month(week) ? null : month(week);
  });
}

const monthFormat = new Intl.DateTimeFormat("en-GB", { month: "short", timeZone: "UTC" });
const dayFormat = new Intl.DateTimeFormat("en-GB", {
  weekday: "short",
  day: "numeric",
  month: "short",
  timeZone: "UTC",
});

/** "Fri 23 Oct" for 2026-10-23. */
export function formatDay(date: string): string {
  return dayFormat.format(new Date(parseDate(date)));
}

/** "2 words, 1 highlight, 3 pages", leaving out what is zero. */
export function describeDay(day: ActivityDay): string {
  const parts = [
    count(day.words, "word", "words"),
    count(day.highlights, "highlight", "highlights"),
    count(day.pages, "page written on", "pages written on"),
  ].filter((p) => p !== null);
  return parts.length > 0 ? parts.join(", ") : "Nothing done";
}

function count(n: number, one: string, many: string): string | null {
  if (n === 0) return null;
  return `${String(n)} ${n === 1 ? one : many}`;
}

/** The day before date. */
export function previousDay(date: string): string {
  return formatDate(parseDate(date) - DAY);
}
