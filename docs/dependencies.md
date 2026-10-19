# Dependencies

Every dependency must earn its place. Exact versions are pinned in `go.mod` and
`package-lock.json`.

## Installed

### Backend (Go)

| Module | Why |
|---|---|
| `github.com/go-chi/chi/v5` | Required router. Small, stdlib-compatible `http.Handler`, route groups, middleware (request ID, recoverer). |
| `github.com/jackc/pgx/v5` | Required PostgreSQL driver and pool; sqlc targets it directly. |

### Frontend runtime

| Package | Why |
|---|---|
| `react`, `react-dom` | Required UI library. |
| `react-router` | Required routing. |

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

## Planned, added in the phase that needs them

| Package | Phase | Why |
|---|---|---|
| `golang.org/x/crypto` | 1 | argon2id password hashing. |
| OpenAPI → TypeScript generator | 1 | Keep frontend types in sync with the contract. `openapi-typescript` 7.x declares a TypeScript 5 peer, so compatibility with TypeScript 6 must be verified first. |
| `pdfjs-dist` | 2 | Required PDF rendering. |

## Deliberately not used

- ORMs or query builders: sqlc gives typed, parameterized SQL.
- `testify` and mocking frameworks: standard `testing` with small fakes is enough.
- `godotenv`, `viper`: the Makefile exports `.env`; config is a few env vars.
- `axios`, `clsx`, state-management libraries, UI kits: `fetch`, a 3-line `cx`
  helper, React state, and our own primitives cover current needs.
- Prettier: not added yet. Worth adopting once more than one person writes
  code, to stop formatting debates.
