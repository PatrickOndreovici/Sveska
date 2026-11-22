import { apiRequest } from "./client";
import type {
  DocumentDetail,
  DocumentEnvelope,
  Lesson,
  LessonDetail,
  LessonDocument,
  LessonEnvelope,
  LessonInput,
  LessonList,
} from "./types";

/** The largest PDF the API accepts; checked here too so a big file fails before it uploads. */
export const MAX_DOCUMENT_BYTES = 20 * 1024 * 1024;

export async function listLessons(signal?: AbortSignal): Promise<Lesson[]> {
  const { lessons } = await apiRequest<LessonList>("GET", "/api/lessons", undefined, signal);
  return lessons;
}

export async function createLesson(input: LessonInput): Promise<Lesson> {
  const { lesson } = await apiRequest<LessonEnvelope>("POST", "/api/lessons", input);
  return lesson;
}

export async function getLesson(id: string, signal?: AbortSignal): Promise<LessonDetail> {
  return apiRequest<LessonDetail>("GET", `/api/lessons/${encodeURIComponent(id)}`, undefined, signal);
}

export async function updateLesson(id: string, input: LessonInput): Promise<Lesson> {
  const { lesson } = await apiRequest<LessonEnvelope>("PUT", `/api/lessons/${encodeURIComponent(id)}`, input);
  return lesson;
}

export async function deleteLesson(id: string): Promise<void> {
  await apiRequest<undefined>("DELETE", `/api/lessons/${encodeURIComponent(id)}`);
}

export async function uploadDocument(lessonId: string, file: File): Promise<LessonDocument> {
  const form = new FormData();
  form.append("file", file);
  const { document } = await apiRequest<DocumentEnvelope>(
    "POST",
    `/api/lessons/${encodeURIComponent(lessonId)}/documents`,
    form,
  );
  return document;
}

export async function getDocument(id: string, signal?: AbortSignal): Promise<DocumentDetail> {
  return apiRequest<DocumentDetail>("GET", `/api/documents/${encodeURIComponent(id)}`, undefined, signal);
}

export async function deleteDocument(id: string): Promise<void> {
  await apiRequest<undefined>("DELETE", `/api/documents/${encodeURIComponent(id)}`);
}

/** Where the PDF bytes are served, for the viewer and the download link. */
export function documentFileUrl(id: string): string {
  return `/api/documents/${encodeURIComponent(id)}/file`;
}
