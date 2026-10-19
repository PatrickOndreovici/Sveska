/**
 * Sveska ("notebook") mark: an exercise-book page with its margin line.
 * Drawn with currentColor and the accent token so it follows light/dark mode.
 * Keep in sync with public/favicon.svg.
 */
export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 40 40" aria-hidden="true" className={className}>
      <rect x="7" y="4" width="26" height="32" rx="3" fill="none" stroke="currentColor" strokeWidth="3" />
      <line x1="14" y1="4" x2="14" y2="36" className="stroke-accent" strokeWidth="3" />
      <line x1="18" y1="14" x2="28" y2="14" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
      <line x1="18" y1="21" x2="28" y2="21" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}
