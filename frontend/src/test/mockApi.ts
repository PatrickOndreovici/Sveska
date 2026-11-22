import { vi } from "vitest";
import type { ActivityDay, Dashboard } from "../lib/api/types";

export interface MockResponse {
  status: number;
  body?: unknown;
}

export type MockHandler = MockResponse | ((requestBody: unknown, url: URL) => MockResponse);

/**
 * Replaces fetch with a fake API keyed by "METHOD /path". A key with a query
 * string matches only that exact URL; a key without one matches any query,
 * and a function handler receives the URL to read it. Unknown routes fail the
 * test loudly. GET /api/health answers "ok" and GET /api/dashboard an empty
 * dashboard unless overridden.
 * Returns the spy so tests can inspect calls.
 */
export function mockApi(routes: Record<string, MockHandler>) {
  const all: Record<string, MockHandler> = {
    "GET /api/health": { status: 200, body: { status: "ok", database: "ok" } },
    "GET /api/dashboard": { status: 200, body: dashboard() },
    ...routes,
  };
  return vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
    const method = init?.method ?? "GET";
    const url = typeof input === "string" ? input : input instanceof URL ? input.pathname : input.url;
    const parsedUrl = new URL(url, "http://localhost");
    const handler = all[`${method} ${url}`] ?? all[`${method} ${parsedUrl.pathname}`];
    if (!handler) return Promise.reject(new Error(`Unexpected request in test: ${method} ${url}`));

    const requestBody: unknown = typeof init?.body === "string" ? JSON.parse(init.body) : undefined;
    const { status, body } = typeof handler === "function" ? handler(requestBody, parsedUrl) : handler;
    const payload = status === 204 ? null : JSON.stringify(body ?? {});
    return Promise.resolve(new Response(payload, { status, headers: { "Content-Type": "application/json" } }));
  });
}

export const student = {
  id: "019a0000-0000-7000-8000-000000000001",
  email: "ana@example.com",
  displayName: "Ana Petrović",
  role: "student",
  avatarUrl: null,
  createdAt: "2026-10-19T12:00:00Z",
} as const;

export const teacher = {
  id: "019a0000-0000-7000-8000-000000000002",
  email: "marko@example.com",
  displayName: "Marko Jovanović",
  role: "teacher",
  avatarUrl: null,
  createdAt: "2026-10-19T12:00:00Z",
} as const;

export const anonymousMe: MockResponse = {
  status: 401,
  body: { error: { code: "unauthenticated", message: "Log in to continue." } },
};

/**
 * A dashboard for Friday 23 October 2026 with nothing done; days lists the
 * ones with activity (reviews 0 unless given), and overrides replaces
 * anything else.
 */
export function dashboard(
  days: (Omit<ActivityDay, "reviews"> & { reviews?: number })[] = [],
  overrides: Partial<Dashboard> = {},
): Dashboard {
  const today = Date.parse("2026-10-23T00:00:00Z");
  const activity = Array.from({ length: 84 }, (_, i) => {
    const date = new Date(today - (83 - i) * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
    const day = days.find((d) => d.date === date);
    return { date, words: 0, highlights: 0, pages: 0, reviews: 0, ...day };
  });
  return {
    today: "2026-10-23",
    totals: { words: 0, wordsThisWeek: 0, lessons: 0, documents: 0, highlights: 0, notes: 0 },
    review: { due: 0, new: 0 },
    streak: 0,
    activity,
    recentWords: [],
    recentDocuments: [],
    ...overrides,
  };
}
