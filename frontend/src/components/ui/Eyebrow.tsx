import type { ReactNode } from "react";

/** Small uppercase label set above a heading, as in a printed section opener. */
export function Eyebrow({ children }: { children: ReactNode }) {
  return <p className="text-eyebrow font-medium uppercase text-ink-muted">{children}</p>;
}
