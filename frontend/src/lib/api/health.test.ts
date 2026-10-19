import { describe, expect, it, vi } from "vitest";
import { fetchHealth } from "./health";

function mockFetch(status: number, body: unknown) {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(
    new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }),
  );
}

describe("fetchHealth", () => {
  it("returns ok when API and database are healthy", async () => {
    mockFetch(200, { status: "ok", database: "ok" });
    await expect(fetchHealth()).resolves.toBe("ok");
  });

  it("returns database-unreachable on 503", async () => {
    mockFetch(503, { status: "unavailable", database: "unreachable" });
    await expect(fetchHealth()).resolves.toBe("database-unreachable");
  });

  it("returns api-unreachable when the request fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(fetchHealth()).resolves.toBe("api-unreachable");
  });

  it("returns api-unreachable for an unexpected body", async () => {
    mockFetch(200, { hello: "world" });
    await expect(fetchHealth()).resolves.toBe("api-unreachable");
  });

  it("returns api-unreachable for an unexpected status", async () => {
    mockFetch(502, { status: "ok", database: "ok" });
    await expect(fetchHealth()).resolves.toBe("api-unreachable");
  });
});
