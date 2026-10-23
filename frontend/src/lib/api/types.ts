/** Friendly aliases over the types generated from api/openapi.yaml (see schema.d.ts). */
import type { components } from "./schema";

type Schemas = components["schemas"];

export type User = Schemas["User"];
export type Role = Schemas["Role"];
export type SignUpRequest = Schemas["SignUpRequest"];
export type LogInRequest = Schemas["LogInRequest"];
export type UserEnvelope = Schemas["UserEnvelope"];
export type ErrorEnvelope = Schemas["Error"];
export type VocabularyEntry = Schemas["VocabularyEntry"];
export type VocabularyEntryInput = Schemas["VocabularyEntryInput"];
export type VocabularyEntryEnvelope = Schemas["VocabularyEntryEnvelope"];
export type VocabularyPage = Schemas["VocabularyPage"];
export type VocabularySort = Schemas["VocabularySort"];
export type Lesson = Schemas["Lesson"];
export type LessonInput = Schemas["LessonInput"];
export type LessonEnvelope = Schemas["LessonEnvelope"];
export type LessonList = Schemas["LessonList"];
export type LessonDetail = Schemas["LessonDetail"];
export type LessonDocument = Schemas["Document"];
export type DocumentEnvelope = Schemas["DocumentEnvelope"];
export type DocumentDetail = Schemas["DocumentDetail"];
