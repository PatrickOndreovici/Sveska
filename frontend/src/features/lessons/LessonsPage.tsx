import { useEffect, useState, type SubmitEvent } from "react";
import { Link, useNavigate } from "react-router";
import { Button } from "../../components/ui/Button";
import { Eyebrow } from "../../components/ui/Eyebrow";
import { ApiError, errorMessage } from "../../lib/api/client";
import { createLesson, listLessons } from "../../lib/api/lessons";
import type { Lesson } from "../../lib/api/types";
import { formatDate, pdfCount } from "./format";
import { LessonFields, readLessonForm, type LessonErrors } from "./LessonFields";

/** The student's lessons, newest first, with a form to start a new one. */
export function LessonsPage() {
  const [lessons, setLessons] = useState<Lesson[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [version, setVersion] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    listLessons(controller.signal).then(
      (next) => {
        setLessons(next);
        setLoadError(null);
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setLoadError(errorMessage(error, "Could not load your lessons. Try again."));
      },
    );
    return () => {
      controller.abort();
    };
  }, [version]);

  return (
    <div>
      <header className="max-w-prose">
        <Eyebrow>Lessons</Eyebrow>
        <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">Lekcije</h1>
        <p className="mt-4 font-serif text-lg text-ink-muted">
          Group your PDFs by lesson, then open them to read.
        </p>
      </header>

      {/* As on the vocabulary page, the form comes first so it sits above the
          list on phones and moves to the right-hand column on wide screens. */}
      <div className="mt-8 grid gap-8 sm:mt-10 lg:grid-cols-[1fr_20rem] lg:items-start lg:gap-10">
        <div className="lg:col-start-2 lg:row-start-1">
          <NewLessonForm />
        </div>

        <section aria-labelledby="lessons-list-heading" className="min-w-0 lg:col-start-1 lg:row-start-1">
          <h2 id="lessons-list-heading" className="sr-only">
            Your lessons
          </h2>
          {loadError ? (
            <div role="alert" className="flex flex-wrap items-center gap-4">
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
          ) : !lessons ? (
            <p role="status" className="text-ink-muted">
              Loading your lessons…
            </p>
          ) : lessons.length === 0 ? (
            <p className="text-ink-muted">No lessons yet. Create your first one to upload PDFs into it.</p>
          ) : (
            <ul className="border-t border-rule">
              {lessons.map((lesson) => (
                <li key={lesson.id} className="border-b border-rule">
                  <Link
                    to={`/lessons/${lesson.id}`}
                    className="group grid gap-1 py-4 sm:grid-cols-[1fr_auto] sm:items-baseline sm:gap-6"
                  >
                    <span className="min-w-0">
                      <span className="block font-serif text-xl text-ink group-hover:text-accent">{lesson.title}</span>
                      {lesson.description && (
                        <span className="mt-1 block truncate text-ink-muted">{lesson.description}</span>
                      )}
                    </span>
                    <span className="text-sm text-ink-subtle">
                      {pdfCount(lesson.documentCount)} · {formatDate(lesson.createdAt)}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}

/** Creates a lesson and opens it, so the next step, uploading, is right there. */
function NewLessonForm() {
  const navigate = useNavigate();
  const [errors, setErrors] = useState<LessonErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const { input, errors: found } = readLessonForm(event.currentTarget);
    setErrors(found);
    setFormError(null);
    if (Object.keys(found).length > 0) return;

    setSubmitting(true);
    try {
      const lesson = await createLesson(input);
      await navigate(`/lessons/${lesson.id}`);
    } catch (error) {
      if (error instanceof ApiError && Object.keys(error.fields).length > 0) setErrors(error.fields);
      else setFormError(errorMessage(error, "Could not create the lesson. Try again."));
      setSubmitting(false);
    }
  }

  return (
    <section
      aria-labelledby="new-lesson-heading"
      className="rounded-md border border-rule bg-paper-raised p-4 sm:p-6 lg:sticky lg:top-6"
    >
      <h2 id="new-lesson-heading" className="font-serif text-xl text-ink">
        New lesson
      </h2>
      <form
        noValidate
        onSubmit={(event) => {
          void handleSubmit(event);
        }}
        className="mt-5 grid gap-4"
      >
        <LessonFields idPrefix="new-lesson" errors={errors} />
        {formError && (
          <p role="alert" className="text-sm text-danger">
            {formError}
          </p>
        )}
        <Button type="submit" className="mt-1 w-full" disabled={submitting}>
          Create lesson
        </Button>
      </form>
    </section>
  );
}
