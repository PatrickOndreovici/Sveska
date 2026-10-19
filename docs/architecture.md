# Architecture

Sveska, a language-learning workspace for Serbian.

Status: draft for review. Describes the intended shape of the system; only the
parts marked **built** exist today.

## Goals that shape the design

- Readable, boring code that a new contributor can follow in an afternoon.
- Business rules and authorization live in one place on the backend.
- The HTTP contract is written down (OpenAPI) before it is implemented.
- No AI features and no generated educational content.

## System overview

```
Browser (React SPA, PDF.js)
   │  same-origin /api/* (Vite proxy in development)
   ▼
Go API (net/http + chi)
   │  pgx pool, sqlc-generated queries
   ▼
PostgreSQL ◄── golang-migrate (SQL files)

PDF bytes: file storage behind a small Go interface
(local disk in development, object storage later).
```

A single Go binary (modular monolith) serves the API. The frontend is a static
single-page app. In production both can sit behind one origin so session
cookies stay first-party and CORS is unnecessary.

## Repository structure

```
.
├── api/
│   ├── openapi.yaml            # REST contract (built: /api/health)
│   └── redocly.yaml            # contract lint rules
├── backend/
│   ├── cmd/api/                # main: config, wiring, graceful shutdown (built)
│   ├── db/
│   │   ├── migrations/         # golang-migrate SQL, numbered (built)
│   │   └── queries/            # sqlc input, one file per area (empty)
│   ├── internal/
│   │   ├── config/             # env parsing (built)
│   │   ├── database/           # pgx pool (built)
│   │   ├── httpapi/            # router, middleware, handlers, JSON helpers (built: health)
│   │   ├── store/              # sqlc output, generated, never edited (phase 1)
│   │   ├── access/             # authorization policy functions (phase 1)
│   │   ├── identity/           # users, sessions, passwords (phase 1)
│   │   ├── learning/           # lessons, documents, annotations, vocabulary, quizzes, progress
│   │   ├── teaching/           # links, classes, assignments, submissions, grading, feedback
│   │   └── filestore/          # PDF storage interface + local implementation (phase 2)
│   ├── go.mod
│   └── sqlc.yaml
├── frontend/
│   ├── public/                 # favicon
│   ├── src/
│   │   ├── app/                # router, root layout (built)
│   │   ├── pages/              # route-level components, thin (built: home, 404)
│   │   ├── features/<area>/    # data hooks + feature components (phase 1+)
│   │   ├── components/ui/      # design-system primitives (built: Button, TextLink, Eyebrow)
│   │   ├── components/         # shared non-primitive components (built: ApiStatus)
│   │   ├── lib/api/            # fetch client, response parsing (built: health)
│   │   ├── styles/index.css    # Tailwind + design tokens (built)
│   │   └── test/setup.ts       # Vitest setup (built)
│   └── vite.config.ts
├── docs/                       # this document and its siblings
├── docker-compose.yml          # PostgreSQL + migrate/sqlc tool containers (built)
├── Makefile                    # every routine command (built)
└── .env.example
```

## Backend layering

Each request passes through three layers. Each layer only calls the one below.

1. **`httpapi` handlers** decode the request, read the authenticated actor from
   the context, call one service method, and encode the result or map the error
   to the JSON error envelope. No SQL, no business rules.
2. **Domain services** (`identity`, `learning`, `teaching`) hold the rules:
   validation, state transitions, and authorization checks via `access`. They
   take an explicit `Actor` argument so authorization cannot be forgotten
   silently, and they depend on narrow interfaces over `store.Queries` so unit
   tests can substitute fakes.
3. **`store`** is sqlc-generated, parameterized SQL. Multi-statement operations
   run inside a transaction owned by the service.

Rules:

- **Authorization on the backend, always.** Every service method that reads or
  writes user data checks the actor against the resource. Queries are also
  scoped (`WHERE owner_id = $1`) so a missed check fails closed. The frontend
  hides controls for convenience only.
- **Typed errors.** Services return sentinel errors (`ErrNotFound`,
  `ErrForbidden`, `ErrValidation`). Handlers map them to HTTP status codes in
  one function. A resource the actor may not see returns 404, not 403, to avoid
  leaking existence.
- **No global state** besides the logger. Dependencies are passed in from
  `main`.

## Authentication (phase 1 proposal)

- Email + password. Passwords hashed with argon2id (`golang.org/x/crypto`).
- Server-side sessions: random 32-byte token in an `HttpOnly; Secure;
  SameSite=Lax` cookie; only its SHA-256 hash is stored in `sessions`.
- CSRF: SameSite=Lax plus an `Origin` check on unsafe methods.
- Role (`student` or `teacher`) is chosen at sign-up and stored on the user.

## Frontend

- **Pages** compose features and own routing concerns only.
- **Feature hooks** (`useLessons`, ...) call `lib/api` functions and hold
  loading/error state. **Presentational components** take props and render;
  they do not fetch. `ApiStatus` + `useHealth` is the reference example.
- **`lib/api`** functions are plain async functions that validate the response
  shape before returning typed data. Request and response types will be
  generated from `api/openapi.yaml` once a compatible generator is chosen (see
  dependencies.md).
- **Server state** uses plain hooks at first. If caching and invalidation get
  repetitive across features, adopt TanStack Query in one deliberate step
  rather than growing an in-house cache.

## Design system

- Tokens live in `frontend/src/styles/index.css` under `@theme`. The default
  Tailwind palette is removed so only named tokens can be used.
- Token names are semantic (`paper`, `ink`, `rule`, `accent`, `highlight`).
  Dark mode redefines the same names under `:root[data-theme="dark"]`, so
  components never change for it.
- Theme switching: the user picks Light, Dark or System in the header.
  - `<html data-theme>` always holds the resolved theme, `light` or `dark`.
  - An inline script in `index.html` sets it before first paint, so there is no
    flash of the wrong theme.
  - `ThemeProvider` (`src/lib/theme/`) keeps it current and follows the OS
    while the preference is System. The choice is saved in `localStorage`
    under `sveska.theme`.
  - Tailwind's `dark:` variant is wired to the same attribute, but tokens
    should cover almost every case.
- Palette: **Ink Blue** (decided 2026-10-19). Cool off-white paper, near-black
  ink, hairline rules and one deep blue accent. Serif for reading text and
  headings, sans for interface text. No gradients, glass, or card grids of stats.
- Color roles are fixed:
  - `accent` (blue) is for primary actions and links only.
  - `success` and `danger` (green, red) are for correct and incorrect answers
    and for errors. Never use them decoratively.
  - `highlight` (yellow) is for student highlights only.
  Blue was chosen because it can never be confused with the correct/incorrect
  colors that quizzes depend on.
- Every text token passes WCAG AA (4.5:1) on `paper` and `paper-raised` in
  both light and dark mode. Check new tokens before adding them.
- Brand: the product is called **Sveska** ("notebook"). The mark is an
  exercise-book page with a margin line, in `frontend/src/components/Logo.tsx`
  and `frontend/public/favicon.svg`. Keep the two in sync. Domain and
  trademark availability have not been checked.
- Fonts: Source Serif 4 and IBM Plex Sans, both covering Latin Extended
  (č ć š ž đ) and Cyrillic, including Serbian localized italic forms
  (`lang="sr"` is required on Serbian text for these). Loaded from Google Fonts
  in development; self-host before production.
- Primitives live in `components/ui` and are added when a second caller needs
  them, not in advance.

## PDFs and annotations (phase 2–3 proposal)

- Upload goes through the API, which validates type and size, stores bytes via
  `filestore`, and records metadata (size, SHA-256, page count) in PostgreSQL.
- Downloads are served by the API after an authorization check. No public URLs.
- PDF.js (`pdfjs-dist`) renders pages in the browser with its worker.
- Annotations are anchored by page number plus rectangles in normalized page
  coordinates (0–1), with the selected text stored as a quote. This survives
  zoom changes and lets vocabulary entries cite their source.

## Testing

- Go: standard `testing` and `httptest`. Handlers are tested through the router.
  Services are tested with fake stores. Phase 1 adds integration tests that run
  against the Compose PostgreSQL using a separate test database.
- Frontend: Vitest + React Testing Library, querying by role and accessible
  name. `lib/api` functions are tested with a mocked `fetch`.
- Contract: Redocly lints `api/openapi.yaml`.

## Open questions

1. Can one account be both a student and a teacher, or are roles exclusive?
2. What can a linked teacher see of a student's private lessons, PDFs and
   vocabulary: nothing, only what is submitted, or everything?
3. Interface language: English only, Serbian, or both from the start?
4. Production file storage: local volume or S3-compatible service?
5. Grading scale: points, percentage, or free-form?
