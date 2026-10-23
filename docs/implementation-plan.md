# Implementation plan

Each phase ends with its features working end to end, tested, and documented in
`api/openapi.yaml`. No phase leaves stubbed endpoints or placeholder screens.

## Phase 0 — Foundation (done)

Configuration, Docker Compose PostgreSQL, migrations tooling, sqlc config, Go API
with `/api/health`, React shell with routing and 404, design tokens (Ink Blue,
light and dark with a Light/Dark/System switch), Sveska logo and favicon, UI primitives, checks in the Makefile.

## Phase 1 — Identity and authorization skeleton (done)

- Migrations: `users`, `sessions`.
- First sqlc queries; confirm `make sqlc` generates `internal/store`.
- Sign-up with role, log in, log out, `GET /api/me`.
- Session middleware, `Actor` in context, `access` package, error mapping.
- Integration test harness against a dedicated PostgreSQL test database.
- Decide frontend type generation from OpenAPI. Decided: openapi-typescript
  run as a pinned `npx` tool, output committed, drift checked by `make check`.
- Frontend: auth pages, protected route wrapper, role-aware navigation.
- Exit: a student and a teacher can register and log in; unauthenticated
  requests to protected endpoints return 401; tests cover each rule.

## Phase 2 — Lessons and PDFs (done)

- `lessons`, `documents`; `filestore` with local disk.
- Upload with size and type limits, list, download through the API.
- PDF.js viewer with page navigation and zoom.
- Lessons, documents and vocabulary are for students only, for now.

## Phase 3 — Annotations and vocabulary (done)

- Highlights and notes on PDF pages; vocabulary capture from a selection.
  Decided: highlights on selected text with an optional note.
- Writing and drawing on pages for exercises: a pen for circling, text
  typed where the student clicks for filling in, an eraser and undo.
- Vocabulary list with search and editing (built ahead of phase 2). Words
  saved from a PDF link back to their page.
- Settings for both roles: display name and profile picture. A dashboard
  on a student's home page: words, lessons and PDFs, highlights and notes,
  activity by day with a streak, and the PDFs to pick up again.

## Phase 4 — Quizzes and progress

- Quiz authoring, attempts, automatic scoring for multiple choice, manual
  marking for short answers.
- Lesson progress and a progress overview derived from stored data.

## Phase 5 — Teacher linking and classes

- Student-initiated teacher invitations, acceptance, links.
- Classes and memberships.

## Phase 6 — Assignments, grading, feedback

- Assignment drafting and publishing with optional document or quiz.
- Submissions, grading, feedback threads, returned work.

## Phase 7 — Hardening and release preparation

- Rate limiting on auth endpoints, security headers, audit of authorization tests.
- Rate limiting on log-in and sign-up, accessibility pass, self-hosted fonts.
- Production Dockerfiles and CI running `make check`.
