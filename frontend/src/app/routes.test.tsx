import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider } from "../lib/theme/ThemeProvider";
import { routes } from "./routes";

function renderAt(path: string) {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(
    new Response(JSON.stringify({ status: "ok", database: "ok" }), { status: 200 }),
  );
  render(
    <ThemeProvider>
      <RouterProvider router={createMemoryRouter(routes, { initialEntries: [path] })} />
    </ThemeProvider>,
  );
}

describe("routes", () => {
  it("renders the home page inside the layout", async () => {
    renderAt("/");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(/place to read, annotate/);
    expect(screen.getByRole("link", { name: "Sveska" })).toHaveAttribute("href", "/");
    expect(await screen.findByRole("status")).toHaveTextContent("API and database connected");
  });

  it("renders the not-found page for unknown paths", () => {
    renderAt("/no/such/page");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("This page does not exist.");
    expect(screen.getByRole("link", { name: "return to the workspace" })).toHaveAttribute("href", "/");
  });
});
