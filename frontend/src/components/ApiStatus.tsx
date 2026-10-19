import type { HealthState } from "../lib/api/health";
import { cx } from "./ui/cx";

const labels: Record<HealthState | "checking", string> = {
  checking: "Checking API…",
  ok: "API and database connected",
  "database-unreachable": "API running, database unreachable",
  "api-unreachable": "API unreachable",
};

const dotClasses: Record<HealthState | "checking", string> = {
  checking: "bg-rule-strong",
  ok: "bg-success",
  "database-unreachable": "bg-warning",
  "api-unreachable": "bg-danger",
};

/** Presentational: shows a health state. Fetching lives in useHealth. */
export function ApiStatus({ state }: { state: HealthState | "checking" }) {
  return (
    <p role="status" className="inline-flex items-center gap-2 text-sm text-ink-muted">
      <span aria-hidden="true" className={cx("size-2 rounded-full", dotClasses[state])} />
      {labels[state]}
    </p>
  );
}
