import { describe, expect, it, vi } from "vitest";
import { ApiError, apiRequest } from "./client";

function respond(status: number, body: string | null) {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(body, { status }));
}

describe("apiRequest", () => {
  it("returns the parsed body on success", async () => {
    respond(200, JSON.stringify({ user: { id: "1" } }));
    await expect(apiRequest("GET", "/api/me")).resolves.toEqual({ user: { id: "1" } });
  });

  it("returns undefined for 204", async () => {
    respond(204, null);
    await expect(apiRequest("POST", "/api/auth/logout")).resolves.toBeUndefined();
  });

  it("sends JSON with the session cookie", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}", { status: 200 }));
    await apiRequest("POST", "/api/auth/login", { email: "a@b.rs" });
    expect(spy).toHaveBeenCalledWith("/api/auth/login", {
      method: "POST",
      credentials: "same-origin",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: '{"email":"a@b.rs"}',
    });
  });

  it("sends FormData as multipart, leaving the boundary to the browser", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}", { status: 201 }));
    const form = new FormData();
    form.append("file", new File(["%PDF-"], "a.pdf"));
    await apiRequest("POST", "/api/lessons/1/documents", form);
    const init = spy.mock.calls[0]?.[1];
    expect(init?.body).toBe(form);
    expect(init?.headers).toEqual({ Accept: "application/json" });
  });

  it("turns the error envelope into an ApiError", async () => {
    respond(422, JSON.stringify({ error: { code: "validation_failed", message: "Invalid.", fields: { email: "Bad." } } }));
    const error = await apiRequest("POST", "/api/auth/signup", {}).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 422, code: "validation_failed", message: "Invalid.", fields: { email: "Bad." } });
  });

  it("uses a generic message when the error body is not the envelope", async () => {
    respond(502, "<html>Bad gateway</html>");
    await expect(apiRequest("GET", "/api/me")).rejects.toMatchObject({ status: 502, code: "unexpected_response" });
  });

  it("reports network failures", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(apiRequest("GET", "/api/me")).rejects.toMatchObject({ status: 0, code: "network_error" });
  });
});
