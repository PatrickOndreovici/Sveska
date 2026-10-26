import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { ReviewCard, ReviewQueue, VocabularyEntry } from "../../lib/api/types";
import { mockApi, student, type MockHandler, type MockResponse } from "../../test/mockApi";
import { renderApp } from "../../test/renderApp";

const blank = {
  notes: "",
  example: "",
  partOfSpeech: "",
  gender: "",
  plural: "",
  aspect: "",
  aspectPair: "",
  feminineForm: "",
  neuterForm: "",
  source: null,
  createdAt: "2026-10-23T10:00:00Z",
  updatedAt: "2026-10-23T10:00:00Z",
} as const;

function card(id: string, term: string, translation: string, extra: Partial<VocabularyEntry> = {}, isNew = false): ReviewCard {
  return {
    entry: { id, term, translation, ...blank, ...extra },
    new: isNew,
    nextIntervals: isNew ? { again: 0, hard: 1, good: 2, easy: 4 } : { again: 0, hard: 12, good: 25, easy: 33 },
  };
}

const kuca = card("entry-kuca", "kuća", "house", {
  partOfSpeech: "noun",
  gender: "feminine",
  plural: "kuće",
  example: "Moja kuća je velika.",
  source: { annotationId: "a1", documentId: "d1", documentTitle: "Čitanka", pageNumber: 3 },
});
const voda = card("entry-voda", "voda", "water", {}, true);

function queue(cards: ReviewCard[], extra: Partial<ReviewQueue> = {}): ReviewQueue {
  return {
    today: "2026-10-23",
    cards,
    due: cards.filter((c) => !c.new).length,
    new: cards.filter((c) => c.new).length,
    nextDueOn: "2026-10-24",
    ...extra,
  };
}

/**
 * Answers GET /api/review-queue with the given queues in turn (the last one
 * again once they run out) and records every answer. An answer comes back
 * due after the card's interval for its rating, unless `answer` says otherwise.
 */
function fakeReviewApi(queues: ReviewQueue[], answer?: (body: { rating: string }) => MockResponse | undefined) {
  let fetches = 0;
  const answers: { entryId: string; body: unknown }[] = [];
  const routes: Record<string, MockHandler> = {
    "GET /api/me": { status: 200, body: { user: student } },
    "GET /api/review-queue": () => {
      const q = queues[Math.min(fetches, queues.length - 1)];
      fetches++;
      return { status: 200, body: q };
    },
  };
  for (const c of queues.flatMap((q) => q.cards)) {
    routes[`POST /api/vocabulary/${c.entry.id}/reviews`] = (body) => {
      const { rating } = body as { rating: keyof ReviewCard["nextIntervals"] };
      answers.push({ entryId: c.entry.id, body });
      const override = answer?.({ rating });
      if (override) return override;
      const days = c.nextIntervals[rating];
      const dueOn = new Date(Date.parse("2026-10-23T00:00:00Z") + days * 86_400_000).toISOString().slice(0, 10);
      return { status: 201, body: { review: { entryId: c.entry.id, rating, dueOn, intervalDays: days } } };
    };
  }
  const fetchSpy = mockApi(routes);
  return { fetchSpy, answers, fetches: () => fetches };
}

const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;

describe("ReviewPage", () => {
  it("asks for the queue in the browser's time zone", async () => {
    const { fetchSpy } = fakeReviewApi([queue([kuca])]);
    renderApp("/review");
    await screen.findByText("kuća");
    const call = fetchSpy.mock.calls.find(([url]) => typeof url === "string" && url.startsWith("/api/review-queue"));
    expect(call?.[0]).toBe(`/api/review-queue?timeZone=${encodeURIComponent(timeZone)}`);
  });

  it("shows a word, then its answer, then the next word", async () => {
    const { answers } = fakeReviewApi([queue([kuca, voda])]);
    renderApp("/review");

    expect(await screen.findByText("kuća")).toBeInTheDocument();
    expect(screen.getByText("2 words left today")).toBeInTheDocument();
    expect(screen.queryByText("house")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Show answer" }));
    expect(screen.getByText("house")).toBeInTheDocument();
    expect(screen.getByTitle("Feminine · ženski rod")).toHaveTextContent("f.");
    expect(screen.getByText("pl. kuće")).toBeInTheDocument();
    expect(screen.getByText("Moja kuća je velika.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Čitanka, page 3" })).toHaveAttribute("href", "/documents/d1?page=3");
    // Focus moves to the answer so a screen reader reads it.
    expect(document.activeElement).toHaveTextContent("house");

    const ratings = within(screen.getByRole("group", { name: "How well did you know it?" }));
    expect(ratings.getAllByRole("button").map((b) => b.getAttribute("aria-label"))).toEqual([
      "Again, back today",
      "Hard, back in 12 days",
      "Good, back in 25 days",
      "Easy, back in 1 month",
    ]);

    await userEvent.click(ratings.getByRole("button", { name: "Good, back in 25 days" }));
    expect(answers).toEqual([{ entryId: "entry-kuca", body: { rating: "good", timeZone } }]);

    expect(await screen.findByText("voda")).toBeInTheDocument();
    expect(screen.getByText("New word")).toBeInTheDocument();
    expect(screen.getByText("1 word left today")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show answer" })).toHaveFocus();
  });

  it("brings a forgotten word back later the same day", async () => {
    const { answers } = fakeReviewApi([
      queue([kuca]),
      // Answered "again", kuća is due again today.
      queue([kuca]),
      queue([], { nextDueOn: "2026-10-24" }),
    ]);
    renderApp("/review");

    await userEvent.click(await screen.findByRole("button", { name: "Show answer" }));
    await userEvent.click(screen.getByRole("button", { name: "Again, back today" }));
    // The batch is answered, so the queue is asked for again and kuća is in it.
    expect(await screen.findByRole("button", { name: "Show answer" })).toHaveFocus();
    expect(screen.getByText("kuća")).toBeInTheDocument();
    expect(screen.getByText("1 word left today")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Show answer" }));
    await userEvent.click(screen.getByRole("button", { name: "Good, back in 25 days" }));

    const done = await screen.findByRole("heading", { name: "All done for today" });
    expect(done).toHaveFocus();
    expect(screen.getByText("You answered 2 cards. Your next words are due tomorrow, Sat 24 Oct.")).toBeInTheDocument();
    expect(answers.map((a) => (a.body as { rating: string }).rating)).toEqual(["again", "good"]);
  });

  it("works from the keyboard", async () => {
    const { answers } = fakeReviewApi([queue([kuca, voda])]);
    renderApp("/review");
    await screen.findByText("kuća");

    // A number does nothing until the answer shows.
    await userEvent.keyboard("3");
    expect(answers).toHaveLength(0);

    await userEvent.keyboard(" ");
    expect(screen.getByText("house")).toBeInTheDocument();
    await userEvent.keyboard("2");
    expect(answers).toEqual([{ entryId: "entry-kuca", body: { rating: "hard", timeZone } }]);
    expect(await screen.findByText("voda")).toBeInTheDocument();
  });

  it("says when to come back when nothing is due", async () => {
    fakeReviewApi([queue([], { nextDueOn: "2026-10-30" })]);
    renderApp("/review");

    expect(await screen.findByRole("heading", { name: "All done for today" })).not.toHaveFocus();
    expect(screen.getByText("Your next words are due Fri 30 Oct.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to your workspace" })).toHaveAttribute("href", "/");
  });

  it("points to the vocabulary when there are no words yet", async () => {
    fakeReviewApi([queue([], { nextDueOn: null })]);
    renderApp("/review");

    expect(await screen.findByRole("heading", { name: "No words to review yet" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "add to your vocabulary" })).toHaveAttribute("href", "/vocabulary");
  });

  it("keeps the card when an answer cannot be saved", async () => {
    let fail = true;
    const { answers } = fakeReviewApi([queue([kuca, voda])], () =>
      fail
        ? { status: 500, body: { error: { code: "internal_error", message: "Something went wrong on our side." } } }
        : undefined,
    );
    renderApp("/review");
    await userEvent.click(await screen.findByRole("button", { name: "Show answer" }));
    await userEvent.click(screen.getByRole("button", { name: "Good, back in 25 days" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong on our side.");
    expect(screen.getByText("house")).toBeInTheDocument();

    fail = false;
    await userEvent.click(screen.getByRole("button", { name: "Good, back in 25 days" }));
    expect(await screen.findByText("voda")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(answers).toHaveLength(2);
  });

  it("moves on when the word was answered somewhere else", async () => {
    const message = "This word was just reviewed somewhere else. Go on with the next one.";
    const api = fakeReviewApi([queue([kuca, voda]), queue([voda])], () => ({
      status: 409,
      body: { error: { code: "review_conflict", message } },
    }));
    renderApp("/review");
    await userEvent.click(await screen.findByRole("button", { name: "Show answer" }));
    await userEvent.click(screen.getByRole("button", { name: "Good, back in 25 days" }));

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(await screen.findByText("voda")).toBeInTheDocument();
    expect(screen.queryByText("kuća")).not.toBeInTheDocument();
    expect(api.fetches()).toBe(2);
  });

  it("shows a message and retries when the queue does not load", async () => {
    let calls = 0;
    mockApi({
      "GET /api/me": { status: 200, body: { user: student } },
      "GET /api/review-queue": () => {
        calls++;
        return calls === 1
          ? { status: 500, body: { error: { code: "internal_error", message: "Something went wrong on our side." } } }
          : { status: 200, body: queue([kuca]) };
      },
    });
    renderApp("/review");

    expect(await screen.findByRole("alert")).toHaveTextContent("Something went wrong on our side.");
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("kuća")).toBeInTheDocument();
  });

  it("is in the main navigation", async () => {
    fakeReviewApi([queue([kuca])]);
    renderApp("/review");
    await screen.findByText("kuća");

    const nav = screen.getAllByRole("navigation", { name: "Main" })[0];
    if (!nav) throw new Error("no main navigation");
    const link = within(nav).getByRole("link", { name: "Review" });
    expect(link).toHaveAttribute("href", "/review");
    await waitFor(() => {
      expect(link).toHaveAttribute("aria-current", "page");
    });
  });
});
