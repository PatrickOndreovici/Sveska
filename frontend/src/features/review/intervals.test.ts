import { describe, expect, it } from "vitest";
import { formatInterval } from "./intervals";

describe("formatInterval", () => {
  it.each([
    [0, "today"],
    [1, "1 day"],
    [2, "2 days"],
    [29, "29 days"],
    [30, "1 month"],
    [44, "1 month"],
    [45, "2 months"],
    [364, "12 months"],
    [365, "1 year"],
  ])("writes %i days as %s", (days, want) => {
    expect(formatInterval(days)).toBe(want);
  });
});
