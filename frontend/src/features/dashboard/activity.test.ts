import { describe, expect, it } from "vitest";
import type { ActivityDay } from "../../lib/api/types";
import { calendarWeeks, describeDay, formatDay, level, monthLabels, previousDay } from "./activity";

function day(date: string, words = 0, highlights = 0, pages = 0): ActivityDay {
  return { date, words, highlights, pages };
}

describe("level", () => {
  it.each([
    [0, 0],
    [1, 1],
    [2, 1],
    [3, 2],
    [5, 2],
    [6, 3],
    [9, 3],
    [10, 4],
    [250, 4],
  ])("puts a total of %i at step %i", (total, want) => {
    expect(level(total)).toBe(want);
  });
});

describe("calendarWeeks", () => {
  // 23 October 2026 is a Friday.
  const weeks = calendarWeeks([day("2026-10-21", 2), day("2026-08-03", 0, 1)], "2026-10-23");

  it("ends with the week of today, from its Monday", () => {
    expect(weeks).toHaveLength(12);
    const last = weeks[11];
    expect(last?.start).toBe("2026-10-19");
    expect(last?.days.map((d) => d?.date ?? null)).toEqual([
      "2026-10-19",
      "2026-10-20",
      "2026-10-21",
      "2026-10-22",
      "2026-10-23",
      null,
      null,
    ]);
  });

  it("starts eleven weeks before, on a Monday", () => {
    expect(weeks[0]?.start).toBe("2026-08-03");
    expect(weeks[0]?.days[0]).toEqual(day("2026-08-03", 0, 1));
  });

  it("fills days without activity with zeros", () => {
    expect(weeks[11]?.days[2]).toEqual(day("2026-10-21", 2));
    expect(weeks[11]?.days[3]).toEqual(day("2026-10-22"));
  });

  it("fills the whole last week on a Sunday", () => {
    const sunday = calendarWeeks([], "2026-10-25");
    expect(sunday[11]?.days.every((d) => d !== null)).toBe(true);
    expect(sunday[11]?.start).toBe("2026-10-19");
  });

  it("keeps a day 24 hours long across a clock change", () => {
    // Europe changes clocks on 25 October 2026; dates must not shift.
    const across = calendarWeeks([], "2026-10-27");
    expect(across[11]?.days.map((d) => d?.date ?? null).slice(0, 2)).toEqual(["2026-10-26", "2026-10-27"]);
    expect(across[10]?.start).toBe("2026-10-19");
  });
});

describe("monthLabels", () => {
  it("labels the first week and each week a new month starts in", () => {
    const labels = monthLabels(calendarWeeks([], "2026-10-23"));
    // Weeks start 3 Aug, 10, 17, 24, 31 Aug, 7 Sept … 28 Sept, 5 Oct … 19 Oct.
    expect(labels).toEqual(["Aug", null, null, null, null, "Sept", null, null, null, "Oct", null, null]);
  });

  it("leaves the first week unlabelled when the second starts a month", () => {
    // Weeks start 25 Aug 2025, then 1 Sept.
    const labels = monthLabels(calendarWeeks([], "2025-11-14"));
    expect(labels[0]).toBeNull();
    expect(labels[1]).toBe("Sept");
  });
});

describe("day text", () => {
  it("formats a date", () => {
    expect(formatDay("2026-10-23")).toBe("Fri 23 Oct");
  });

  it("describes what was done, leaving out zeros", () => {
    expect(describeDay(day("2026-10-23", 2, 1, 3))).toBe("2 words, 1 highlight, 3 pages written on");
    expect(describeDay(day("2026-10-23", 1, 0, 1))).toBe("1 word, 1 page written on");
    expect(describeDay(day("2026-10-23"))).toBe("Nothing done");
  });

  it("finds the previous day across a month", () => {
    expect(previousDay("2026-11-01")).toBe("2026-10-31");
  });
});
