import { useState } from "react";
import { Button } from "../../components/ui/Button";
import { ConfirmDialog } from "../../components/ui/ConfirmDialog";
import { cx } from "../../components/ui/cx";
import { Eyebrow } from "../../components/ui/Eyebrow";
import { Select } from "../../components/ui/Select";
import { TextField } from "../../components/ui/TextField";
import { AddWordForm } from "./AddWordForm";
import type { EntryFields } from "./entryFields";
import { mockEntries, type MockVocabularyEntry } from "./mockEntries";
import { VocabularyRow } from "./VocabularyRow";

// MOCK: entries live in component state only and reset on reload. Search,
// time range, sort and paging all happen here in the browser; the real
// version sends them to GET /api/vocabulary and gets one page back. Add,
// edit and delete become POST, PATCH and DELETE calls.

const PAGE_SIZE = 10;
const DAY_MS = 24 * 60 * 60 * 1000;

type Range = "1d" | "7d" | "30d" | "all";
type Sort = "newest" | "az";

const ranges: { value: Range; label: string; days: number | null }[] = [
  { value: "1d", label: "1 day", days: 1 },
  { value: "7d", label: "7 days", days: 7 },
  { value: "30d", label: "30 days", days: 30 },
  { value: "all", label: "All time", days: null },
];

// Serbian Latin alphabet order: c < č < ć < d < dž < đ ... (PostgreSQL: COLLATE "sr-Latn-x-icu").
const serbianOrder = new Intl.Collator("sr-Latn", { sensitivity: "base" });

/**
 * Lowercase and strip diacritics, so "c" finds "č" and "ć". NFD splits "č"
 * into "c" plus a combining mark, which is then removed. "đ" is its own
 * letter rather than "d" plus a mark, so it is mapped by hand.
 * (PostgreSQL: lower(unaccent(...)).)
 */
function fold(text: string): string {
  return text.toLowerCase().normalize("NFD").replace(/\p{M}/gu, "").replace(/đ/g, "d");
}

/**
 * Case- and diacritic-insensitive substring of the word, its translation or
 * its other forms, so "ljudi" finds "čovek".
 */
function matches(entry: MockVocabularyEntry, query: string): boolean {
  const q = fold(query.trim());
  if (q === "") return true;
  const searchable = [
    entry.term,
    entry.translation,
    entry.plural,
    entry.aspectPair,
    entry.feminineForm,
    entry.neuterForm,
  ];
  return searchable.some((text) => fold(text).includes(q));
}

function inRange(entry: MockVocabularyEntry, range: Range): boolean {
  const days = ranges.find((r) => r.value === range)?.days ?? null;
  if (days === null) return true;
  return Date.now() - new Date(entry.createdAt).getTime() <= days * DAY_MS;
}

function compare(sort: Sort) {
  return (a: MockVocabularyEntry, b: MockVocabularyEntry): number =>
    sort === "az"
      ? serbianOrder.compare(a.term, b.term) || serbianOrder.compare(a.translation, b.translation)
      : b.createdAt.localeCompare(a.createdAt);
}

export function VocabularyPage() {
  const [entries, setEntries] = useState(mockEntries);
  const [query, setQuery] = useState("");
  const [range, setRange] = useState<Range>("all");
  const [sort, setSort] = useState<Sort>("newest");
  const [page, setPage] = useState(1);
  const [pendingDelete, setPendingDelete] = useState<MockVocabularyEntry | null>(null);

  const filtered = entries.filter((entry) => matches(entry, query) && inRange(entry, range)).sort(compare(sort));
  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  // Deleting the last entry on the last page would otherwise leave an empty page.
  const currentPage = Math.min(page, pageCount);
  const firstIndex = (currentPage - 1) * PAGE_SIZE;
  const visible = filtered.slice(firstIndex, firstIndex + PAGE_SIZE);

  /** Mirrors the unique index: same owner, same term ignoring case, same translation. */
  function isDuplicate(fields: EntryFields, exceptId?: string): boolean {
    return entries.some(
      (entry) =>
        entry.id !== exceptId &&
        entry.term.toLowerCase() === fields.term.toLowerCase() &&
        entry.translation === fields.translation,
    );
  }

  function handleAdd(fields: EntryFields): boolean {
    if (isDuplicate(fields)) return false;
    setEntries([{ id: crypto.randomUUID(), ...fields, createdAt: new Date().toISOString() }, ...entries]);
    return true;
  }

  function handleSave(id: string, fields: EntryFields): boolean {
    if (isDuplicate(fields, id)) return false;
    setEntries(entries.map((entry) => (entry.id === id ? { ...entry, ...fields } : entry)));
    return true;
  }

  function confirmDelete() {
    if (pendingDelete) setEntries(entries.filter((entry) => entry.id !== pendingDelete.id));
    setPendingDelete(null);
  }

  return (
    <div>
      <header className="max-w-prose">
        <Eyebrow>Vocabulary</Eyebrow>
        <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">Rečnik</h1>
        <p className="mt-4 font-serif text-lg text-ink-muted">
          {entries.length} {entries.length === 1 ? "word" : "words"} collected so far.
        </p>
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

          {visible.length === 0 ? (
            <p className="mt-8 text-ink-muted">
              {entries.length === 0
                ? "No words yet. Add your first one."
                : query.trim() === ""
                  ? "No words added in this period."
                  : `Nothing matches “${query.trim()}” in this period.`}
            </p>
          ) : (
            <>
              <ul className="mt-6 border-t border-rule">
                {visible.map((entry) => (
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
                  {firstIndex + 1}–{firstIndex + visible.length} of {filtered.length}
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
        onConfirm={confirmDelete}
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
