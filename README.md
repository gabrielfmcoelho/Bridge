# Bridge

Bridge is an internal infrastructure-management platform for a Brazilian
state government org (GovPI / SEAD-PI). It inventories hosts, services, DNS
records, projects, API specs (the "Atlas" module), secrets, external tools
and contacts, with access scoped by a hierarchical org-unit tree
("entidades").

The project is a Go backend paired with a Next.js frontend:

- `internal/`, `cmd/`, `main.go` — the Go API server and CLI (module
  `github.com/gabrielfmcoelho/ssh-config-manager`).
- `frontend/` — the Next.js 16 / React 19 web console, proxying `/api/*` to
  the Go server (see `frontend/next.config.ts`).

## Prerequisites

- Go (see `go.mod` for the version)
- Node.js + npm
- PostgreSQL, reachable via the `SSHCM_DB_DSN` environment variable — the
  backend is Postgres-only
- Docker, running — the backend test suite starts real Postgres instances
  via testcontainers

## Running it

```sh
make dev          # Go API (:8080) + Next.js dev server (:3000) together
make dev-api       # Go API only
make dev-frontend  # Next.js dev server only
make build         # production build of both halves
```

## Tests

```sh
go vet ./...
go test -race -buildvcs=false ./...   # needs Docker running (testcontainers)
```

Frontend has no test runner installed; the few `frontend/src/lib/*.test.ts`
files run individually with Node's built-in runner, e.g.:

```sh
cd frontend && node --test src/lib/entidades.test.ts
```

Typecheck/lint/build the frontend with:

```sh
cd frontend && npx tsc --noEmit && npm run lint && npm run build
```

## Where to go next

- `CLAUDE.md` — architecture, conventions, and the entidades scoping
  contract; the primary reference for working in this codebase.
- `internal/spec/` — design specs for individual subsystems (entidades,
  secrets manager, etc).
