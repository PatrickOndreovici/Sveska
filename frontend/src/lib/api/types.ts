/** Friendly aliases over the types generated from api/openapi.yaml (see schema.d.ts). */
import type { components } from "./schema";

type Schemas = components["schemas"];

export type User = Schemas["User"];
export type Role = Schemas["Role"];
export type SignUpRequest = Schemas["SignUpRequest"];
export type LogInRequest = Schemas["LogInRequest"];
export type UserEnvelope = Schemas["UserEnvelope"];
export type ErrorEnvelope = Schemas["Error"];
