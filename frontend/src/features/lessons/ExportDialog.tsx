import { useEffect, useId, useRef, useState, type SubmitEvent } from "react";
import { Button } from "../../components/ui/Button";
import type { ExportFormat } from "./exportDocument";

/** Which pages go into an exported PDF. */
export type ExportPages = "current" | "written" | "all";

type ExportDialogProps = {
  open: boolean;
  /** The page being viewed. */
  page: number;
  /** The pages written or drawn on, in order. */
  writtenPages: readonly number[];
  /** Makes and saves the file; rejects when it could not. */
  onExport: (format: ExportFormat, pages: ExportPages) => Promise<void>;
  onClose: () => void;
};

const FORMATS: { value: ExportFormat; label: string; hint: string }[] = [
  { value: "pdf", label: "PDF", hint: "One or more pages, to send or print." },
  { value: "png", label: "PNG", hint: "A picture of this page, sharpest for text." },
  { value: "jpeg", label: "JPEG", hint: "A picture of this page, the smallest file." },
];

const pagesLabel = (pages: readonly number[]) =>
  pages.length === 1 ? `page ${String(pages[0])}` : `${String(pages.length)} pages`;

/**
 * Saves the worksheet with what was written and drawn on it, as a PDF or a
 * picture of the page being viewed, ready to send to a teacher.
 */
export function ExportDialog({ open, page, writtenPages, onExport, onClose }: ExportDialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const [format, setFormat] = useState<ExportFormat>("pdf");
  const [pages, setPages] = useState<ExportPages>("current");
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  function close() {
    if (working) return;
    setError(null);
    onClose();
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    setWorking(true);
    setError(null);
    try {
      await onExport(format, format === "pdf" ? pages : "current");
      setWorking(false);
      onClose();
    } catch (err) {
      console.error("export document", err);
      setError("Could not export the pages. Try again.");
      setWorking(false);
    }
  }

  const pageOptions: { value: ExportPages; label: string; disabled?: boolean }[] = [
    { value: "current", label: `This page (${String(page)})` },
    {
      value: "written",
      label: writtenPages.length > 0 ? `Pages you wrote on (${pagesLabel(writtenPages)})` : "Pages you wrote on (none yet)",
      disabled: writtenPages.length === 0,
    },
    { value: "all", label: "All pages" },
  ];

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onCancel={(event) => {
        // Escape must not close it while the file is being made.
        if (working) event.preventDefault();
      }}
      onClose={close}
      className="m-auto max-h-[calc(100%-2rem)] w-[calc(100%-2rem)] max-w-md overflow-auto rounded-md border border-rule bg-paper-raised p-6 text-ink backdrop:bg-[rgb(0_0_0/0.5)]"
    >
      <h2 id={titleId} className="font-serif text-xl">
        Export with your writing
      </h2>
      <p className="mt-2 text-sm text-ink-muted">
        Saves the pages with what you wrote and drew on them, to send to your teacher. Highlights and notes are left
        out.
      </p>
      <form
        onSubmit={(event) => {
          void handleSubmit(event);
        }}
        className="mt-5 grid gap-5"
      >
        <fieldset className="grid gap-2" disabled={working}>
          <legend className="mb-1 text-sm font-medium text-ink">Format</legend>
          {FORMATS.map((f) => (
            <label key={f.value} className="flex items-start gap-2 text-sm">
              <input
                type="radio"
                name="export-format"
                value={f.value}
                checked={format === f.value}
                onChange={() => {
                  setFormat(f.value);
                }}
                className="mt-1 accent-accent"
              />
              <span>
                <span className="font-medium">{f.label}</span>
                <span className="text-ink-muted"> — {f.hint}</span>
              </span>
            </label>
          ))}
        </fieldset>

        {format === "pdf" ? (
          <fieldset className="grid gap-2" disabled={working}>
            <legend className="mb-1 text-sm font-medium text-ink">Pages</legend>
            {pageOptions.map((o) => (
              <label key={o.value} className="flex items-center gap-2 text-sm has-disabled:text-ink-subtle">
                <input
                  type="radio"
                  name="export-pages"
                  value={o.value}
                  checked={pages === o.value}
                  disabled={o.disabled}
                  onChange={() => {
                    setPages(o.value);
                  }}
                  className="accent-accent"
                />
                {o.label}
              </label>
            ))}
          </fieldset>
        ) : (
          <p className="text-sm text-ink-muted">The picture is of the page you are on, page {page}.</p>
        )}

        {error && (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        )}
        {/* Always rendered so screen readers announce the text when it appears. */}
        <p role="status" className="text-sm text-ink-muted empty:hidden">
          {working ? "Preparing the file…" : ""}
        </p>

        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={close} disabled={working}>
            Cancel
          </Button>
          <Button type="submit" disabled={working}>
            Export
          </Button>
        </div>
      </form>
    </dialog>
  );
}
