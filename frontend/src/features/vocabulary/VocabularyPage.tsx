import { useEffect, useState } from "react";
import { Button } from "../../components/ui/Button";
import { ConfirmDialog } from "../../components/ui/ConfirmDialog";
import { cx } from "../../components/ui/cx";
import { Eyebrow } from "../../components/ui/Eyebrow";
import { Select } from "../../components/ui/Select";
import { TextField } from "../../components/ui/TextField";
import { ApiError } from "../../lib/api/client";
import type { VocabularyEntry, VocabularyPage as Page } from "../../lib/api/types";
import {
  createVocabularyEntry,
  deleteVocabularyEntry,
  listVocabulary,
  updateVocabularyEntry,
} from "../../lib/api/vocabulary";
import { AddWordForm } from "./AddWordForm";
import type { EntryFields } from "./entryFields";
import { useDebouncedValue } from "./useDebouncedValue";
import { VocabularyRow } from "./VocabularyRow";

// Search, time range, sort and paging all happen in the API: the page sends
// them to GET /api/vocabulary and shows the one page that comes back.

/** How long typing must pause before the search is sent. */
const SEARCH_DELAY_MS = 300;

type Range = "1d" | "7d" | "30d" | "all";
type Sort = "newest" | "az";

const ranges: { value: Range; label: string; days: number }[] = [
  { value: "1d", label: "1 day", days: 1 },
  { value: "7d", label: "7 days", days: 7 },
  { value: "30d", label: "30 days", days: 30 },
  { value: "all", label: "All time", days: 0 },
];

function messageOf(error: unknown, fallback: string): string {
  return error instanceof ApiError ? error.message : fallback;
}

export function VocabularyPage() {
  const [query, setQuery] = useState("");
  const search = useDebouncedValue(query.trim(), SEARCH_DELAY_MS);
  const [range, setRange] = useState<Range>("all");
  const [sort, setSort] = useState<Sort>("newest");
  const [page, setPage] = useState(1);
  // Bumped after every add, edit or delete so the current page is fetched again.
  const [version, setVersion] = useState(0);
  const [result, setResult] = useState<Page | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<VocabularyEntry | null>(null);

  const days = ranges.find((r) => r.value === range)?.days ?? 0;

  useEffect(() => {
    // Aborting on cleanup means a slow response for an old search can never
    // overwrite the result of a newer one.
    const controller = new AbortController();
    listVocabulary({ query: search, days, sort, page }, controller.signal).then(
      (next) => {
        // Deleting the last word on the last page leaves that page empty: step back.
        const lastPage = Math.max(1, Math.ceil(next.total / next.pageSize));
        if (page > lastPage) {
          setPage(lastPage);
          return;
        }
        setResult(next);
        setLoadError(null);
      },
      (error: unknown) => {
        if (controller.signal.aborted) return;
        setLoadError(messageOf(error, "Could not load your words. Try again."));
      },
    );
    return () => {
      controller.abort();
    };
  }, [search, days, sort, page, version]);

  function reload() {
    setVersion((v) => v + 1);
  }

  async function handleAdd(fields: EntryFields) {
    await createVocabularyEntry(fields);
    reload();
  }

  async function handleSave(id: string, fields: EntryFields) {
    await updateVocabularyEntry(id, fields);
    reload();
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    const entry = pendingDelete;
    setPendingDelete(null);
    setDeleteError(null);
    try {
      await deleteVocabularyEntry(entry.id);
    } catch (error) {
      setDeleteError(messageOf(error, "Could not delete the word. Try again."));
    }
    reload();
  }

  const entries = result?.entries ?? [];
  const total = result?.total ?? 0;
  const pageSize = result?.pageSize ?? 1;
  const currentPage = result?.page ?? 1;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const firstIndex = (currentPage - 1) * pageSize;

  return (
    <div>
      <header className="max-w-prose">
        <Eyebrow>Vocabulary</Eyebrow>
        <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">Rečnik</h1>
        {result && (
          <p className="mt-4 font-serif text-lg text-ink-muted">
            {result.unfilteredTotal} {result.unfilteredTotal === 1 ? "word" : "words"} collected so far.
          </p>
        )}
      </header>

      {/* The form comes first in the page so it sits above the list on phones;
          on wide screens the grid moves it to the right-hand column. */}
      <div className="mt-8 grid gap-8 sm:mt-10 lg:grid-cols-[1fr_20rem] lg:items-start lg:gap-10">
        <div className="lg:col-start-2 lg:row-start-1">
          <AddWordForm onAdd={handleAdd} />
        </div>

        <section aria-labelledby="vocabulary-list-heading" className="min-w-0 lg:col-start-1 lg:row-start-1">
          <h2 id="vocabulary-list-heading" className="sr-only">
            Your words
          </h2>
          <TextField
            id="vocabulary-search"
            type="search"
            label="Search"
            placeholder="Serbian or English"
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
            spellCheck={false}
          />

          <div className="mt-4 flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
            <div className="flex items-center gap-3">
              <span id="vocabulary-range-label" className="text-sm text-ink-muted">
                Added
              </span>
              <div
                role="group"
                aria-labelledby="vocabulary-range-label"
                className="inline-flex rounded-sm border border-rule p-0.5"
              >
                {ranges.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    aria-pressed={range === option.value}
                    onClick={() => {
                      setRange(option.value);
                      setPage(1);
                    }}
                    className={cx(
                      // py-[3px] makes the group 32px tall, the same as the Sort select next to it.
                      "rounded-sm px-2.5 py-[3px] text-sm font-medium whitespace-nowrap transition-colors",
                      range === option.value ? "bg-paper-sunken text-ink" : "text-ink-muted hover:text-ink",
                    )}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-3">
              <label htmlFor="vocabulary-sort" className="text-sm text-ink-muted">
                Sort
              </label>
              <Select
                id="vocabulary-sort"
                size="sm"
                value={sort}
                onChange={(event) => {
                  setSort(event.target.value as Sort);
                  setPage(1);
                }}
              >
                <option value="newest">Newest first</option>
                <option value="az">A → Z</option>
              </Select>
            </div>
          </div>

          {deleteError && (
            <p role="alert" className="mt-6 text-sm text-danger">
              {deleteError}
            </p>
          )}

          {loadError ? (
            <div role="alert" className="mt-8 flex flex-wrap items-center gap-4">
              <p className="text-danger">{loadError}</p>
              <Button variant="secondary" size="sm" onClick={reload}>
                Try again
              </Button>
            </div>
          ) : !result ? (
            <p role="status" className="mt-8 text-ink-muted">
              Loading your words…
            </p>
          ) : entries.length === 0 ? (
            <p className="mt-8 text-ink-muted">
              {result.unfilteredTotal === 0
                ? "No words yet. Add your first one."
                : query.trim() === ""
                  ? "No words added in this period."
                  : `Nothing matches “${query.trim()}” in this period.`}
            </p>
          ) : (
            <>
              <ul className="mt-6 border-t border-rule">
                {entries.map((entry) => (
                  <VocabularyRow
                    key={entry.id}
                    entry={entry}
                    onSave={(fields) => handleSave(entry.id, fields)}
                    onDelete={() => {
                      setPendingDelete(entry);
                    }}
                  />
                ))}
              </ul>

              <nav aria-label="Pages" className="mt-6 flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-ink-muted">
                  {firstIndex + 1}–{firstIndex + entries.length} of {total}
                </p>
                {pageCount > 1 && (
                  <div className="flex items-center gap-3">
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={currentPage === 1}
                      onClick={() => {
                        setPage(currentPage - 1);
                      }}
                    >
                      Previous
                    </Button>
                    <span className="text-sm text-ink-muted">
                      {currentPage} / {pageCount}
                    </span>
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={currentPage === pageCount}
                      onClick={() => {
                        setPage(currentPage + 1);
                      }}
                    >
                      Next
                    </Button>
                  </div>
                )}
              </nav>
            </>
          )}
        </section>
      </div>

      <ConfirmDialog
        open={pendingDelete !== null}
        title="Delete this word?"
        confirmLabel="Delete"
        onConfirm={() => {
          void confirmDelete();
        }}
        onCancel={() => {
          setPendingDelete(null);
        }}
      >
        {pendingDelete && (
          <p>
            <span lang="sr" className="font-medium text-ink">
              {pendingDelete.term}
            </span>{" "}
            ({pendingDelete.translation}) will be removed from your vocabulary. This can’t be undone.
          </p>
        )}
      </ConfirmDialog>
    </div>
  );
}
