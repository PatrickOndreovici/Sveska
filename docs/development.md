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
make check                      # gofmt, vet, Go tests, typecheck, lint, Vitest, OpenAPI lint
make migrate-new name=create_users
make migrate-up / migrate-down  # down rolls back one step
make sqlc                       # regenerate backend/internal/store
make db-psql                    # psql shell
make db-reset                   # DELETES local data, recreates and migrates
```

## Notes

- `make sqlc` fails until the first query file exists in
  `backend/db/queries`. This is expected in phase 0.
- PostgreSQL 18 stores data under `/var/lib/postgresql/18`; the Compose volume
  is mounted at `/var/lib/postgresql` accordingly.
