import { vi } from "vitest";

export interface MockResponse {
  status: number;
  body?: unknown;
}

export type MockHandler = MockResponse | ((requestBody: unknown, url: URL) => MockResponse);

/**
 * Replaces fetch with a fake API keyed by "METHOD /path". A key with a query
 * string matches only that exact URL; a key without one matches any query,
 * and a function handler receives the URL to read it. Unknown routes fail the
 * test loudly. GET /api/health answers "ok" unless overridden.
 * Returns the spy so tests can inspect calls.
 */
export function mockApi(routes: Record<string, MockHandler>) {
  const all: Record<string, MockHandler> = {
    "GET /api/health": { status: 200, body: { status: "ok", database: "ok" } },
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
  createdAt: "2026-10-19T12:00:00Z",
} as const;

export const teacher = {
  id: "019a0000-0000-7000-8000-000000000002",
  email: "marko@example.com",
  displayName: "Marko Jovanović",
  role: "teacher",
  createdAt: "2026-10-19T12:00:00Z",
} as const;

export const anonymousMe: MockResponse = {
  status: 401,
  body: { error: { code: "unauthenticated", message: "Log in to continue." } },
};
