import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { anonymousMe, mockApi, student } from "../test/mockApi";
import { renderApp } from "../test/renderApp";

// jsdom ignores CSS, so the wide-screen controls are present too.
// These tests look only inside the menu panel.
function menuPanel() {
  const panel = document.getElementById("site-menu");
  if (!panel) throw new Error("menu panel is not open");
  return within(panel);
}

describe("SiteHeader menu", () => {
  it("opens a menu with navigation, account and theme controls", async () => {
    mockApi({ "GET /api/me": { status: 200, body: { user: student } } });
    renderApp("/");
    const menuButton = await screen.findByRole("button", { name: "Menu" });
    expect(menuButton).toHaveAttribute("aria-expanded", "false");
    expect(document.getElementById("site-menu")).toBeNull();

    await userEvent.click(menuButton);

    expect(menuButton).toHaveAttribute("aria-expanded", "true");
    const panel = menuPanel();
    expect(panel.getByRole("link", { name: "My workspace" })).toHaveAttribute("aria-current", "page");
    expect(panel.getByText("Ana Petrović")).toBeInTheDocument();
    expect(panel.getByText("AP")).toBeInTheDocument();
    expect(panel.getByText("Student")).toBeInTheDocument();
    expect(panel.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/settings");
    expect(panel.getByRole("group", { name: "Color theme" })).toBeInTheDocument();
    expect(panel.getByRole("button", { name: "Log out" })).toBeInTheDocument();
  });

  it("closes on Escape and returns focus to the menu button", async () => {
    mockApi({ "GET /api/me": { status: 200, body: { user: student } } });
    renderApp("/");
    const menuButton = await screen.findByRole("button", { name: "Menu" });
    await userEvent.click(menuButton);
    await userEvent.keyboard("{Escape}");

    expect(document.getElementById("site-menu")).toBeNull();
    expect(menuButton).toHaveFocus();
  });

  it("closes when a link is chosen, even the current page", async () => {
    mockApi({ "GET /api/me": { status: 200, body: { user: student } } });
    renderApp("/");
    await userEvent.click(await screen.findByRole("button", { name: "Menu" }));
    await userEvent.click(menuPanel().getByRole("link", { name: "My workspace" }));
    expect(document.getElementById("site-menu")).toBeNull();
  });

  it("logs out from the menu", async () => {
    mockApi({
      "GET /api/me": { status: 200, body: { user: student } },
      "POST /api/auth/logout": { status: 204 },
    });
    renderApp("/");
    await userEvent.click(await screen.findByRole("button", { name: "Menu" }));
    await userEvent.click(menuPanel().getByRole("button", { name: "Log out" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Menu" })).not.toBeInTheDocument();
  });

  it("shows the profile picture beside the name", async () => {
    const avatarUrl = "/api/me/avatar?v=1";
    mockApi({ "GET /api/me": { status: 200, body: { user: { ...student, avatarUrl } } } });
    renderApp("/");
    await userEvent.click(await screen.findByRole("button", { name: "Menu" }));

    const picture = menuPanel().getByRole("presentation");
    expect(picture).toHaveAttribute("src", avatarUrl);
    expect(menuPanel().queryByText("AP")).not.toBeInTheDocument();
  });

  it("has no menu for visitors who are not logged in", async () => {
    mockApi({ "GET /api/me": anonymousMe });
    renderApp("/login");
    await screen.findByRole("heading", { level: 1, name: "Log in" });
    expect(screen.queryByRole("button", { name: "Menu" })).not.toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Color theme" })).toBeInTheDocument();
  });
});
