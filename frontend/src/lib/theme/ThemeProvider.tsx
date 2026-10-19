import { createContext, useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  applyTheme,
  DARK_MEDIA_QUERY,
  readPreference,
  resolveTheme,
  systemPrefersDark,
  writePreference,
  type ThemePreference,
} from "./theme";

export interface ThemeContextValue {
  preference: ThemePreference;
  setPreference: (preference: ThemePreference) => void;
}

export const ThemeContext = createContext<ThemeContextValue | null>(null);

/** Owns the theme preference and keeps <html data-theme> in sync with it and with the OS setting. */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreferenceState] = useState<ThemePreference>(readPreference);

  useEffect(() => {
    const apply = () => {
      applyTheme(resolveTheme(preference, systemPrefersDark()));
    };
    apply();

    if (preference !== "system" || typeof window.matchMedia !== "function") return undefined;
    const media = window.matchMedia(DARK_MEDIA_QUERY);
    media.addEventListener("change", apply);
    return () => {
      media.removeEventListener("change", apply);
    };
  }, [preference]);

  const setPreference = useCallback((next: ThemePreference) => {
    writePreference(next);
    setPreferenceState(next);
  }, []);

  const value = useMemo(() => ({ preference, setPreference }), [preference, setPreference]);
  return <ThemeContext value={value}>{children}</ThemeContext>;
}
