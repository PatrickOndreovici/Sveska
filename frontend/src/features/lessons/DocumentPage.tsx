import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useSearchParams } from "react-router";
import { Button } from "../../components/ui/Button";
import { ConfirmDialog } from "../../components/ui/ConfirmDialog";
import { TextLink } from "../../components/ui/TextLink";
import {
  createAnnotation,
  deleteAnnotation,
  listAnnotations,
  MAX_QUOTE_LENGTH,
  updateAnnotationNote,
} from "../../lib/api/annotations";
import { ApiError, errorMessage } from "../../lib/api/client";
import { DEFAULT_FONT_SIZE } from "../../lib/api/drawings";
import { documentFileUrl, getDocument } from "../../lib/api/lessons";
import type { Annotation, DocumentDetail, InkColor } from "../../lib/api/types";
import { createVocabularyEntry } from "../../lib/api/vocabulary";
import { clearSelection } from "../../lib/pdf/selection";
import type { EntryFields } from "../vocabulary/entryFields";
import { AnnotationPanel } from "./AnnotationPanel";
import { DrawingLayer, type Tool } from "./DrawingLayer";
import { DrawingTools } from "./DrawingTools";
import { ExportDialog, type ExportPages } from "./ExportDialog";
import { exportDocument, pagesWithInk } from "./exportDocument";
import { PdfViewer, type TextSelection } from "./PdfViewer";
import { SaveWordDialog } from "./SaveWordDialog";
import { useDrawings } from "./useDrawings";

/** The API's limit on rectangles per highlight. */
const MAX_RECTS = 200;

/** Reading order: by page, then oldest first, as the API lists them. */
function byReadingOrder(a: Annotation, b: Annotation): number {
  return a.pageNumber - b.pageNumber || a.createdAt.localeCompare(b.createdAt);
}

/** ?page=3, or the first page when it is missing or not a page number. */
function pageFrom(params: URLSearchParams): number {
  const n = Number(params.get("page"));
  return Number.isInteger(n) && n >= 1 ? n : 1;
}

/** Ctrl+Z or ⌘Z, except while typing, where it undoes the typing instead. */
function isUndoShortcut(event: KeyboardEvent): boolean {
  const typing =
    event.target instanceof HTMLElement &&
    (event.target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(event.target.tagName));
  return (event.ctrlKey || event.metaKey) && !event.shiftKey && event.key.toLowerCase() === "z" && !typing;
}

/**
 * Reading one PDF: the viewer, and beside it the highlights and notes made
 * on it. Selected text can be highlighted, given a note, or saved as a word;
 * with the other tools the student writes and draws on the page, as on a
 * worksheet. The page is kept in the URL, so links can open a given page.
 */
export function DocumentPage() {
  const { documentId = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const page = pageFrom(searchParams);
  const [detail, setDetail] = useState<DocumentDetail | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [annotations, setAnnotations] = useState<Annotation[] | null>(null);
  const [annotationsError, setAnnotationsError] = useState<string | null>(null);
  const [annotationsVersion, setAnnotationsVersion] = useState(0);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Annotation | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [savedWord, setSavedWord] = useState<string | null>(null);
  const [wordSelection, setWordSelection] = useState<TextSelection | null>(null);
  // The highlight made for the word being saved, so a retry after a refused
  // word (a duplicate, say) does not highlight the same text twice.
  const wordHighlight = useRef<Annotation | null>(null);
  const drawings = useDrawings(documentId);
  const { undo } = drawings;
  const [tool, setTool] = useState<Tool>("select");
  const [color, setColor] = useState<InkColor>("black");
  const [fontSize, setFontSize] = useState(DEFAULT_FONT_SIZE);
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!isUndoShortcut(event)) return;
      event.preventDefault();
      undo();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [undo]);

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

  useEffect(() => {
    const controller = new AbortController();
    listAnnotations(documentId, controller.signal).then(
      (next) => {
        setAnnotations(next);
        setAnnotationsError(null);
      },
      (error: unknown) => {
        if (!controller.signal.aborted) {
          setAnnotationsError(errorMessage(error, "Could not load the highlights. Try again."));
        }
      },
    );
    return () => {
      controller.abort();
    };
  }, [documentId, annotationsVersion]);

  const setPage = useCallback(
    (n: number) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set("page", String(n));
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  function added(annotation: Annotation) {
    setAnnotations((list) => [...(list ?? []), annotation].sort(byReadingOrder));
    setActiveId(annotation.id);
  }

  async function highlight(selection: TextSelection, withNote: boolean) {
    setActionError(null);
    setSavedWord(null);
    try {
      const annotation = await createAnnotation(documentId, {
        pageNumber: selection.pageNumber,
        rects: selection.rects,
        quote: selection.quote,
        note: "",
      });
      added(annotation);
      if (withNote) setEditingId(annotation.id);
      clearSelection();
    } catch (error) {
      setActionError(errorMessage(error, "Could not save the highlight. Try again."));
    }
  }

  async function saveWord(fields: EntryFields) {
    if (!wordSelection) return;
    let source = wordHighlight.current;
    if (!source) {
      try {
        source = await createAnnotation(documentId, {
          pageNumber: wordSelection.pageNumber,
          rects: wordSelection.rects,
          quote: wordSelection.quote,
          note: "",
        });
      } catch (error) {
        // Its field problems are about the selection, which the word form has
        // no inputs for, so they become one message for the whole form.
        if (error instanceof ApiError && Object.keys(error.fields).length > 0) {
          throw new ApiError(error.status, error.code, "Could not highlight the selected text. Select it again.");
        }
        throw error;
      }
      wordHighlight.current = source;
      added(source);
    }
    await createVocabularyEntry({ ...fields, sourceAnnotationId: source.id });
    setSavedWord(fields.term);
  }

  async function saveNote(id: string, note: string) {
    const updated = await updateAnnotationNote(id, note);
    setAnnotations((list) => list?.map((a) => (a.id === id ? updated : a)) ?? null);
    setEditingId(null);
  }

  async function confirmDelete() {
    const target = pendingDelete;
    setPendingDelete(null);
    if (!target) return;
    setActionError(null);
    try {
      await deleteAnnotation(target.id);
      setAnnotations((list) => list?.filter((a) => a.id !== target.id) ?? null);
      if (activeId === target.id) setActiveId(null);
    } catch (error) {
      setActionError(errorMessage(error, "Could not delete the highlight. Try again."));
    }
  }

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
  const writtenPages = pagesWithInk(drawings.strokes ?? [], drawings.textBoxes ?? []);
  const exportPages = (pages: ExportPages) =>
    pages === "all" ? "all" : pages === "written" ? writtenPages : [page];

  return (
    <div>
      <TextLink to={`/lessons/${detail.lesson.id}`}>← {detail.lesson.title}</TextLink>
      <header className="mt-6 flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
        <h1 className="min-w-0 font-serif text-3xl break-words text-ink sm:text-4xl">{detail.document.title}</h1>
        <div className="flex items-center gap-4">
          <a
            href={fileUrl}
            download
            className="text-sm text-accent underline decoration-rule-strong underline-offset-4 hover:decoration-accent"
          >
            Download
          </a>
          <Button
            variant="secondary"
            size="sm"
            // What was written must be loaded, or it would be missing from the file.
            disabled={drawings.strokes === null || drawings.textBoxes === null}
            onClick={() => {
              setExporting(true);
            }}
          >
            Export
          </Button>
        </div>
      </header>

      {actionError && (
        <p role="alert" className="mt-4 text-sm text-danger">
          {actionError}
        </p>
      )}
      {/* Always rendered so screen readers announce the text when it appears. */}
      <p role="status" className="mt-4 text-sm text-success empty:hidden">
        {savedWord && (
          <>
            Saved{" "}
            <span lang="sr" className="font-medium">
              {savedWord}
            </span>{" "}
            to your <TextLink to="/vocabulary">vocabulary</TextLink>.
          </>
        )}
      </p>

      <div className="mt-6 grid gap-8 lg:grid-cols-[minmax(0,1fr)_20rem] lg:items-start">
        <PdfViewer
          url={fileUrl}
          title={detail.document.title}
          page={page}
          onPageChange={setPage}
          highlights={(annotations ?? []).map((a) => ({
            id: a.id,
            pageNumber: a.pageNumber,
            rects: a.rects,
            hasNote: a.note !== "",
          }))}
          activeHighlightId={activeId}
          onHighlightClick={setActiveId}
          selectable={tool === "select"}
          tools={
            <div className="grid gap-2">
              <DrawingTools
                tool={tool}
                onToolChange={(next) => {
                  if (next !== "select") clearSelection();
                  setTool(next);
                }}
                color={color}
                onColorChange={setColor}
                fontSize={fontSize}
                onFontSizeChange={setFontSize}
                canUndo={drawings.canUndo}
                onUndo={undo}
                drawingReady={drawings.strokes !== null && drawings.textBoxes !== null}
              />
              {drawings.loadError && (
                <div role="alert" className="flex flex-wrap items-center gap-3">
                  <p className="text-sm text-danger">{drawings.loadError}</p>
                  <Button variant="secondary" size="sm" onClick={drawings.reload}>
                    Try again
                  </Button>
                </div>
              )}
              {drawings.saveError && (
                <p role="alert" className="text-sm text-danger">
                  {drawings.saveError}
                </p>
              )}
            </div>
          }
          overlay={({ pageNumber, scale }) => (
            <DrawingLayer
              // A new page starts with nothing being drawn or typed.
              key={pageNumber}
              scale={scale}
              tool={tool}
              color={color}
              fontSize={fontSize}
              onFontSizeChange={setFontSize}
              strokes={(drawings.strokes ?? []).filter((s) => s.pageNumber === pageNumber)}
              textBoxes={(drawings.textBoxes ?? []).filter((b) => b.pageNumber === pageNumber)}
              onDraw={(points) => {
                drawings.draw(pageNumber, color, points);
              }}
              onErase={drawings.erase}
              onAddText={(fields) => {
                drawings.addTextBox(pageNumber, fields);
              }}
              onChangeText={drawings.updateTextBox}
            />
          )}
          selectionActions={(selection, dismiss) =>
            selection.quote.length > MAX_QUOTE_LENGTH || selection.rects.length > MAX_RECTS ? (
              <p className="px-2 py-1 text-sm text-ink-muted">Select a shorter passage to highlight it.</p>
            ) : (
              <>
                <Button
                  variant="quiet"
                  size="sm"
                  onClick={() => {
                    void highlight(selection, false).then(dismiss);
                  }}
                >
                  Highlight
                </Button>
                <Button
                  variant="quiet"
                  size="sm"
                  onClick={() => {
                    void highlight(selection, true).then(dismiss);
                  }}
                >
                  Add note
                </Button>
                <Button
                  variant="quiet"
                  size="sm"
                  onClick={() => {
                    setActionError(null);
                    setSavedWord(null);
                    wordHighlight.current = null;
                    setWordSelection(selection);
                    dismiss();
                  }}
                >
                  Save word
                </Button>
              </>
            )
          }
        />

        <div className="lg:sticky lg:top-6 lg:max-h-[calc(100vh-3rem)] lg:overflow-auto">
          <AnnotationPanel
            annotations={annotations}
            loadError={annotationsError}
            onRetry={() => {
              setAnnotationsVersion((v) => v + 1);
            }}
            activeId={activeId}
            editingId={editingId}
            onSelect={(a) => {
              setActiveId(a.id);
              setPage(a.pageNumber);
            }}
            onEditNote={setEditingId}
            onSaveNote={saveNote}
            onDelete={setPendingDelete}
          />
        </div>
      </div>

      <SaveWordDialog
        quote={wordSelection?.quote ?? null}
        onSave={saveWord}
        onClose={() => {
          setWordSelection(null);
          clearSelection();
        }}
      />

      <ExportDialog
        open={exporting}
        page={page}
        writtenPages={writtenPages}
        onExport={(format, pages) =>
          exportDocument({
            url: fileUrl,
            title: detail.document.title,
            format,
            pages: exportPages(pages),
            strokes: drawings.strokes ?? [],
            textBoxes: drawings.textBoxes ?? [],
          })
        }
        onClose={() => {
          setExporting(false);
        }}
      />

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete this highlight?"
        confirmLabel="Delete"
        onConfirm={() => {
          void confirmDelete();
        }}
        onCancel={() => {
          setPendingDelete(null);
        }}
      >
        <p>
          {pendingDelete?.note ? "The highlight and its note will be deleted." : "The highlight will be deleted."} Words
          you saved from it stay in your vocabulary.
        </p>
      </ConfirmDialog>
    </div>
  );
}
