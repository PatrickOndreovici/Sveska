import { apiRequest } from "./client";
import type { Annotation, AnnotationEnvelope, AnnotationInput, AnnotationList } from "./types";

/** The highlighted text and the note are limited like the API's. */
export const MAX_QUOTE_LENGTH = 2000;
export const MAX_NOTE_LENGTH = 5000;

export async function listAnnotations(documentId: string, signal?: AbortSignal): Promise<Annotation[]> {
  const { annotations } = await apiRequest<AnnotationList>(
    "GET",
    `/api/documents/${encodeURIComponent(documentId)}/annotations`,
    undefined,
    signal,
  );
  return annotations;
}

export async function createAnnotation(documentId: string, input: AnnotationInput): Promise<Annotation> {
  const { annotation } = await apiRequest<AnnotationEnvelope>(
    "POST",
    `/api/documents/${encodeURIComponent(documentId)}/annotations`,
    input,
  );
  return annotation;
}

/** Replaces the note; the highlighted text cannot change. */
export async function updateAnnotationNote(id: string, note: string): Promise<Annotation> {
  const { annotation } = await apiRequest<AnnotationEnvelope>("PUT", `/api/annotations/${encodeURIComponent(id)}`, {
    note,
  });
  return annotation;
}

export async function deleteAnnotation(id: string): Promise<void> {
  await apiRequest<undefined>("DELETE", `/api/annotations/${encodeURIComponent(id)}`);
}
