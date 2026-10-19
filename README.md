# Sveska

Sveska ("notebook" in Serbian) is a workspace for students learning Serbian: lessons, PDFs, annotations,
vocabulary, quizzes and progress, with optional teachers who run classes,
assignments, grading and feedback. No AI features.

**Status:** phase 0, project foundation only. No product features yet.

- Local setup: [docs/development.md](docs/development.md)
- Architecture and repository layout: [docs/architecture.md](docs/architecture.md)
- Data model draft: [docs/data-model.md](docs/data-model.md)
- Implementation plan: [docs/implementation-plan.md](docs/implementation-plan.md)
- Dependencies: [docs/dependencies.md](docs/dependencies.md)
- API contract: [api/openapi.yaml](api/openapi.yaml)

```sh
make setup && make db-up && make migrate-up
make api   # http://localhost:8080/api/health
make web   # http://localhost:5180
make check
```
