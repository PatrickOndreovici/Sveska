import { afterEach, describe, expect, it } from "vitest";
import { applyTheme, readPreference, resolveTheme, THEME_STORAGE_KEY, writePreference } from "./theme";

afterEach(() => {
  localStorage.clear();
});

describe("readPreference", () => {
  it("defaults to system when nothing is saved", () => {
    expect(readPreference()).toBe("system");
  });

  it("returns a saved valid preference", () => {
    writePreference("dark");
    expect(readPreference()).toBe("dark");
  });

  it("ignores an invalid saved value", () => {
    localStorage.setItem(THEME_STORAGE_KEY, "purple");
    expect(readPreference()).toBe("system");
  });
});

describe("resolveTheme", () => {
  it.each([
    ["light", false, "light"],
    ["light", true, "light"],
    ["dark", false, "dark"],
    ["dark", true, "dark"],
    ["system", false, "light"],
    ["system", true, "dark"],
  ] as const)("resolves %s with system dark=%s to %s", (preference, systemDark, expected) => {
    expect(resolveTheme(preference, systemDark)).toBe(expected);
  });
});

describe("applyTheme", () => {
  it("writes the theme to data-theme", () => {
    const root = document.createElement("div");
    applyTheme("dark", root);
    expect(root.dataset["theme"]).toBe("dark");
  });
});
