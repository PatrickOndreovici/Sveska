# Dependencies

Every dependency must earn its place. Exact versions are pinned in `go.mod` and
`package-lock.json`.

## Installed

### Backend (Go)

| Module | Why |
|---|---|
| `github.com/go-chi/chi/v5` | Required router. Small, stdlib-compatible `http.Handler`, route groups, middleware (request ID, recoverer). |
| `github.com/jackc/pgx/v5` | Required PostgreSQL driver and pool; sqlc targets it directly. |
| `golang.org/x/crypto` | argon2id password hashing (Go team's extended standard library). |

### Frontend runtime

| Package | Why |
|---|---|
| `react`, `react-dom` | Required UI library. |
| `react-router` | Required routing. |
| `pdfjs-dist` | Required PDF rendering. Loaded on first use, with its worker, so it stays out of the main bundle. Only `src/lib/pdf/pdf.ts` imports it. |

### Frontend development

| Package | Why |
|---|---|
| `vite`, `@vitejs/plugin-react` | Required build tool and dev server. |
| `typescript` 6.0 | Strict type checking. Held at 6.0 because `typescript-eslint` does not yet support 7.x. |
| `tailwindcss`, `@tailwindcss/vite` | Required styling; tokens via `@theme`. |
| `vitest`, `jsdom` | Required test runner and DOM environment. |
| `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event` | Required component testing; accessible queries, DOM matchers, realistic input events. |
| `eslint`, `@eslint/js`, `typescript-eslint`, `eslint-plugin-react-hooks`, `globals` | Catches bugs the compiler cannot: unsafe `any`, floating promises, broken hook dependencies. |
| `@types/react`, `@types/react-dom`, `@types/node` | Type definitions. |

### Tool containers (not project dependencies)

| Image | Why |
|---|---|
| `postgres:18-alpine` | Database. |
| `migrate/migrate:v4.19.0` | Runs migrations without installing the CLI on the host. |
| `sqlc/sqlc:1.30.0` | Generates query code without installing the CLI on the host. |
| `redocly/cli:2.6.0` | Lints the OpenAPI contract. |

### Pinned command-line tools (run with npx, not installed)

| Tool | Why |
|---|---|
| `openapi-typescript@7.13.0` | Generates `frontend/src/lib/api/schema.d.ts` from the contract. Run through `npx` because it declares a TypeScript 5 peer while the project uses TypeScript 6; npx gives it its own copy. |

## Planned, added in the phase that needs them

None at the moment. A later phase that needs a package lists it here first.

## Deliberately not used

- ORMs or query builders: sqlc gives typed, parameterized SQL.
- `testify` and mocking frameworks: standard `testing` with small fakes is enough.
- `godotenv`, `viper`: the Makefile exports `.env`; config is a few env vars.
- `axios`, `clsx`, state-management libraries, UI kits: `fetch`, a 3-line `cx`
  helper, React state, and our own primitives cover current needs.
- Prettier: not added yet. Worth adopting once more than one person writes
  code, to stop formatting debates.
