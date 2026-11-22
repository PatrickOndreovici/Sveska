import { describe, expect, it } from "vitest";
import { wrapLines } from "./paintInk";

/** Every character is 10 wide. */
const measure = (s: string) => s.length * 10;

describe("wrapLines", () => {
  it("keeps text that fits on one line", () => {
    expect(wrapLines("Čokolada je slatka.", 1000, measure)).toEqual(["Čokolada je slatka."]);
  });

  it("starts a new line at each line break, keeping empty lines", () => {
    expect(wrapLines("sir\n\nmed", 1000, measure)).toEqual(["sir", "", "med"]);
  });

  it("wraps between words", () => {
    expect(wrapLines("kafa je gorka", 90, measure)).toEqual(["kafa je", "gorka"]);
  });

  it("breaks a word only when it is wider than a line", () => {
    expect(wrapLines("ab slatkiši", 40, measure)).toEqual(["ab", "slat", "kiši"]);
  });
});
