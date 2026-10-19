import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider } from "../lib/theme/ThemeProvider";
import { THEME_STORAGE_KEY } from "../lib/theme/theme";
import { ThemeToggle } from "./ThemeToggle";

function mockSystemDark(matches: boolean) {
  const listeners = new Set<() => void>();
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({
      matches,
      addEventListener: (_: string, fn: () => void) => listeners.add(fn),
      removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
    })),
  );
  return listeners;
}

function renderToggle() {
  render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>,
  );
}

afterEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset["theme"];
  vi.unstubAllGlobals();
});

describe("ThemeToggle", () => {
  it("starts on System and follows the OS setting", () => {
    mockSystemDark(true);
    renderToggle();
    expect(screen.getByRole("button", { name: "System" })).toHaveAttribute("aria-pressed", "true");
    expect(document.documentElement.dataset["theme"]).toBe("dark");
  });

  it("applies and saves an explicit choice", async () => {
    mockSystemDark(false);
    renderToggle();
    await userEvent.click(screen.getByRole("button", { name: "Dark" }));

    expect(document.documentElement.dataset["theme"]).toBe("dark");
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
    expect(screen.getByRole("button", { name: "Dark" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "System" })).toHaveAttribute("aria-pressed", "false");
  });

  it("restores a saved choice on load", () => {
    mockSystemDark(true);
    localStorage.setItem(THEME_STORAGE_KEY, "light");
    renderToggle();
    expect(screen.getByRole("button", { name: "Light" })).toHaveAttribute("aria-pressed", "true");
    expect(document.documentElement.dataset["theme"]).toBe("light");
  });

  it("stops following the OS once an explicit theme is chosen", async () => {
    const listeners = mockSystemDark(false);
    renderToggle();
    expect(listeners.size).toBe(1);
    await userEvent.click(screen.getByRole("button", { name: "Light" }));
    expect(listeners.size).toBe(0);
  });
});
