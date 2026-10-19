/**
 * Theme preference logic, free of React so it can be tested directly.
 *
 * The user picks a preference ("light", "dark" or "system"). The app always
 * resolves it to a concrete theme and writes that to <html data-theme>, which
 * the CSS tokens in styles/index.css key off.
 *
 * The inline script in index.html repeats a minimal version of
 * readPreference + resolveTheme to avoid a flash before React loads.
 * Keep THEME_STORAGE_KEY and the resolution rule in sync with it.
 */

export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

export const THEME_PREFERENCES: readonly ThemePreference[] = ["light", "dark", "system"];
export const THEME_STORAGE_KEY = "sveska.theme";
export const DARK_MEDIA_QUERY = "(prefers-color-scheme: dark)";

function isThemePreference(value: unknown): value is ThemePreference {
  return value === "light" || value === "dark" || value === "system";
}

/** Reads the saved preference. Falls back to "system" if missing, invalid, or storage is blocked. */
export function readPreference(): ThemePreference {
  try {
    const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
    return isThemePreference(stored) ? stored : "system";
  } catch {
    return "system";
  }
}

/** Saves the preference. Storage can be unavailable (private mode); the choice then lasts for the session only. */
export function writePreference(preference: ThemePreference): void {
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, preference);
  } catch {
    // Ignore: the in-memory preference still applies.
  }
}

export function resolveTheme(preference: ThemePreference, systemPrefersDark: boolean): ResolvedTheme {
  if (preference === "system") return systemPrefersDark ? "dark" : "light";
  return preference;
}

export function systemPrefersDark(): boolean {
  return typeof window.matchMedia === "function" && window.matchMedia(DARK_MEDIA_QUERY).matches;
}

export function applyTheme(theme: ResolvedTheme, root: HTMLElement = document.documentElement): void {
  root.dataset["theme"] = theme;
}
