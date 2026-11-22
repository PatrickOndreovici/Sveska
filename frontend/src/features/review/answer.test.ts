import { describe, expect, it } from "vitest";
import { checkAnswer } from "./answer";

describe("checkAnswer", () => {
  it("accepts the word however it is cased and spaced", () => {
    expect(checkAnswer("kuća", "kuća")).toBe("right");
    expect(checkAnswer("  Kuća ", "kuća")).toBe("right");
    expect(checkAnswer("dobar  dan", "Dobar dan!")).toBe("right");
    // č typed as c and a combining caron.
    expect(checkAnswer("čitati", "čitati")).toBe("right");
  });

  it("leaves out a part in brackets and accepts each of several words", () => {
    expect(checkAnswer("pas", "pas (psa)")).toBe("right");
    expect(checkAnswer("dom", "kuća / dom")).toBe("right");
    expect(checkAnswer("kuća", "kuća, dom")).toBe("right");
  });

  it("tells a missing č, ć, š, ž or đ from a wrong word", () => {
    expect(checkAnswer("kuca", "kuća")).toBe("letters");
    expect(checkAnswer("citati", "čitati")).toBe("letters");
    expect(checkAnswer("sesir", "šešir")).toBe("letters");
    expect(checkAnswer("zena", "žena")).toBe("letters");
    expect(checkAnswer("djak", "đak")).toBe("letters");
    expect(checkAnswer("dak", "đak")).toBe("letters");
    expect(checkAnswer("kuča", "kuća")).toBe("letters");
    expect(checkAnswer("dom", "kuca / dom")).toBe("right");

    expect(checkAnswer("kuce", "kuća")).toBe("wrong");
    expect(checkAnswer("stan", "kuća")).toBe("wrong");
  });

  it("counts nothing typed as wrong", () => {
    expect(checkAnswer("", "kuća")).toBe("wrong");
    expect(checkAnswer(" (?) ", "kuća")).toBe("wrong");
  });
});
