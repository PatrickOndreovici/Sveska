import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, student, teacher, type MockHandler } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

const avatarUrl = "/api/me/avatar?v=1792747800123";

function renderSettings(overrides: Record<string, MockHandler> = {}, user: object = student) {
  const fetchSpy = mockApi({
    "GET /api/me": { status: 200, body: { user } },
    "PUT /api/me": (body) => ({
      status: 200,
      body: { user: { ...user, displayName: (body as { displayName: string }).displayName } },
    }),
    "PUT /api/me/avatar": { status: 200, body: { user: { ...user, avatarUrl } } },
    "DELETE /api/me/avatar": { status: 204 },
    ...overrides,
  });
  const router = renderApp("/settings");
  return { fetchSpy, router };
}

function picture(name = "me.jpg", size = 100): File {
  const file = new File(["jpeg"], name, { type: "image/jpeg" });
  Object.defineProperty(file, "size", { value: size });
  return file;
}

/** The site header; the page's own header counts as a banner in jsdom too. */
function header() {
  return within(screen.getAllByRole("banner")[0] as HTMLElement);
}

describe("SettingsPage", () => {
  it("shows the account and the initials when there is no picture", async () => {
    renderSettings();

    expect(await screen.findByRole("heading", { level: 1, name: "Podešavanja" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Your initials" })).toHaveTextContent("AP");
    expect(screen.getByLabelText("Display name")).toHaveValue("Ana Petrović");
    expect(screen.getByText("ana@example.com")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Remove picture" })).not.toBeInTheDocument();
  });

  it("is open to teachers too", async () => {
    renderSettings({}, teacher);
    expect(await screen.findByRole("heading", { level: 1, name: "Podešavanja" })).toBeInTheDocument();
    expect(screen.getByText("Teacher", { selector: "dd" })).toBeInTheDocument();
  });

  it("is reached from the header", async () => {
    mockApi({ "GET /api/me": { status: 200, body: { user: student } } });
    const router = renderApp("/");
    await userEvent.click((await screen.findAllByRole("link", { name: "Settings" }))[0] as HTMLElement);
    expect(router.state.location.pathname).toBe("/settings");
  });

  it("saves the name and shows it in the header", async () => {
    const { fetchSpy } = renderSettings();
    const field = await screen.findByLabelText("Display name");
    await userEvent.clear(field);
    await userEvent.type(field, "  Ana Marković ");
    await userEvent.click(screen.getByRole("button", { name: "Save name" }));

    expect(await screen.findByText("Saved.")).toBeInTheDocument();
    const call = fetchSpy.mock.calls.find(([url, init]) => url === "/api/me" && init?.method === "PUT");
    expect(JSON.parse(call?.[1]?.body as string)).toEqual({ displayName: "Ana Marković" });
    expect(header().getAllByText("Ana Marković").length).toBeGreaterThan(0);
  });

  it("asks for a name before saving an empty one", async () => {
    const { fetchSpy } = renderSettings();
    await userEvent.clear(await screen.findByLabelText("Display name"));
    await userEvent.click(screen.getByRole("button", { name: "Save name" }));

    expect(screen.getByLabelText("Display name")).toHaveAccessibleDescription("Enter your name.");
    expect(fetchSpy.mock.calls.some(([url, init]) => url === "/api/me" && init?.method === "PUT")).toBe(false);
  });

  it("shows the API's reason for refusing a name", async () => {
    renderSettings({
      "PUT /api/me": {
        status: 422,
        body: {
          error: {
            code: "validation_failed",
            message: "Some fields are missing or invalid.",
            fields: { displayName: "Remove line breaks and control characters." },
          },
        },
      },
    });
    await userEvent.click(await screen.findByRole("button", { name: "Save name" }));

    expect(await screen.findByText("Remove line breaks and control characters.")).toBeInTheDocument();
    expect(screen.getByLabelText("Display name")).toHaveAttribute("aria-invalid", "true");
  });

  it("uploads a chosen picture and shows it here and in the header", async () => {
    const { fetchSpy } = renderSettings();
    await userEvent.upload(await screen.findByLabelText("Choose a picture"), picture());

    expect(await screen.findByText("Your picture is saved.")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Your profile picture" })).toHaveAttribute("src", avatarUrl);
    // The header's picture is decorative: the name is beside it.
    expect(document.querySelector(`header img[src="${avatarUrl}"]`)).toHaveAttribute("alt", "");
    const call = fetchSpy.mock.calls.find(([url]) => url === "/api/me/avatar");
    expect(call?.[1]?.method).toBe("PUT");
    expect((call?.[1]?.body as FormData).get("file")).toBeInstanceOf(File);
    expect(screen.getByLabelText("Choose a new picture")).toBeInTheDocument();
  });

  it("refuses a picture over 10 MB without uploading it", async () => {
    const { fetchSpy } = renderSettings();
    await userEvent.upload(await screen.findByLabelText("Choose a picture"), picture("huge.jpg", 11 * 1024 * 1024));

    expect(await screen.findByRole("alert")).toHaveTextContent("Choose a picture of at most 10 MB.");
    expect(fetchSpy.mock.calls.some(([url]) => url === "/api/me/avatar")).toBe(false);
  });

  it("shows the API's reason for refusing a picture", async () => {
    renderSettings({
      "PUT /api/me/avatar": {
        status: 422,
        body: {
          error: {
            code: "validation_failed",
            message: "Some fields are missing or invalid.",
            fields: { file: "Choose a JPEG, PNG, GIF or WebP picture." },
          },
        },
      },
    });
    await userEvent.upload(await screen.findByLabelText("Choose a picture"), picture("me.heic"));

    expect(await screen.findByRole("alert")).toHaveTextContent("Choose a JPEG, PNG, GIF or WebP picture.");
    expect(screen.getByRole("img", { name: "Your initials" })).toBeInTheDocument();
  });

  it("removes the picture and goes back to the initials", async () => {
    const { fetchSpy } = renderSettings({}, { ...student, avatarUrl });
    expect(await screen.findByRole("img", { name: "Your profile picture" })).toHaveAttribute("src", avatarUrl);

    await userEvent.click(screen.getByRole("button", { name: "Remove picture" }));

    expect(await screen.findByText("Your picture is removed.")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Your initials" })).toBeInTheDocument();
    expect(document.querySelector("header img")).toBeNull();
    expect(screen.queryByRole("button", { name: "Remove picture" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("Choose a picture")).toHaveFocus();
    expect(fetchSpy).toHaveBeenCalledWith("/api/me/avatar", expect.objectContaining({ method: "DELETE" }));
  });
});
