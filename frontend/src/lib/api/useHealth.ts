import { useEffect, useState } from "react";
import { fetchHealth, type HealthState } from "./health";

/** Checks API health once on mount. Returns "checking" until a result arrives. */
export function useHealth(): HealthState | "checking" {
  const [state, setState] = useState<HealthState | "checking">("checking");

  useEffect(() => {
    const controller = new AbortController();
    fetchHealth(controller.signal).then(setState, (error: unknown) => {
      if (!controller.signal.aborted) {
        console.error("health check failed", error);
        setState("api-unreachable");
      }
    });
    return () => {
      controller.abort();
    };
  }, []);

  return state;
}
