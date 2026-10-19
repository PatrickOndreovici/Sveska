import { THEME_PREFERENCES, type ThemePreference } from "../lib/theme/theme";
import { useTheme } from "../lib/theme/useTheme";
import { cx } from "./ui/cx";

const labels: Record<ThemePreference, string> = {
  light: "Light",
  dark: "Dark",
  system: "System",
};

/** Three-way switch for the color theme. "System" follows the operating system setting. */
export function ThemeToggle() {
  const { preference, setPreference } = useTheme();

  return (
    <div role="group" aria-label="Color theme" className="inline-flex rounded-sm border border-rule p-0.5">
      {THEME_PREFERENCES.map((option) => (
        <button
          key={option}
          type="button"
          aria-pressed={preference === option}
          onClick={() => {
            setPreference(option);
          }}
          className={cx(
            "rounded-sm px-2.5 py-1 text-xs font-medium transition-colors",
            preference === option ? "bg-paper-sunken text-ink" : "text-ink-muted hover:text-ink",
          )}
        >
          {labels[option]}
        </button>
      ))}
    </div>
  );
}
