import { apiRequest } from "./client";
import type {
  NewVocabularyEntryInput,
  VocabularyEntry,
  VocabularyEntryEnvelope,
  VocabularyEntryInput,
  VocabularyPage,
  VocabularySort,
} from "./types";

export type VocabularyQuery = {
  query: string;
  /** 0 for all time. */
  days: number;
  sort: VocabularySort;
  page: number;
};

/** Fetches one page. Parameters at their default are left out of the URL. */
export async function listVocabulary(q: VocabularyQuery, signal?: AbortSignal): Promise<VocabularyPage> {
  const params = new URLSearchParams();
  if (q.query !== "") params.set("query", q.query);
  if (q.days !== 0) params.set("days", String(q.days));
  if (q.sort !== "newest") params.set("sort", q.sort);
  if (q.page !== 1) params.set("page", String(q.page));
  const search = params.toString();
  return apiRequest<VocabularyPage>("GET", search ? `/api/vocabulary?${search}` : "/api/vocabulary", undefined, signal);
}

/** Adds a word; `sourceAnnotationId` names the highlight it was saved from, if any. */
export async function createVocabularyEntry(input: NewVocabularyEntryInput): Promise<VocabularyEntry> {
  const { entry } = await apiRequest<VocabularyEntryEnvelope>("POST", "/api/vocabulary", input);
  return entry;
}

/** Replaces every field of the entry; see PUT /api/vocabulary/{id}. */
export async function updateVocabularyEntry(id: string, input: VocabularyEntryInput): Promise<VocabularyEntry> {
  const { entry } = await apiRequest<VocabularyEntryEnvelope>("PUT", `/api/vocabulary/${encodeURIComponent(id)}`, input);
  return entry;
}

export async function deleteVocabularyEntry(id: string): Promise<void> {
  await apiRequest<undefined>("DELETE", `/api/vocabulary/${encodeURIComponent(id)}`);
}
