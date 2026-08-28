<claude-mem-context>
# Recent Activity

### Feb 26, 2026

| ID | Time | T | Title | Read |
|----|------|---|-------|------|
| #680 | 9:17 AM | ✅ | Reinstalled sshcm from source | ~253 |
</claude-mem-context>

# Bridge

Bridge is an internal infrastructure-management platform for a Brazilian
state government org (GovPI / SEAD-PI). It inventories hosts, services, DNS
records, projects, API specs (the "Atlas" module), secrets, external tools
and contacts. Access is scoped by a hierarchical org-unit tree called
**entidades**.

## Commands

| Task | Command |
|---|---|
| Backend vet | `go vet ./...` |
| Backend tests | `go test -race -buildvcs=false ./...` — starts real Postgres via testcontainers, so **Docker must be running**. No `test` target exists in the Makefile. |
| Run both halves | `make dev` (or `make dev-api` / `make dev-frontend` individually, `make build`) |
| Frontend typecheck | `cd frontend && npx tsc --noEmit` — no `typecheck` script exists |
| Frontend lint | `cd frontend && npm run lint` |
| Frontend build | `cd frontend && npm run build` |
| Frontend dev | `cd frontend && npm run dev` |
| Frontend unit tests | No runner is installed. Run the three `src/lib/*.test.ts` files individually, e.g. `node --test src/lib/entidades.test.ts` |

## Architecture

Layering: `internal/api` (transport, thin handlers) → `internal/service`
(domain logic) → `internal/store` (repos; own all SQL).

- The service layer exists **only** for host, service, project and dns
  today. Every other entity is repo + thin handler — that's acceptable, not
  a gap to "fix" opportunistically.
- `internal/models/issue.go` is a known pre-refactor leftover that puts raw
  SQL inside the model package. Don't copy that pattern in new code.
- Handlers must not contain raw SQL.

**Routing:** each handler struct has a `registerRoutes(rr routeRegistrar)`
method. `routeRegistrar` (`internal/api/routes.go`) exposes `.public`,
`.auth`, `.role(role)`, `.perm(permission)`. The DI container is
`internal/api/app.go` (`App`, `newApp`) plus `internal/api/deps.go` (`Deps`).

**RBAC:** three roles on `users.role` — `viewer < editor < admin`
(`hasMinRole` in `internal/auth/middleware.go`) — plus a
`permissions`/`role_permissions` table for finer-grained codes like
`hosts.view`, `ssh.operate`, `ai.use`.

## The entidades contract

This is the most important section — read it before adding any new scoped
asset type. Full spec: `internal/spec/entidades.md`.

**Visibility rule:** a user's visible set is every entidade they belong to
**plus all descendants** ("above sees all that below sees"). An asset is
visible iff the caller is admin, or a grant row is `global`, or a grant's
entidade is in the visible set. **No grant rows means admin-only.** Grants
are `(asset_type, asset_id, entidade_id, relation)` with
`relation ∈ {creator, responsible, global}`.

**Adding a new scoped asset type must:**

1. Register in `assetRegistry` (`internal/store/scope.go`, ~line 37).
2. Widen the `asset_type` CHECK constraint on `asset_entidades`
   (`internal/database/migrations_postgres.go`, ~line 1314) with a new
   migration, using the existing `DROP CONSTRAINT` / `ADD CONSTRAINT` idiom.
3. Filter every read with `store.VisibleExpr` / `VisibleExprDyn`.
4. Gate every write through `store.ResolveGrants`
   (`internal/store/asset_entidade.go`).
5. Return **404, never 403**, when an asset is invisible — existence is not
   leaked.
6. Embed `<EntidadeScopeFields>` (`frontend/src/components/entidades/`) in
   its form and spread `...grants` into the payload.

## Database & migrations

Postgres only (`internal/database/dialect.go`); `SSHCM_DB_DSN` is required.
Migrations are Go string literals appended to the `migrationsPostgres` slice
in `internal/database/migrations_postgres.go`, one `// Version N` comment
per element, currently at **v83**. Use idempotent idioms:
`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`,
`INSERT ... ON CONFLICT DO NOTHING`.

A number of comments in that file still say "see migrations_sqlite.go" —
**that file no longer exists** (the project went Postgres-only); those
references are dead, ignore them.

## Conventions

- Portuguese domain nouns stay Portuguese in identifiers and schema:
  `Entidade`, `Responsavel`, `Avulso`, `Chamado`, `Situacao`. Do not
  "helpfully" translate them.
- Every new user-facing string ships in **both**
  `frontend/src/messages/en.json` and `pt-BR.json`. Parity is currently
  exact — keep it that way. Default locale is pt-BR.
- Frontend forms are hand-rolled: `useState` per field, manual validation,
  `useMutation`, `<FormError>`. `react-hook-form`, `zod` and `next-intl` are
  in `package.json` but are **100% unused** in `frontend/src` — do not start
  using them.
- There is no toast component. Use `StatusAlert` for banners, `FormError`
  inline.
- Styling is Tailwind v4 with **no config file**; design tokens are CSS
  custom properties in `frontend/src/app/globals.css`. Never hardcode a hex
  value.
- List endpoints return `{data, meta:{page,per_page,total}}`
  (`internal/api/list.go`), unwrapped client-side by `getList` /
  `getListPaginated` in `frontend/src/lib/api.ts`.

## Where to read more

- `internal/spec/entidades.md` — entidades / visibility spec
- `internal/spec/secrets-manager.md` — secrets model spec
- `frontend/README.md` — frontend overview
- `frontend/src/DESIGN_SYSTEM.md` — design tokens and UI rules
- `Plans.md` — historical status log in mixed Japanese/English; useful for
  archaeology, not a source of current truth