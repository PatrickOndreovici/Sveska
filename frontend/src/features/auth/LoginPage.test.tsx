import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { anonymousMe, mockApi, student } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

async function fillAndSubmit(email: string, password: string) {
  await userEvent.type(await screen.findByLabelText("Email"), email);
  await userEvent.type(screen.getByLabelText("Password"), password);
  await userEvent.click(screen.getByRole("button", { name: "Log in" }));
}

describe("LoginPage", () => {
  it("logs in and returns to the page that required a login", async () => {
    const fetchSpy = mockApi({
      "GET /api/me": anonymousMe,
      "POST /api/auth/login": { status: 200, body: { user: student } },
    });
    const router = renderApp("/");
    await fillAndSubmit("ana@example.com", "dobar-dan-123");

    expect(await screen.findByRole("heading", { level: 1, name: "Dobar dan, Ana." })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    const loginCall = fetchSpy.mock.calls.find(([url]) => url === "/api/auth/login");
    expect(JSON.parse(loginCall?.[1]?.body as string)).toEqual({ email: "ana@example.com", password: "dobar-dan-123" });
  });

  it("shows the server's message for wrong credentials", async () => {
    mockApi({
      "GET /api/me": anonymousMe,
      "POST /api/auth/login": {
        status: 401,
        body: { error: { code: "invalid_credentials", message: "Email or password is incorrect." } },
      },
    });
    renderApp("/login");
    await fillAndSubmit("ana@example.com", "wrong-password");

    expect(await screen.findByRole("alert")).toHaveTextContent("Email or password is incorrect.");
    expect(screen.getByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();
  });

  it("shows field errors inline and focuses the first invalid field", async () => {
    mockApi({
      "GET /api/me": anonymousMe,
      "POST /api/auth/login": {
        status: 422,
        body: {
          error: {
            code: "validation_failed",
            message: "Some fields are missing or invalid.",
            fields: { email: "Enter your email address.", password: "Enter your password." },
          },
        },
      },
    });
    renderApp("/login");
    await userEvent.click(await screen.findByRole("button", { name: "Log in" }));

    const email = await screen.findByLabelText("Email");
    expect(await screen.findByText("Enter your email address.")).toBeInTheDocument();
    expect(email).toHaveAttribute("aria-invalid", "true");
    expect(email).toHaveAccessibleDescription("Enter your email address.");
    expect(screen.getByLabelText("Password")).toHaveAccessibleDescription("Enter your password.");
    expect(email).toHaveFocus();
  });

  it("explains when the API cannot be reached", async () => {
    mockApi({ "GET /api/me": anonymousMe });
    renderApp("/login");
    await screen.findByLabelText("Email");
    // No handler for the login request: the fake fetch rejects like a network failure.
    await fillAndSubmit("ana@example.com", "dobar-dan-123");
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not reach Sveska.");
  });
});
