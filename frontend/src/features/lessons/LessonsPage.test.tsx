import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Lesson } from "../../lib/api/types";
import { mockApi, student, type MockHandler } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

export function lesson(id: string, title: string, extra: Partial<Lesson> = {}): Lesson {
  return {
    id,
    title,
    description: "",
    documentCount: 0,
    createdAt: "2026-10-23T10:00:00Z",
    updatedAt: "2026-10-23T10:00:00Z",
    ...extra,
  };
}

const me = { "GET /api/me": { status: 200, body: { user: student } } };

function renderLessons(routes: Record<string, MockHandler>) {
  const fetchSpy = mockApi({ ...me, ...routes });
  const router = renderApp("/lessons");
  return { fetchSpy, router };
}

describe("LessonsPage", () => {
  it("lists the lessons with their PDF counts", async () => {
    renderLessons({
      "GET /api/lessons": {
        status: 200,
        body: {
          lessons: [
            lesson("l2", "Lekcija 2", { documentCount: 1, description: "Glagoli" }),
            lesson("l1", "Lekcija 1", { documentCount: 3 }),
          ],
        },
      },
    });

    const list = await screen.findByRole("list");
    const links = within(list).getAllByRole("link");
    expect(links).toHaveLength(2);
    expect(links[0]).toHaveAttribute("href", "/lessons/l2");
    expect(links[0]).toHaveTextContent("Lekcija 2");
    expect(links[0]).toHaveTextContent("Glagoli");
    expect(links[0]).toHaveTextContent("1 PDF · 23 Oct 2026");
    expect(links[1]).toHaveTextContent("3 PDFs");
  });

  it("explains what to do when there are no lessons", async () => {
    renderLessons({ "GET /api/lessons": { status: 200, body: { lessons: [] } } });
    expect(await screen.findByText(/No lessons yet/)).toBeInTheDocument();
  });

  it("creates a lesson and opens it", async () => {
    const { fetchSpy, router } = renderLessons({
      "GET /api/lessons": { status: 200, body: { lessons: [] } },
      "POST /api/lessons": { status: 201, body: { lesson: lesson("new-id", "Padeži") } },
      "GET /api/lessons/new-id": { status: 200, body: { lesson: lesson("new-id", "Padeži"), documents: [] } },
    });
    const form = within(await screen.findByRole("region", { name: "New lesson" }));

    await userEvent.type(form.getByLabelText("Title"), "  Padeži ");
    await userEvent.type(form.getByLabelText("Description"), "Sedam padeža");
    await userEvent.click(form.getByRole("button", { name: "Create lesson" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Padeži" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/lessons/new-id");
    const call = fetchSpy.mock.calls.find(([url, init]) => url === "/api/lessons" && init?.method === "POST");
    expect(JSON.parse(call?.[1]?.body as string)).toEqual({ title: "Padeži", description: "Sedam padeža" });
  });

  it("asks for a title before calling the API", async () => {
    const { fetchSpy } = renderLessons({ "GET /api/lessons": { status: 200, body: { lessons: [] } } });
    const form = within(await screen.findByRole("region", { name: "New lesson" }));

    await userEvent.click(form.getByRole("button", { name: "Create lesson" }));

    expect(form.getByLabelText("Title")).toHaveAccessibleDescription("Enter a title.");
    expect(fetchSpy.mock.calls.some(([, init]) => init?.method === "POST")).toBe(false);
  });

  it("offers a retry when the list cannot be loaded", async () => {
    let failures = 1;
    renderLessons({
      "GET /api/lessons": () =>
        failures-- > 0
          ? { status: 500, body: { error: { code: "internal_error", message: "Something went wrong on our side. Try again." } } }
          : { status: 200, body: { lessons: [lesson("l1", "Lekcija 1")] } },
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong on our side.");
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("link", { name: /Lekcija 1/ })).toBeInTheDocument();
  });

  it("is in the student navigation", async () => {
    renderLessons({ "GET /api/lessons": { status: 200, body: { lessons: [] } } });
    const nav = await screen.findByRole("navigation", { name: "Main" });
    expect(within(nav).getByRole("link", { name: "Lessons" })).toHaveAttribute("aria-current", "page");
  });
});
