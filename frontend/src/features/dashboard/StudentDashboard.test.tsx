import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Dashboard } from "../../lib/api/types";
import { dashboard, mockApi, student, type MockHandler } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

const busy = dashboard(
  [
    { date: "2026-10-21", words: 2, highlights: 0, pages: 0 },
    { date: "2026-10-22", words: 0, highlights: 1, pages: 0 },
    { date: "2026-10-23", words: 4, highlights: 3, pages: 5, reviews: 12 },
  ],
  {
    totals: { words: 1284, wordsThisWeek: 6, lessons: 3, documents: 7, highlights: 40, notes: 1 },
    review: { due: 7, new: 5 },
    streak: 3,
    recentWords: [
      { id: "w2", term: "voda", translation: "water", createdAt: "2026-10-23T09:00:00Z" },
      { id: "w1", term: "kuća", translation: "house", createdAt: "2026-10-21T09:00:00Z" },
    ],
    recentDocuments: [
      {
        id: "d1",
        title: "Čitanka",
        lesson: { id: "l1", title: "Lekcija 1" },
        pageNumber: 4,
        workedAt: "2026-10-23T11:00:00Z",
      },
      {
        id: "d2",
        title: "Radna sveska",
        lesson: { id: "l2", title: "Lekcija 2" },
        pageNumber: 1,
        workedAt: "2026-10-01T11:00:00Z",
      },
    ],
  },
);

function renderDashboard(body: Dashboard | MockHandler = busy) {
  const handler: MockHandler = "today" in body ? { status: 200, body } : body;
  const fetchSpy = mockApi({
    "GET /api/me": { status: 200, body: { user: student } },
    "GET /api/dashboard": handler,
  });
  const router = renderApp("/");
  return { fetchSpy, router };
}

function tile(label: string) {
  const term = screen.getByText(label, { selector: "dt" });
  const group = term.parentElement;
  if (!group) throw new Error(`no tile for ${label}`);
  return group;
}

describe("StudentDashboard", () => {
  it("asks for the dashboard in the browser's time zone", async () => {
    const { fetchSpy } = renderDashboard();
    await screen.findByText("Day streak");
    const call = fetchSpy.mock.calls.find(([url]) => typeof url === "string" && url.startsWith("/api/dashboard"));
    const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    expect(call?.[0]).toBe(`/api/dashboard?timeZone=${encodeURIComponent(timeZone)}`);
  });

  it("shows the totals as tiles", async () => {
    renderDashboard();
    await screen.findByText("Day streak");

    expect(tile("Words")).toHaveTextContent("1,284");
    expect(tile("Words")).toHaveTextContent("+6 this week");
    expect(tile("Day streak")).toHaveTextContent("3");
    expect(tile("Day streak")).toHaveTextContent("You studied today");
    expect(tile("PDFs")).toHaveTextContent("7");
    expect(tile("PDFs")).toHaveTextContent("In 3 lessons");
    expect(tile("Highlights")).toHaveTextContent("40");
    expect(tile("Highlights")).toHaveTextContent("1 with a note");
  });

  it.each([
    [0, [], "Study today to start one"],
    [2, [{ date: "2026-10-22", words: 1, highlights: 0, pages: 0 }], "Study today to keep it"],
  ])("with a streak of %i says %s", async (streak, days, text) => {
    renderDashboard(dashboard(days, { streak }));
    expect(await screen.findByText(text)).toBeInTheDocument();
  });

  it("invites the student to review the words due today", async () => {
    renderDashboard();
    const section = await screen.findByRole("region", { name: "Flashcards" });

    expect(section).toHaveTextContent("12 words to review today: 7 due and 5 new.");
    expect(within(section).getByRole("link", { name: "Start review" })).toHaveAttribute("href", "/review");
  });

  it.each([
    [{ due: 3, new: 0 }, "3 words to review today: 3 due."],
    [{ due: 0, new: 1 }, "1 word to review today: 1 new."],
  ])("counts only what there is to review (%o)", async (review, text) => {
    renderDashboard(dashboard([], { totals: { ...busy.totals }, review }));
    expect(await screen.findByRole("region", { name: "Flashcards" })).toHaveTextContent(text);
  });

  it("says when every word is reviewed for today", async () => {
    renderDashboard(dashboard([], { totals: { ...busy.totals }, review: { due: 0, new: 0 } }));
    const section = await screen.findByRole("region", { name: "Flashcards" });

    expect(section).toHaveTextContent("All your words are reviewed for today.");
    expect(within(section).queryByRole("link")).not.toBeInTheDocument();
  });

  it("links each recent PDF to the page last worked on", async () => {
    renderDashboard();
    const section = await screen.findByRole("region", { name: "Pick up where you left off" });

    const first = within(section).getByRole("link", { name: /Čitanka/ });
    expect(first).toHaveAttribute("href", "/documents/d1?page=4");
    expect(first).toHaveTextContent("Lekcija 1 · page 4 · today");
    const second = within(section).getByRole("link", { name: /Radna sveska/ });
    expect(second).toHaveAttribute("href", "/documents/d2?page=1");
    expect(second).toHaveTextContent("Lekcija 2 · page 1 · 1 Oct 2026");
  });

  it("lists the newest words and links to the rest", async () => {
    renderDashboard();
    const section = await screen.findByRole("region", { name: "Newest words" });

    const items = within(section).getAllByRole("listitem");
    expect(items.map((li) => li.textContent)).toEqual(["vodawater", "kućahouse"]);
    expect(within(section).getByRole("link", { name: "See all 1,284 words" })).toHaveAttribute("href", "/vocabulary");
  });

  it("points a new student to where to start", async () => {
    renderDashboard(dashboard());
    await screen.findByText("Day streak");

    expect(screen.getByText("Nothing done in the last 12 weeks yet.")).toBeInTheDocument();
    // No words, so no flashcards yet.
    expect(screen.queryByRole("region", { name: "Flashcards" })).not.toBeInTheDocument();
    const docs = screen.getByRole("region", { name: "Pick up where you left off" });
    expect(within(docs).getByRole("link", { name: "Upload a PDF to a lesson" })).toHaveAttribute("href", "/lessons");
    const words = screen.getByRole("region", { name: "Newest words" });
    expect(within(words).getByRole("link", { name: "add them to your vocabulary" })).toHaveAttribute(
      "href",
      "/vocabulary",
    );
  });

  it("shows a message and retries when the dashboard does not load", async () => {
    let calls = 0;
    renderDashboard(() => {
      calls++;
      return calls === 1
        ? { status: 500, body: { error: { code: "internal_error", message: "Something went wrong on our side." } } }
        : { status: 200, body: busy };
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong on our side.");
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Day streak")).toBeInTheDocument();
  });
});

describe("ActivityCalendar", () => {
  it("counts the active days in the summary", async () => {
    renderDashboard();
    expect(await screen.findByText("Active on 3 days in the last 12 weeks.")).toBeInTheDocument();
  });

  it("draws one cell per day up to today, darker for more done", async () => {
    renderDashboard();
    const calendar = await screen.findByTestId("activity-calendar");

    // 23 October 2026 is a Friday: 11 full weeks and 5 days of this one.
    expect(calendar.querySelectorAll("[data-date]")).toHaveLength(11 * 7 + 5);
    const cell = (date: string) => calendar.querySelector(`[data-date="${date}"] > div`);
    expect(cell("2026-10-20")).toHaveClass("bg-paper-sunken");
    expect(cell("2026-10-21")).toHaveClass("bg-activity-1");
    expect(cell("2026-10-23")).toHaveClass("bg-activity-4");
    expect(calendar).toHaveAttribute("aria-hidden", "true");
  });

  it("shows a day's numbers when pointed at", async () => {
    renderDashboard();
    const calendar = await screen.findByTestId("activity-calendar");
    const day = calendar.querySelector('[data-date="2026-10-23"]');
    if (!day) throw new Error("no cell for today");

    await userEvent.hover(day);
    const tooltip = screen.getByRole("tooltip");
    expect(tooltip).toHaveTextContent("4 words, 3 highlights, 5 pages written on, 12 cards reviewed");
    expect(tooltip).toHaveTextContent("Fri 23 Oct");

    await userEvent.unhover(calendar);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("has the same numbers in a table, newest day first", async () => {
    renderDashboard();
    await userEvent.click(await screen.findByText("Show the days as a table"));

    const rows = within(screen.getByRole("table")).getAllByRole("row").slice(1);
    expect(rows.map((r) => within(r).getAllByRole("cell").map((c) => c.textContent))).toEqual([
      ["4", "3", "5", "12"],
      ["0", "1", "0", "0"],
      ["2", "0", "0", "0"],
    ]);
    expect(within(rows[0] as HTMLElement).getByRole("rowheader")).toHaveTextContent("Fri 23 Oct");
  });
});
