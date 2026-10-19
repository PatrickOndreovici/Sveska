# Development commands. Run `make help` for a list.
# Values come from .env (copy .env.example to create it).

-include .env
export

COMPOSE := docker compose
REDOCLY := docker run --rm -v "$(CURDIR)/api:/spec" -w /spec redocly/cli:2.6.0

.DEFAULT_GOAL := help

.PHONY: help
help: ## List available commands
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

.PHONY: setup
setup: ## Create .env if missing and install Go and npm dependencies
	@test -f .env || (cp .env.example .env && echo "created .env from .env.example")
	cd backend && go mod download
	cd frontend && npm ci

# --- Database ---------------------------------------------------------------

.PHONY: db-up
db-up: ## Start PostgreSQL and wait until it accepts connections
	$(COMPOSE) up -d --wait db

.PHONY: db-down
db-down: ## Stop PostgreSQL (data is kept)
	$(COMPOSE) down

.PHONY: db-reset
db-reset: ## Delete the local database volume, then start fresh and migrate
	$(COMPOSE) down -v
	$(MAKE) db-up migrate-up

.PHONY: db-psql
db-psql: ## Open psql inside the database container
	$(COMPOSE) exec db psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	$(COMPOSE) run --rm migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	$(COMPOSE) run --rm migrate down 1

.PHONY: migrate-version
migrate-version: ## Show the current migration version
	$(COMPOSE) run --rm migrate version

.PHONY: migrate-new
migrate-new: ## Create a migration pair: make migrate-new name=create_users
	@test -n "$(name)" || (echo "usage: make migrate-new name=<snake_case_name>" && exit 1)
	$(COMPOSE) run --rm --no-deps migrate create -ext sql -dir /migrations -seq $(name)

.PHONY: sqlc
sqlc: ## Generate Go query code from backend/db/queries
	$(COMPOSE) run --rm sqlc generate

# --- Run --------------------------------------------------------------------

.PHONY: api
api: ## Run the Go API on the host (needs db-up)
	cd backend && go run ./cmd/api

.PHONY: web
web: ## Run the Vite dev server on http://localhost:5180
	cd frontend && npm run dev

# --- Checks -----------------------------------------------------------------

.PHONY: check
check: check-backend check-frontend openapi-lint check-generated ## Run every check except integration tests

.PHONY: check-backend
check-backend: ## gofmt, go vet and go test
	@unformatted="$$(cd backend && gofmt -l .)"; \
	  if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	cd backend && go vet ./...
	cd backend && go test ./...

.PHONY: check-frontend
check-frontend: ## Typecheck, lint and test the frontend
	cd frontend && npm run typecheck
	cd frontend && npm run lint
	cd frontend && npm test

.PHONY: test-db
test-db: ## Create (if missing) and migrate the integration test database
	@$(COMPOSE) exec -T db psql -U $(POSTGRES_USER) -d postgres -tAc \
	  "SELECT 1 FROM pg_database WHERE datname = '$(POSTGRES_TEST_DB)'" | grep -q 1 \
	  || $(COMPOSE) exec -T db createdb -U $(POSTGRES_USER) $(POSTGRES_TEST_DB)
	MIGRATE_DB=$(POSTGRES_TEST_DB) $(COMPOSE) run --rm migrate up

.PHONY: test-integration
test-integration: test-db ## Run Go integration tests against the test database (needs db-up)
	cd backend && go test -tags integration -count=1 -p 1 ./...

.PHONY: gen
gen: sqlc ## Regenerate Go queries and frontend API types
	cd frontend && npm run gen:api

.PHONY: check-generated
check-generated: gen ## Fail if generated code is out of date with its sources
	@git diff --exit-code --stat -- backend/internal/store frontend/src/lib/api/schema.d.ts \
	  || (echo "Generated code was stale and has been regenerated; review and commit it." && exit 1)

.PHONY: openapi-lint
openapi-lint: ## Validate api/openapi.yaml
	$(REDOCLY) lint openapi.yaml
