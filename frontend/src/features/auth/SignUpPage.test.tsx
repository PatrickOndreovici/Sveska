import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { anonymousMe, mockApi, teacher } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

async function fillForm({ role }: { role?: RegExp } = {}) {
  await userEvent.type(await screen.findByLabelText("Your name"), "Marko Jovanović");
  await userEvent.type(screen.getByLabelText("Email"), "marko@example.com");
  await userEvent.type(screen.getByLabelText("Password"), "dobar-dan-123");
  if (role) await userEvent.click(screen.getByRole("radio", { name: role }));
}

describe("SignUpPage", () => {
  it("creates a teacher account and opens the teacher workspace", async () => {
    const fetchSpy = mockApi({
      "GET /api/me": anonymousMe,
      "POST /api/auth/signup": { status: 201, body: { user: teacher } },
    });
    renderApp("/signup");
    await fillForm({ role: /I teach Serbian/ });
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Dobar dan, Marko." })).toBeInTheDocument();
    expect(screen.getByText("Teacher")).toBeInTheDocument();
    const call = fetchSpy.mock.calls.find(([url]) => url === "/api/auth/signup");
    expect(JSON.parse(call?.[1]?.body as string)).toEqual({
      displayName: "Marko Jovanović",
      email: "marko@example.com",
      password: "dobar-dan-123",
      role: "teacher",
    });
  });

  it("flags every empty field at once without sending anything", async () => {
    const fetchSpy = mockApi({ "GET /api/me": anonymousMe });
    renderApp("/signup");
    await userEvent.click(await screen.findByRole("button", { name: "Create account" }));

    for (const message of ["Enter your name.", "Enter your email address.", "Enter a password.", "Choose student or teacher."]) {
      expect(await screen.findByText(message)).toBeInTheDocument();
    }
    expect(screen.getByLabelText("Your name")).toHaveFocus();
    expect(fetchSpy.mock.calls.some(([url]) => url === "/api/auth/signup")).toBe(false);
  });

  it("asks for a role before sending anything", async () => {
    const fetchSpy = mockApi({ "GET /api/me": anonymousMe });
    renderApp("/signup");
    await fillForm();
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));

    expect(await screen.findByText("Choose student or teacher.")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "How will you use Sveska?" })).toHaveAccessibleDescription("Choose student or teacher.");
    expect(fetchSpy.mock.calls.some(([url]) => url === "/api/auth/signup")).toBe(false);
  });

  it("points out an email that already has an account", async () => {
    mockApi({
      "GET /api/me": anonymousMe,
      "POST /api/auth/signup": {
        status: 409,
        body: { error: { code: "email_taken", message: "An account with this email already exists." } },
      },
    });
    renderApp("/signup");
    await fillForm({ role: /I'm learning Serbian/ });
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));

    const email = screen.getByLabelText("Email");
    expect(await screen.findByText("An account with this email already exists.")).toBeInTheDocument();
    expect(email).toHaveAttribute("aria-invalid", "true");
    expect(email).toHaveFocus();
  });

  it("shows the password rule as a hint until there is an error", async () => {
    mockApi({
      "GET /api/me": anonymousMe,
      "POST /api/auth/signup": {
        status: 422,
        body: { error: { code: "validation_failed", message: "Invalid.", fields: { password: "Use at least 10 characters." } } },
      },
    });
    renderApp("/signup");
    expect(await screen.findByLabelText("Password")).toHaveAccessibleDescription("At least 10 characters.");
    await fillForm({ role: /I'm learning Serbian/ });
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByText("Use at least 10 characters.")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Use at least 10 characters.");
  });
});
