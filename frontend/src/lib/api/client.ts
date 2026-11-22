import type { ErrorEnvelope } from "./types";

/**
 * An error response from the API, in the envelope described by api/openapi.yaml.
 * `code` is stable and safe to branch on; `message` is safe to show to users.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Readonly<Record<string, string>>;

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

const NETWORK_ERROR_MESSAGE = "Could not reach Sveska. Check your connection and try again.";
const UNEXPECTED_ERROR_MESSAGE = "Something went wrong on our side. Try again.";

/**
 * Sends a request to the API. A FormData body goes out as multipart (for
 * uploads); any other body as JSON. Resolves with the parsed body (or
 * undefined for 204) and rejects with ApiError for any non-2xx response.
 * An aborted request rejects with the browser's AbortError instead, so
 * callers can tell a cancelled request from a failed one.
 */
export async function apiRequest<T>(
  method: "GET" | "POST" | "PUT" | "DELETE",
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  const init: RequestInit = { method, credentials: "same-origin", headers };
  if (body instanceof FormData) {
    // No Content-Type: the browser sets multipart/form-data with its boundary.
    init.body = body;
  } else if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  if (signal) init.signal = signal;

  let response: Response;
  try {
    response = await fetch(path, init);
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    throw new ApiError(0, "network_error", NETWORK_ERROR_MESSAGE);
  }

  if (response.status === 204) return undefined as T;
  const parsed: unknown = await response.json().catch(() => null);

  if (!response.ok) {
    if (isErrorEnvelope(parsed)) {
      const { code, message, fields } = parsed.error;
      throw new ApiError(response.status, code, message, fields ?? {});
    }
    throw new ApiError(response.status, "unexpected_response", UNEXPECTED_ERROR_MESSAGE);
  }
  // Successful bodies are trusted to match the contract: the backend is ours
  // and its handlers are tested against the same OpenAPI document.
  return parsed as T;
}

function isErrorEnvelope(value: unknown): value is ErrorEnvelope {
  if (typeof value !== "object" || value === null || !("error" in value)) return false;
  const error = value.error;
  return (
    typeof error === "object" &&
    error !== null &&
    typeof (error as Record<string, unknown>)["code"] === "string" &&
    typeof (error as Record<string, unknown>)["message"] === "string"
  );
}

/** The API's message for an ApiError, which is safe to show; the fallback for anything else. */
export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof ApiError ? error.message : fallback;
}
