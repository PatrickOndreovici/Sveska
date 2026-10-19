import type { ReactNode } from "react";

/** Form-level error announced to screen readers as soon as it appears. */
export function FormAlert({ children }: { children: ReactNode }) {
  return (
    <div role="alert" className="rounded-sm border border-danger bg-paper-raised px-4 py-3 text-sm text-danger">
      {children}
    </div>
  );
}
