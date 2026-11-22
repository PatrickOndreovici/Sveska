import { useEffect, useState, type ReactNode } from "react";
import { Link } from "react-router";
import { Button, buttonClasses } from "../../components/ui/Button";
import { TextLink } from "../../components/ui/TextLink";
import { errorMessage } from "../../lib/api/client";
import { getDashboard, localTimeZone } from "../../lib/api/dashboard";
import type { Dashboard, RecentDocument, RecentWord, ReviewCounts } from "../../lib/api/types";
import { formatDate } from "../lessons/format";
import { ActivityCalendar } from "./ActivityCalendar";
import { dayTotal, previousDay } from "./activity";

/** A student's overview: words to review, totals, activity by day, and where to pick up. */
export function StudentDashboard() {
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [version, setVersion] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    getDashboard(localTimeZone(), controller.signal).then(
      (next) => {
        setDashboard(next);
        setLoadError(null);
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setLoadError(errorMessage(error, "Could not load your dashboard. Try again."));
      },
    );
    return () => {
      controller.abort();
    };
  }, [version]);

  if (loadError) {
    return (
      <div role="alert" className="mt-10 flex flex-wrap items-center gap-4">
        <p className="text-danger">{loadError}</p>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            setVersion((v) => v + 1);
          }}
        >
          Try again
        </Button>
      </div>
    );
  }
  if (!dashboard) {
    return (
      <p role="status" className="mt-10 text-ink-muted">
        Loading your dashboard…
      </p>
    );
  }

  const { totals, today } = dashboard;
  const activeToday = dashboard.activity.some((d) => d.date === today && dayTotal(d) > 0);

  return (
    <div className="mt-10 grid gap-8">
      {totals.words > 0 && <ReviewPrompt review={dashboard.review} />}

      <section aria-labelledby="totals-heading">
        <h2 id="totals-heading" className="sr-only">
          Your totals
        </h2>
        <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-md border border-rule bg-rule lg:grid-cols-4">
          <StatTile
            label="Words"
            value={totals.words}
            detail={totals.wordsThisWeek > 0 ? `+${String(totals.wordsThisWeek)} this week` : "None added this week"}
          />
          <StatTile
            label="Day streak"
            value={dashboard.streak}
            detail={
              dashboard.streak === 0
                ? "Study today to start one"
                : activeToday
                  ? "You studied today"
                  : "Study today to keep it"
            }
          />
          <StatTile
            label="PDFs"
            value={totals.documents}
            detail={`In ${String(totals.lessons)} ${totals.lessons === 1 ? "lesson" : "lessons"}`}
          />
          <StatTile
            label="Highlights"
            value={totals.highlights}
            detail={`${String(totals.notes)} with ${totals.notes === 1 ? "a note" : "notes"}`}
          />
        </dl>
      </section>

      <div className="grid gap-8 lg:grid-cols-2 lg:items-start">
        <Card title="Activity" headingId="activity-heading">
          <ActivityCalendar activity={dashboard.activity} today={today} />
        </Card>

        <div className="grid gap-8">
          <Card title="Pick up where you left off" headingId="recent-documents-heading">
            <RecentDocuments documents={dashboard.recentDocuments} today={today} />
          </Card>
          <Card title="Newest words" headingId="recent-words-heading">
            <RecentWords words={dashboard.recentWords} total={totals.words} />
          </Card>
        </div>
      </div>
    </div>
  );
}

/** How many words are waiting as flashcards today, with the way in. */
function ReviewPrompt({ review }: { review: ReviewCounts }) {
  const total = review.due + review.new;
  const parts = [review.due > 0 && `${String(review.due)} due`, review.new > 0 && `${String(review.new)} new`];
  return (
    <section
      aria-labelledby="review-heading"
      className="flex flex-wrap items-center justify-between gap-4 rounded-md border border-rule bg-paper-raised p-4 sm:p-5"
    >
      <div>
        <h2 id="review-heading" className="font-serif text-xl text-ink">
          Flashcards
        </h2>
        <p className="mt-1 text-sm text-ink-muted">
          {total === 0
            ? "All your words are reviewed for today."
            : `${pluralWords(total)} to review today: ${parts.filter(Boolean).join(" and ")}.`}
        </p>
      </div>
      {total > 0 && (
        <Link to="/review" className={buttonClasses()}>
          Start review
        </Link>
      )}
    </section>
  );
}

/** A headline number with what it counts and one line of context. */
function StatTile({ label, value, detail }: { label: string; value: number; detail: string }) {
  return (
    <div className="flex flex-col bg-paper-raised p-4 sm:p-5">
      <dt className="text-sm text-ink-muted">{label}</dt>
      {/* The number reads first; the label stays first for screen readers. */}
      <dd className="order-first text-3xl font-semibold text-ink sm:text-4xl">{value.toLocaleString("en-GB")}</dd>
      <dd className="mt-1 text-sm text-ink-subtle">{detail}</dd>
    </div>
  );
}

function Card({ title, headingId, children }: { title: string; headingId: string; children: ReactNode }) {
  return (
    <section aria-labelledby={headingId} className="min-w-0 rounded-md border border-rule bg-paper-raised p-4 sm:p-6">
      <h2 id={headingId} className="font-serif text-xl text-ink">
        {title}
      </h2>
      <div className="mt-3">{children}</div>
    </section>
  );
}

/** "today", "yesterday" or the date, in the student's own time zone. */
function when(iso: string, today: string): string {
  const date = localDate.format(new Date(iso));
  if (date === today) return "today";
  if (date === previousDay(today)) return "yesterday";
  return formatDate(iso);
}

// en-CA writes dates as YYYY-MM-DD, like the API's.
const localDate = new Intl.DateTimeFormat("en-CA", { year: "numeric", month: "2-digit", day: "2-digit" });

function RecentDocuments({ documents, today }: { documents: readonly RecentDocument[]; today: string }) {
  if (documents.length === 0) {
    return (
      <p className="text-sm text-ink-muted">
        Nothing yet. <TextLink to="/lessons">Upload a PDF to a lesson</TextLink> to start reading.
      </p>
    );
  }
  return (
    <ul className="divide-y divide-rule">
      {documents.map((doc) => (
        <li key={doc.id}>
          <Link
            to={`/documents/${encodeURIComponent(doc.id)}?page=${String(doc.pageNumber)}`}
            className="group grid gap-0.5 py-3"
          >
            <span className="font-serif text-lg text-ink group-hover:text-accent">{doc.title}</span>
            <span className="text-sm text-ink-subtle">
              {doc.lesson.title} · page {doc.pageNumber} · {when(doc.workedAt, today)}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function RecentWords({ words, total }: { words: readonly RecentWord[]; total: number }) {
  if (words.length === 0) {
    return (
      <p className="text-sm text-ink-muted">
        No words yet. Save them from a PDF, or <TextLink to="/vocabulary">add them to your vocabulary</TextLink>.
      </p>
    );
  }
  return (
    <>
      <ul className="divide-y divide-rule">
        {words.map((word) => (
          <li key={word.id} className="flex flex-wrap items-baseline justify-between gap-x-4 py-2.5">
            <span lang="sr" className="font-serif text-lg text-ink">
              {word.term}
            </span>
            <span className="text-sm text-ink-muted">{word.translation}</span>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-sm">
        <TextLink to="/vocabulary">See all {pluralWords(total)}</TextLink>
      </p>
    </>
  );
}

function pluralWords(n: number): string {
  return n === 1 ? "1 word" : `${n.toLocaleString("en-GB")} words`;
}
