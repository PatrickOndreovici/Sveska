import { useEffect, useRef, useState, type ChangeEvent, type SubmitEvent } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { Button } from "../../components/ui/Button";
import { ConfirmDialog } from "../../components/ui/ConfirmDialog";
import { Eyebrow } from "../../components/ui/Eyebrow";
import { TextLink } from "../../components/ui/TextLink";
import { ApiError, errorMessage } from "../../lib/api/client";
import {
  deleteDocument,
  deleteLesson,
  getLesson,
  MAX_DOCUMENT_BYTES,
  updateLesson,
  uploadDocument,
} from "../../lib/api/lessons";
import type { LessonDetail, LessonDocument } from "../../lib/api/types";
import { formatBytes, formatDate, pdfCount } from "./format";
import { LessonFields, readLessonForm, type LessonErrors } from "./LessonFields";

type Pending = { kind: "lesson" } | { kind: "document"; document: LessonDocument };

/** One lesson: its title and description, and the PDFs in it. */
export function LessonPage() {
  const { lessonId = "" } = useParams();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<LessonDetail | null>(null);
  const [loadError, setLoadError] = useState<{ message: string; notFound: boolean } | null>(null);
  // Bumped after every change so the lesson is fetched again.
  const [version, setVersion] = useState(0);
  const [editing, setEditing] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<Pending | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    getLesson(lessonId, controller.signal).then(
      (next) => {
        setDetail(next);
        setLoadError(null);
      },
      (error: unknown) => {
        if (controller.signal.aborted) return;
        setLoadError({
          message: errorMessage(error, "Could not load this lesson. Try again."),
          notFound: error instanceof ApiError && error.status === 404,
        });
      },
    );
    return () => {
      controller.abort();
    };
  }, [lessonId, version]);

  function reload() {
    setVersion((v) => v + 1);
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    const pending = pendingDelete;
    setPendingDelete(null);
    setDeleteError(null);
    try {
      if (pending.kind === "lesson") {
        await deleteLesson(lessonId);
        await navigate("/lessons");
        return;
      }
      await deleteDocument(pending.document.id);
    } catch (error) {
      setDeleteError(errorMessage(error, "Could not delete. Try again."));
    }
    reload();
  }

  if (loadError) {
    return (
      <div className="max-w-prose">
        <TextLink to="/lessons">← All lessons</TextLink>
        <div role="alert" className="mt-8 flex flex-wrap items-center gap-4">
          <p className="text-danger">{loadError.message}</p>
          {!loadError.notFound && (
            <Button variant="secondary" size="sm" onClick={reload}>
              Try again
            </Button>
          )}
        </div>
      </div>
    );
  }
  if (!detail) {
    return (
      <p role="status" className="text-ink-muted">
        Loading the lesson…
      </p>
    );
  }

  const { lesson, documents } = detail;

  return (
    <div>
      <TextLink to="/lessons">← All lessons</TextLink>

      {editing ? (
        <EditLessonForm
          lessonId={lessonId}
          initial={{ title: lesson.title, description: lesson.description }}
          onDone={(saved) => {
            setEditing(false);
            if (saved) reload();
          }}
        />
      ) : (
        <header className="mt-6 max-w-prose">
          <Eyebrow>Lesson</Eyebrow>
          <h1 className="mt-3 font-serif text-4xl break-words text-ink sm:text-display">{lesson.title}</h1>
          {lesson.description && <p className="mt-4 font-serif text-lg text-ink-muted">{lesson.description}</p>}
          <div className="mt-4 flex gap-4">
            <Button
              variant="quiet"
              size="sm"
              className="px-0"
              onClick={() => {
                setEditing(true);
              }}
            >
              Edit lesson
            </Button>
            <Button
              variant="quiet"
              size="sm"
              className="px-0"
              onClick={() => {
                setPendingDelete({ kind: "lesson" });
              }}
            >
              Delete lesson
            </Button>
          </div>
        </header>
      )}

      <section aria-labelledby="lesson-documents-heading" className="mt-10 max-w-3xl">
        <h2 id="lesson-documents-heading" className="font-serif text-2xl text-ink">
          PDFs
        </h2>
        <UploadField lessonId={lessonId} onUploaded={reload} />

        {deleteError && (
          <p role="alert" className="mt-4 text-sm text-danger">
            {deleteError}
          </p>
        )}

        {documents.length === 0 ? (
          <p className="mt-6 text-ink-muted">No PDFs in this lesson yet.</p>
        ) : (
          <ul className="mt-6 border-t border-rule">
            {documents.map((doc) => (
              <li
                key={doc.id}
                className="grid gap-1 border-b border-rule py-4 sm:grid-cols-[1fr_auto] sm:items-baseline sm:gap-6"
              >
                <Link
                  to={`/documents/${doc.id}`}
                  className="min-w-0 font-serif text-lg break-words text-ink hover:text-accent"
                >
                  {doc.title}
                </Link>
                <div className="flex items-center gap-4 text-sm text-ink-subtle">
                  <span>
                    {formatBytes(doc.byteSize)} · {formatDate(doc.createdAt)}
                  </span>
                  <Button
                    variant="quiet"
                    size="sm"
                    className="px-0"
                    aria-label={`Delete ${doc.title}`}
                    onClick={() => {
                      setPendingDelete({ kind: "document", document: doc });
                    }}
                  >
                    Delete
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <ConfirmDialog
        open={pendingDelete !== null}
        title={pendingDelete?.kind === "lesson" ? "Delete this lesson?" : "Delete this PDF?"}
        confirmLabel="Delete"
        onConfirm={() => {
          void confirmDelete();
        }}
        onCancel={() => {
          setPendingDelete(null);
        }}
      >
        {pendingDelete?.kind === "lesson" && (
          <p>
            <span className="font-medium text-ink">{lesson.title}</span>
            {documents.length > 0 ? ` and its ${pdfCount(documents.length)}` : ""} will be deleted. This can’t be
            undone.
          </p>
        )}
        {pendingDelete?.kind === "document" && (
          <p>
            <span className="font-medium text-ink">{pendingDelete.document.title}</span> will be removed from this
            lesson. This can’t be undone.
          </p>
        )}
      </ConfirmDialog>
    </div>
  );
}

/** A file picker that uploads the chosen PDF straight away. */
function UploadField({ lessonId, onUploaded }: { lessonId: string; onUploaded: () => void }) {
  const [uploading, setUploading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [uploaded, setUploaded] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  async function handleChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    // Cleared so choosing the same file again still fires a change.
    event.target.value = "";
    if (!file) return;
    setError(null);
    setUploaded(null);
    if (file.size > MAX_DOCUMENT_BYTES) {
      setError("Choose a PDF of at most 20 MB.");
      return;
    }

    setUploading(file.name);
    try {
      const doc = await uploadDocument(lessonId, file);
      setUploaded(doc.title);
      onUploaded();
    } catch (err) {
      setError(
        err instanceof ApiError && err.fields["file"]
          ? err.fields["file"]
          : errorMessage(err, "Could not upload the PDF. Try again."),
      );
    } finally {
      setUploading(null);
      inputRef.current?.focus();
    }
  }

  return (
    <div className="mt-4 grid gap-1.5">
      <label htmlFor="lesson-upload" className="text-sm font-medium text-ink">
        Add a PDF
      </label>
      <input
        ref={inputRef}
        id="lesson-upload"
        type="file"
        accept="application/pdf,.pdf"
        disabled={uploading !== null}
        aria-describedby="lesson-upload-hint"
        aria-invalid={error ? true : undefined}
        onChange={(event) => {
          void handleChange(event);
        }}
        className="block w-full text-sm text-ink-muted file:mr-3 file:h-8 file:cursor-pointer file:rounded-sm file:border file:border-rule-strong file:bg-paper-raised file:px-3 file:text-sm file:font-medium file:text-ink hover:file:border-ink disabled:opacity-50"
      />
      <p id="lesson-upload-hint" className="text-sm text-ink-subtle">
        PDF files up to 20 MB.
      </p>
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
      {/* Always rendered so screen readers announce the text when it appears. */}
      <p role="status" className="text-sm text-ink-muted empty:hidden">
        {uploading ? `Uploading ${uploading}…` : uploaded ? `Added ${uploaded}.` : ""}
      </p>
    </div>
  );
}

function EditLessonForm({
  lessonId,
  initial,
  onDone,
}: {
  lessonId: string;
  initial: { title: string; description: string };
  onDone: (saved: boolean) => void;
}) {
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
      await updateLesson(lessonId, input);
      onDone(true);
    } catch (error) {
      if (error instanceof ApiError && Object.keys(error.fields).length > 0) setErrors(error.fields);
      else setFormError(errorMessage(error, "Could not save the lesson. Try again."));
      setSubmitting(false);
    }
  }

  return (
    <form
      noValidate
      aria-label="Edit lesson"
      onSubmit={(event) => {
        void handleSubmit(event);
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape") onDone(false);
      }}
      className="mt-6 grid max-w-prose gap-4"
    >
      <LessonFields idPrefix="edit-lesson" initial={initial} errors={errors} autoFocus />
      {formError && (
        <p role="alert" className="text-sm text-danger">
          {formError}
        </p>
      )}
      <div className="flex gap-3">
        <Button type="submit" size="sm" disabled={submitting}>
          Save
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => {
            onDone(false);
          }}
        >
          Cancel
        </Button>
      </div>
    </form>
  );
}
