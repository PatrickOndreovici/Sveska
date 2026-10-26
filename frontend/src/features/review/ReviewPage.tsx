import { useCallback, useEffect, useRef, useState, type RefObject } from "react";
import { Link } from "react-router";
import { Button, buttonClasses } from "../../components/ui/Button";
import { Eyebrow } from "../../components/ui/Eyebrow";
import { TextLink } from "../../components/ui/TextLink";
import { ApiError, errorMessage } from "../../lib/api/client";
import { localTimeZone } from "../../lib/api/dashboard";
import { createReview, getReviewQueue } from "../../lib/api/reviews";
import type { ReviewCard, ReviewQueue, ReviewRating } from "../../lib/api/types";
import { formatDay, previousDay } from "../dashboard/activity";
import { formsOf } from "../vocabulary/entryFields";
import { GrammarTag } from "../vocabulary/VocabularyRow";
import { RATINGS, formatInterval } from "./intervals";

// The API decides what to show and when each word comes back: the page shows
// the cards of one GET /api/review-queue in order, sends each answer, and asks
// for the queue again once they are answered. A word answered "again" is due
// again today, so it comes back in that next queue.

type Session = {
  queue: ReviewQueue;
  /** The cards of this queue not answered yet; the first is on screen. */
  cards: ReviewCard[];
  /** Words left today: due and new, less those answered for a later day. */
  left: number;
};

type FocusTarget = "show" | "answer" | "done";

export function ReviewPage() {
  const [session, setSession] = useState<Session | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  // Bumped to fetch the queue again.
  const [version, setVersion] = useState(0);
  const [revealed, setRevealed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [answerError, setAnswerError] = useState<string | null>(null);
  // Something to say above the next card, such as why a word was skipped.
  const [notice, setNotice] = useState<string | null>(null);
  const [answered, setAnswered] = useState(0);
  // Set while an answer is being sent, so a second key press cannot send another.
  const sending = useRef(false);

  // Focus moves in an effect, once the element it goes to is rendered and enabled.
  const [focusRequest, setFocusRequest] = useState<{ target: FocusTarget; n: number } | null>(null);
  const requestFocus = useCallback((target: FocusTarget) => {
    setFocusRequest((r) => ({ target, n: (r?.n ?? 0) + 1 }));
  }, []);
  const showButton = useRef<HTMLButtonElement>(null);
  const answerBlock = useRef<HTMLDivElement>(null);
  const doneHeading = useRef<HTMLHeadingElement>(null);
  // Off for the first load, so opening the page does not move focus.
  const focusAfterLoad = useRef(false);

  useEffect(() => {
    if (!focusRequest) return;
    const refs = { show: showButton, answer: answerBlock, done: doneHeading };
    refs[focusRequest.target].current?.focus();
  }, [focusRequest]);

  useEffect(() => {
    const controller = new AbortController();
    getReviewQueue(localTimeZone(), controller.signal).then(
      (queue) => {
        setSession({ queue, cards: queue.cards, left: queue.due + queue.new });
        setRevealed(false);
        setLoadError(null);
        if (focusAfterLoad.current) requestFocus(queue.cards.length > 0 ? "show" : "done");
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setLoadError(errorMessage(error, "Could not load your words. Try again."));
      },
    );
    return () => {
      controller.abort();
    };
  }, [version, requestFocus]);

  const current = session?.cards[0];

  const reveal = useCallback(() => {
    setRevealed(true);
    requestFocus("answer");
  }, [requestFocus]);

  const reload = useCallback(() => {
    focusAfterLoad.current = true;
    setSession(null);
    setVersion((v) => v + 1);
  }, []);

  const answer = useCallback(
    async (rating: ReviewRating) => {
      if (!session || !current || sending.current) return;
      sending.current = true;
      setSaving(true);
      setAnswerError(null);
      try {
        const review = await createReview(current.entry.id, rating, localTimeZone());
        setAnswered((n) => n + 1);
        setNotice(null);
        const cards = session.cards.slice(1);
        if (cards.length === 0) {
          reload();
        } else {
          const left = review.dueOn > session.queue.today ? session.left - 1 : session.left;
          setSession({ ...session, cards, left });
          setRevealed(false);
          requestFocus("show");
        }
      } catch (error) {
        // Answered in another tab, or deleted since: the queue has moved on.
        if (error instanceof ApiError && (error.status === 409 || error.status === 404)) {
          setNotice(error.status === 404 ? "That word was deleted, so it was skipped." : error.message);
          reload();
        } else {
          setAnswerError(errorMessage(error, "Could not save your answer. Try again."));
        }
      } finally {
        sending.current = false;
        setSaving(false);
      }
    },
    [session, current, reload, requestFocus],
  );

  // Space or Enter shows the answer; 1–4 answer. Keys typed into a field, or
  // pressed on a button or link (which act on their own), are left alone.
  useEffect(() => {
    if (!current) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.altKey || event.ctrlKey || event.metaKey || event.repeat) return;
      const target = event.target instanceof Element ? event.target : null;
      if (target?.closest("input, textarea, select, [contenteditable]")) return;
      if (!revealed) {
        if ((event.key === " " || event.key === "Enter") && !target?.closest("button, a")) {
          event.preventDefault();
          reveal();
        }
        return;
      }
      const choice = RATINGS.find((r) => r.key === event.key);
      if (choice) {
        event.preventDefault();
        void answer(choice.rating);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [current, revealed, reveal, answer]);

  return (
    <div className="mx-auto max-w-2xl">
      <header>
        <Eyebrow>Review</Eyebrow>
        <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">Ponavljanje</h1>
        {current && (
          <p className="mt-3 text-ink-muted">
            {session.left === 1 ? "1 word" : `${String(session.left)} words`} left today
          </p>
        )}
      </header>

      {notice && (
        <p role="status" className="mt-6 text-sm text-ink-muted">
          {notice}
        </p>
      )}

      {loadError ? (
        <div role="alert" className="mt-10 flex flex-wrap items-center gap-4">
          <p className="text-danger">{loadError}</p>
          <Button variant="secondary" size="sm" onClick={reload}>
            Try again
          </Button>
        </div>
      ) : !session ? (
        <p role="status" className="mt-10 text-ink-muted">
          Loading your words…
        </p>
      ) : current ? (
        <Flashcard
          card={current}
          revealed={revealed}
          saving={saving}
          answerError={answerError}
          showButton={showButton}
          answerBlock={answerBlock}
          onReveal={reveal}
          onAnswer={(rating) => {
            void answer(rating);
          }}
        />
      ) : (
        <Done queue={session.queue} answered={answered} heading={doneHeading} />
      )}
    </div>
  );
}

type FlashcardProps = {
  card: ReviewCard;
  revealed: boolean;
  saving: boolean;
  answerError: string | null;
  showButton: RefObject<HTMLButtonElement | null>;
  answerBlock: RefObject<HTMLDivElement | null>;
  onReveal: () => void;
  onAnswer: (rating: ReviewRating) => void;
};

/** One word: the Serbian side, then the answer and four buttons to say how well it was remembered. */
function Flashcard({ card, revealed, saving, answerError, showButton, answerBlock, onReveal, onAnswer }: FlashcardProps) {
  const { entry } = card;
  const forms = formsOf(entry);

  return (
    <section aria-label="Flashcard" className="mt-8">
      <div className="rounded-md border border-rule bg-paper-raised px-6 py-10 text-center sm:px-10">
        {card.new && <p className="text-eyebrow font-medium text-accent uppercase">New word</p>}
        <p lang="sr" className="mt-2 font-serif text-5xl break-words text-ink">
          {entry.term}
        </p>

        {revealed && (
          <div ref={answerBlock} tabIndex={-1} className="mt-8 border-t border-rule pt-8 outline-none">
            <p className="text-2xl text-ink">{entry.translation}</p>
            {(entry.gender || entry.aspect || forms) && (
              <p className="mt-2 flex flex-wrap items-baseline justify-center gap-x-2 text-ink-muted">
                <GrammarTag entry={entry} />
                {forms && <span lang="sr">{forms}</span>}
              </p>
            )}
            {entry.example && (
              <p lang="sr" className="mt-4 font-serif text-lg text-ink-muted italic">
                {entry.example}
              </p>
            )}
            {entry.notes && <p className="mt-3 text-sm text-ink-subtle">{entry.notes}</p>}
            {entry.source && (
              <p className="mt-3 text-sm text-ink-subtle">
                From{" "}
                <TextLink to={`/documents/${entry.source.documentId}?page=${String(entry.source.pageNumber)}`}>
                  {entry.source.documentTitle}, page {entry.source.pageNumber}
                </TextLink>
              </p>
            )}
          </div>
        )}
      </div>

      {revealed ? (
        <div className="mt-6">
          <p id="rating-question" className="text-center text-sm text-ink-muted">
            How well did you know it?
          </p>
          <div role="group" aria-labelledby="rating-question" className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {RATINGS.map(({ rating, label, key }) => {
              const when = formatInterval(card.nextIntervals[rating]);
              return (
                <Button
                  key={rating}
                  variant={rating === "good" ? "primary" : "secondary"}
                  className="h-auto flex-col gap-0.5 py-2.5"
                  aria-label={`${label}, back ${when === "today" ? "today" : `in ${when}`}`}
                  aria-keyshortcuts={key}
                  disabled={saving}
                  onClick={() => {
                    onAnswer(rating);
                  }}
                >
                  <span>{label}</span>
                  <span className="text-xs font-normal opacity-80">{when}</span>
                </Button>
              );
            })}
          </div>
        </div>
      ) : (
        <div className="mt-6 flex justify-center">
          <Button ref={showButton} className="w-full sm:w-auto sm:min-w-48" aria-keyshortcuts="Space" onClick={onReveal}>
            Show answer
          </Button>
        </div>
      )}

      {answerError && (
        <p role="alert" className="mt-4 text-center text-sm text-danger">
          {answerError}
        </p>
      )}
      <p className="mt-6 hidden text-center text-xs text-ink-subtle sm:block">
        Keys: Space shows the answer, then 1 Again, 2 Hard, 3 Good, 4 Easy.
      </p>
    </section>
  );
}

/** Nothing left for today: when to come back, or how to get words to review. */
function Done({
  queue,
  answered,
  heading,
}: {
  queue: ReviewQueue;
  answered: number;
  heading: RefObject<HTMLHeadingElement | null>;
}) {
  if (queue.nextDueOn === null) {
    return (
      <section className="mt-10 max-w-prose">
        <h2 ref={heading} tabIndex={-1} className="font-serif text-2xl text-ink outline-none">
          No words to review yet
        </h2>
        <p className="mt-3 text-ink-muted">
          Words you save from a PDF or <TextLink to="/vocabulary">add to your vocabulary</TextLink> become flashcards
          here.
        </p>
      </section>
    );
  }
  const next = queue.nextDueOn;
  const when = previousDay(next) === queue.today ? `tomorrow, ${formatDay(next)}` : formatDay(next);
  return (
    <section className="mt-10 max-w-prose">
      <h2 ref={heading} tabIndex={-1} className="font-serif text-2xl text-ink outline-none">
        All done for today
      </h2>
      <p className="mt-3 text-ink-muted">
        {answered > 0 && `You answered ${answered === 1 ? "1 card" : `${String(answered)} cards`}. `}
        Your next words are due {when}.
      </p>
      <div className="mt-6 flex flex-wrap gap-3">
        <Link to="/" className={buttonClasses("secondary")}>
          Back to your workspace
        </Link>
        <Link to="/vocabulary" className={buttonClasses("quiet")}>
          Add more words
        </Link>
      </div>
    </section>
  );
}
