# Local development

## Prerequisites

- Go 1.26+
- Node 22.22+ and npm
- Docker with Compose v2
- make

sqlc, golang-migrate and the OpenAPI linter run in containers; nothing else needs
installing.

## First run

```sh
make setup        # creates .env, downloads Go modules, npm ci
make db-up        # starts PostgreSQL on 127.0.0.1:5433
make migrate-up   # applies migrations
make api          # terminal 1: API on http://localhost:8080
make web          # terminal 2: app on http://localhost:5180
```

Open http://localhost:5180. The footer shows whether the API and database are
reachable.

## Ports

| Service | Port | Why not the default |
|---|---|---|
| PostgreSQL | 5433 | Avoids clashing with a host PostgreSQL on 5432. |
| API | 8080 | |
| Vite | 5180 | 5173 is often taken by other Vite projects. `strictPort` makes a clash fail loudly. |

Vite proxies `/api` to the API, so the browser sees one origin.

## Everyday commands

```sh
make check                      # gofmt, vet, Go tests, typecheck, lint, Vitest, OpenAPI lint, generated-code drift
make test-integration           # Go tests against the lsp_test database (needs db-up)
make gen                        # regenerate Go queries and frontend API types
make migrate-new name=create_users
make migrate-up / migrate-down  # down rolls back one step
make sqlc                       # regenerate backend/internal/store
make db-psql                    # psql shell
make db-reset                   # DELETES local data, recreates and migrates
```

## Notes

- New settings in phase 1: `APP_ORIGIN`, `COOKIE_SECURE`, `POSTGRES_TEST_DB`
  and `TEST_DATABASE_URL`. Copy them from `.env.example` into an existing `.env`.
- `COOKIE_SECURE=false` is for local HTTP only. Any deployed environment must
  leave it unset (the default is `true`).
- PostgreSQL 18 stores data under `/var/lib/postgresql/18`; the Compose volume
  is mounted at `/var/lib/postgresql` accordingly.
