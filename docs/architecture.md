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
│   ├── openapi.yaml            # REST contract (built: health, auth, /me)
│   └── redocly.yaml            # contract lint rules
├── backend/
│   ├── cmd/api/                # main: config, wiring, graceful shutdown (built)
│   ├── db/
│   │   ├── migrations/         # golang-migrate SQL, numbered (built)
│   │   └── queries/            # sqlc input, one file per area (built: users, sessions)
│   ├── internal/
│   │   ├── config/             # env parsing (built)
│   │   ├── database/           # pgx pool (built)
│   │   ├── httpapi/            # router, middleware, handlers, error mapping (built)
│   │   ├── apperr/             # error kinds services return (built)
│   │   ├── store/              # sqlc output, generated, never edited (built)
│   │   ├── access/             # Actor in context, authorization rules (built)
│   │   ├── identity/           # users, sessions, passwords (built)
│   │   ├── testdb/             # integration-test database helper (built)
│   │   ├── learning/           # lessons, documents, annotations, vocabulary, quizzes, progress
│   │   ├── teaching/           # links, classes, assignments, submissions, grading, feedback
│   │   └── filestore/          # PDF storage on local disk (built in phase 2)
│   ├── go.mod
│   └── sqlc.yaml
├── frontend/
│   ├── public/                 # favicon
│   ├── src/
│   │   ├── app/                # router, root layout (built)
│   │   ├── pages/              # route-level components, thin (built: home, 404)
│   │   ├── features/<area>/    # state, pages and components per feature (built: auth)
│   │   ├── components/ui/      # design-system primitives (built: Button, TextField, FormAlert, ...)
│   │   ├── components/         # shared non-primitive components (built: ApiStatus)
│   │   ├── lib/api/            # fetch client, generated contract types (built)
│   │   ├── styles/index.css    # Tailwind + design tokens (built)
│   │   └── test/               # Vitest setup, fake API, app renderer (built)
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
- **Typed errors.** Services return `*apperr.Error` with a `Kind`
  (`invalid`, `unauthenticated`, `conflict`), a stable `Code` and a
  user-safe `Message`. `writeServiceError` in `httpapi` is the only place
  that maps kinds to status codes (422, 401, 409). Anything else is logged
  and answered with a generic 500. New kinds (`forbidden`, `not_found`) are
  added with the first feature that needs them. A resource the actor may not
  see will return 404, not 403, to avoid leaking existence.
- **No global state** besides the logger. Dependencies are passed in from
  `main`.

## Authentication (built in phase 1)

- **Accounts.** Email + password. A role (`student` or `teacher`) is chosen at
  sign-up and cannot change. Emails are unique case-insensitively (`citext`).
- **Passwords.** argon2id with OWASP parameters (19 MiB, 2 iterations),
  stored in PHC format so parameters can be raised later. 10 to 128
  characters. Login with an unknown email still runs a hash check, so timing
  and responses do not reveal which emails have accounts.
- **Sessions.** A random 32-byte token in the `sveska_session` cookie
  (`HttpOnly`, `SameSite=Lax`, `Secure` unless `COOKIE_SECURE=false`, 30
  days). Only its SHA-256 is stored. Logging in or signing up while a session
  cookie is present ends the old session. Deleting a user deletes their
  sessions.
- **CSRF.** State-changing requests with an `Origin` header other than
  `APP_ORIGIN` get 403. SameSite=Lax is the second layer.
- **Middleware.** `loadSession` turns a valid cookie into an `access.Actor` in
  the request context and clears stale cookies. `requireActor` guards every
  protected route with 401. Handlers and services read the actor through
  `access.RequireActor`, or `access.RequireStudent` for a student's own
  material (lessons, documents, vocabulary), which answers a teacher with 403
  `students_only`. The frontend's `RequireRole` route guard mirrors it.
- **Headers.** Every API response sets `nosniff`, `DENY` framing, a
  `default-src 'none'` CSP, `no-referrer` and `no-store`.
- **Profile.** `PUT /api/me` changes the display name; email and role are
  fixed. A profile picture is uploaded to `/api/me/avatar` and turned into
  the largest centred square, upright as the photo's EXIF orientation says,
  at most 256×256, re-encoded as JPEG (which drops other metadata, such as
  the place a photo was taken). The user's `avatarUrl` carries the picture's
  version, so the browser may keep it for good. These endpoints act on the
  logged-in user only (`identity.ProfileService`).
- **Not yet built.** Rate limiting on log-in and sign-up, password reset,
  email verification and "log out everywhere" (phase 7 or when needed).

## Frontend

- **Pages** compose features and own routing concerns only.
- **Feature hooks** (`useLessons`, ...) call `lib/api` functions and hold
  loading/error state. **Presentational components** take props and render;
  they do not fetch. `ApiStatus` + `useHealth` is the reference example.
- **`lib/api`** holds plain async functions over one `apiRequest` helper,
  which turns the error envelope into an `ApiError` (`status`, `code`,
  `message`, `fields`). Request and response types are generated from
  `api/openapi.yaml` into `schema.d.ts` with `npm run gen:api` and must not be
  edited. `make check` fails if they drift from the contract. Success bodies
  are trusted to match the contract, since the backend is tested against it.
- **Auth state** lives in `AuthProvider`, which mirrors `GET /api/me`. The
  session cookie is HttpOnly and never touched by JavaScript.
  `RequireAuth` and `GuestOnly` route wrappers redirect based on that state,
  and `navigationByRole` lists each role's navigation.
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

## PDFs and annotations (built in phases 2 and 3)

- Upload goes through the API, which checks that the file starts with `%PDF-`,
  stops reading past 20 MB, stores bytes via `filestore`, and records metadata
  (size, SHA-256) in PostgreSQL. The file is written first and deleted again
  if the row cannot be inserted.
- Downloads are served by the API after an authorization check. No public URLs.
- PDF.js (`pdfjs-dist`) renders pages in the browser with its worker, and lays
  the page's text over the canvas as invisible, selectable spans (its text
  layer; the CSS for it is copied into `lib/pdf/textLayer.css`).
- Annotations are highlights on selected text, each with an optional note.
- Students also write and draw on pages, as on a worksheet: strokes drawn
  with a pen (`ink_strokes`) and text typed where they click (`text_boxes`),
  in three colours. Both use the same 0–1 page coordinates as highlights.
  The drawing is laid over the PDF's text, and lets pointers through to it
  while the select tool is in use.
- Drawing changes show at once and are saved in the background, one request
  after another (`useDrawings`). Each change can be undone, which sends the
  opposite request; a failed save reloads what the API has.
- An annotation is anchored by page number plus rectangles in normalized page
  coordinates (0–1), with the selected text stored as a quote. This survives
  zoom changes and lets vocabulary entries cite their source.
  `lib/pdf/selection.ts` turns a browser selection into those rectangles,
  merging the pieces of each line.
- Saving a selected word creates its highlight first, then the vocabulary
  entry citing it (`sourceAnnotationId`). Deleting the highlight keeps the
  word and clears only the link.

## Dashboard

- A student's home page is their dashboard: totals, a streak, a calendar of
  the last twelve weeks and where they left off. `GET /api/dashboard` builds
  it from the study tables (`learning.DashboardService`) in the time zone the
  browser sends, so days start at the student's midnight.
- A day counts as active when the student added a word, highlighted text,
  wrote on a page or answered a flashcard. The streak counts active days in a row up to today, or up
  to yesterday while nothing is done yet today.
- The calendar is a picture only; the summary line and a table carry the
  same numbers. Its colours are one blue ramp, checked as an ordered ramp
  against the card in both themes (`--color-activity-*`).

## Flashcards

- Every word in a student's vocabulary is a flashcard, Serbian side first
  (`sr_en`). Once that card has been remembered twice in a row, the word
  gets a second card, English side first (`en_sr`), answered by typing the
  Serbian word; each card has its own schedule. `GET /api/review-queue`
  returns today's cards: cards due, longest overdue first, then up to 20
  new English-first cards a day, then up to 20 new words a day, oldest
  first, at most 50 at a time. `POST /api/vocabulary/{id}/reviews` records
  an answer to one of a word's cards (again, hard, good, easy) and
  schedules that card's next review (`learning.ReviewService`).
- A word shows one card a day, so that one does not give away the other's
  answer: the queue holds at most one card per word, and a card waits for
  tomorrow while the word's other card was reviewed today. A card answered
  "again" still comes back the same day.
- The schedule is a simplified SM-2, as Anki uses it, in whole days: a word
  not yet learned comes back in 1, 2 or 4 days; after that each answer
  stretches the interval by 1.2 (hard), the word's ease (good, 2.5 to start)
  or the ease and 1.3 (easy), up to a year. "Again" brings the word back the
  same day and starts it over. The API sends each card's next intervals, so
  the buttons show them without the frontend knowing the rules.
- Days are dates in the student's time zone, as on the dashboard. The
  schedule and the logged answer are saved in one statement, and only if the
  schedule is still the one that was read, so an answer sent twice (from two
  tabs) counts once; the second gets 409 `review_conflict`.
- The review page (`/review`) shows the cards of one queue in order and asks
  for the queue again once they are answered, which brings back the cards
  answered "again". Space shows the answer and 1–4 answer it. On an
  English-first card the page checks the typed word itself
  (`features/review/answer.ts`; the word is in the queue anyway) and
  suggests a rating, which Enter takes: good when right, hard when only a
  č, ć, š, ž or đ is off, again when wrong. Case, punctuation and a part in
  brackets do not count.
- Buttons for č ć š ž đ (`components/SerbianLetters.tsx`) sit by the typed
  answer and in the vocabulary forms, where they type into the Serbian
  field used last. They stay out of the tab order.

## Testing

- Go: standard `testing` and `httptest`. Handlers are tested through the router
  with fake services, and services with fake stores. Integration tests
  (`//go:build integration`, `make test-integration`) run the real router,
  services and queries against a separate `lsp_test` database that is
  truncated before each test.
- Frontend: Vitest + React Testing Library, querying by role and accessible
  name. `lib/api` functions are tested with a mocked `fetch`.
- Contract: Redocly lints `api/openapi.yaml`.

## Open questions

1. ~~Can one account be both a student and a teacher?~~ Decided for phase 1:
   roles are exclusive and fixed at sign-up. Revisit if teachers ask to study.
2. What can a linked teacher see of a student's private lessons, PDFs and
   vocabulary: nothing, only what is submitted, or everything?
3. Interface language: English only, Serbian, or both from the start?
4. Production file storage: local volume or S3-compatible service?
5. Grading scale: points, percentage, or free-form?
