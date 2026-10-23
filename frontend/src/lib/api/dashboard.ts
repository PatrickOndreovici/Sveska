import { apiRequest } from "./client";
import type { Dashboard } from "./types";

/** The browser's time zone, so the API counts days from the student's own midnight. */
export function localTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

export async function getDashboard(timeZone: string, signal?: AbortSignal): Promise<Dashboard> {
  return apiRequest<Dashboard>("GET", `/api/dashboard?timeZone=${encodeURIComponent(timeZone)}`, undefined, signal);
}
