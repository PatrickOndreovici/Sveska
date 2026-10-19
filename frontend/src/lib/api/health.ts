/**
 * Client for GET /api/health (see api/openapi.yaml).
 * Kept free of React so it can be tested and reused without rendering.
 */

export type HealthState = "ok" | "database-unreachable" | "api-unreachable";

export async function fetchHealth(signal?: AbortSignal): Promise<HealthState> {
  let response: Response;
  try {
    response = await fetch("/api/health", signal ? { signal } : {});
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    return "api-unreachable";
  }

  if (response.status !== 200 && response.status !== 503) {
    return "api-unreachable";
  }

  const body: unknown = await response.json().catch(() => null);
  if (!isHealthBody(body)) return "api-unreachable";
  return body.database === "ok" ? "ok" : "database-unreachable";
}

interface HealthBody {
  status: "ok" | "unavailable";
  database: "ok" | "unreachable";
}

function isHealthBody(value: unknown): value is HealthBody {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return (
    (candidate["status"] === "ok" || candidate["status"] === "unavailable") &&
    (candidate["database"] === "ok" || candidate["database"] === "unreachable")
  );
}
