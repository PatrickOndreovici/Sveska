import { apiRequest } from "./client";
import type {
  NewTextBoxInput,
  Stroke,
  StrokeEnvelope,
  StrokeInput,
  StrokeList,
  TextBox,
  TextBoxEnvelope,
  TextBoxInput,
  TextBoxList,
} from "./types";

/** Limits shared with the API. */
export const MAX_STROKE_POINTS = 1000;
export const MAX_TEXT_BOX_LENGTH = 1000;

export async function listStrokes(documentId: string, signal?: AbortSignal): Promise<Stroke[]> {
  const { strokes } = await apiRequest<StrokeList>(
    "GET",
    `/api/documents/${encodeURIComponent(documentId)}/strokes`,
    undefined,
    signal,
  );
  return strokes;
}

export async function createStroke(documentId: string, input: StrokeInput): Promise<Stroke> {
  const { stroke } = await apiRequest<StrokeEnvelope>(
    "POST",
    `/api/documents/${encodeURIComponent(documentId)}/strokes`,
    input,
  );
  return stroke;
}

export async function deleteStroke(id: string): Promise<void> {
  await apiRequest<undefined>("DELETE", `/api/strokes/${encodeURIComponent(id)}`);
}

export async function listTextBoxes(documentId: string, signal?: AbortSignal): Promise<TextBox[]> {
  const { textBoxes } = await apiRequest<TextBoxList>(
    "GET",
    `/api/documents/${encodeURIComponent(documentId)}/text-boxes`,
    undefined,
    signal,
  );
  return textBoxes;
}

export async function createTextBox(documentId: string, input: NewTextBoxInput): Promise<TextBox> {
  const { textBox } = await apiRequest<TextBoxEnvelope>(
    "POST",
    `/api/documents/${encodeURIComponent(documentId)}/text-boxes`,
    input,
  );
  return textBox;
}

/** Replaces the text, colour and place; the page stays the same. */
export async function updateTextBox(id: string, input: TextBoxInput): Promise<TextBox> {
  const { textBox } = await apiRequest<TextBoxEnvelope>("PUT", `/api/text-boxes/${encodeURIComponent(id)}`, input);
  return textBox;
}

export async function deleteTextBox(id: string): Promise<void> {
  await apiRequest<undefined>("DELETE", `/api/text-boxes/${encodeURIComponent(id)}`);
}
