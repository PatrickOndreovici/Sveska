import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { anonymousMe, mockApi, student, teacher } from "../test/mockApi";
import { renderApp } from "../test/renderApp";

describe("routing and access", () => {
  it("sends anonymous visitors from the workspace to the login page", async () => {
    mockApi({ "GET /api/me": anonymousMe });
    const router = renderApp("/");
    expect(await screen.findByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/login");
  });

  it("shows a student their workspace and student navigation", async () => {
    mockApi({ "GET /api/me": { status: 200, body: { user: student } } });
    renderApp("/");
    expect(await screen.findByRole("heading", { level: 1, name: "Dobar dan, Ana." })).toBeInTheDocument();
    const nav = screen.getByRole("navigation", { name: "Main" });
    expect(within(nav).getByRole("link", { name: "My workspace" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByText("Student")).toBeInTheDocument();
    expect(await screen.findByText("Day streak")).toBeInTheDocument();
  });

  it("shows a teacher the teacher navigation", async () => {
    mockApi({ "GET /api/me": { status: 200, body: { user: teacher } } });
    renderApp("/");
    expect(await screen.findByRole("heading", { level: 1, name: "Dobar dan, Marko." })).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Main" })).getByRole("link", { name: "Teaching" })).toBeInTheDocument();
    expect(screen.getByText("Teacher")).toBeInTheDocument();
    expect(screen.getByText(/classes, assignments and student work/)).toBeInTheDocument();
  });

  it.each(["/lessons", "/lessons/l1", "/documents/d1", "/vocabulary"])(
    "sends a teacher from the student page %s to their workspace",
    async (path) => {
      const fetchSpy = mockApi({ "GET /api/me": { status: 200, body: { user: teacher } } });
      const router = renderApp(path);
      expect(await screen.findByRole("heading", { level: 1, name: "Dobar dan, Marko." })).toBeInTheDocument();
      expect(router.state.location.pathname).toBe("/");
      const studentApi = fetchSpy.mock.calls.filter(
        ([url]) => typeof url === "string" && /^\/api\/(lessons|documents|vocabulary)/.test(url),
      );
      expect(studentApi).toEqual([]);
    },
  );

  it("sends logged-in users away from the login and sign-up pages", async () => {
    mockApi({ "GET /api/me": { status: 200, body: { user: student } } });
    const router = renderApp("/signup");
    expect(await screen.findByRole("heading", { level: 1, name: "Dobar dan, Ana." })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("logs out and returns to the login page", async () => {
    const fetchSpy = mockApi({
      "GET /api/me": { status: 200, body: { user: student } },
      "POST /api/auth/logout": { status: 204 },
    });
    renderApp("/");
    await userEvent.click(await screen.findByRole("button", { name: "Log out" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Main" })).not.toBeInTheDocument();
    expect(fetchSpy).toHaveBeenCalledWith("/api/auth/logout", expect.objectContaining({ method: "POST" }));
  });

  it("renders the not-found page for unknown paths", async () => {
    mockApi({ "GET /api/me": anonymousMe });
    renderApp("/no/such/page");
    expect(await screen.findByRole("heading", { level: 1, name: "This page does not exist." })).toBeInTheDocument();
  });
});
