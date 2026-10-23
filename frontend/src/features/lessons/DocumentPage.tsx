import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { TextLink } from "../../components/ui/TextLink";
import { errorMessage } from "../../lib/api/client";
import { documentFileUrl, getDocument } from "../../lib/api/lessons";
import type { DocumentDetail } from "../../lib/api/types";
import { PdfViewer } from "./PdfViewer";

/** Reading one PDF: its title, a way back to the lesson, and the viewer. */
export function DocumentPage() {
  const { documentId = "" } = useParams();
  const [detail, setDetail] = useState<DocumentDetail | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    getDocument(documentId, controller.signal).then(
      (next) => {
        setDetail(next);
        setLoadError(null);
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setLoadError(errorMessage(error, "Could not load this PDF. Try again."));
      },
    );
    return () => {
      controller.abort();
    };
  }, [documentId]);

  if (loadError) {
    return (
      <div className="max-w-prose">
        <TextLink to="/lessons">← All lessons</TextLink>
        <p role="alert" className="mt-8 text-danger">
          {loadError}
        </p>
      </div>
    );
  }
  if (!detail) {
    return (
      <p role="status" className="text-ink-muted">
        Loading…
      </p>
    );
  }

  const fileUrl = documentFileUrl(documentId);

  return (
    <div>
      <TextLink to={`/lessons/${detail.lesson.id}`}>← {detail.lesson.title}</TextLink>
      <header className="mt-6 flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
        <h1 className="min-w-0 font-serif text-3xl break-words text-ink sm:text-4xl">{detail.document.title}</h1>
        <a
          href={fileUrl}
          download
          className="text-sm text-accent underline decoration-rule-strong underline-offset-4 hover:decoration-accent"
        >
          Download
        </a>
      </header>
      <div className="mt-6">
        <PdfViewer url={fileUrl} title={detail.document.title} />
      </div>
    </div>
  );
}
