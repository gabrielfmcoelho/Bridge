# Entidades — hierarchical org units & per-asset visibility

Status: **Implemented** (branch `feat/entidades`, 2026-08-19). Companion to
`secrets-manager.md` (D7 two-axis model) — this spec adds the *who-sees-what* axis that
applies to every module.

## 1. Model

- **Entidade** = org unit (órgão/setor) in a **strict tree** (`parent_id`, siblings =
  "parallel"). Seeded: `GovPI` → `ETIPI`, `SEAD-PI`{`NTGD`,`SGA`,`SGP`,`SGI`},
  `Parceiros`{`Trin`,`Syslae`,`Vobys`}.
- A **user belongs to N entidades**, at most one **primary** (default creator for new assets).
- Every **root asset** is created by **one** entidade (`creator`), may be the responsibility
  of **N** entidades (`responsible`), and may be flagged **`global`**.
- **Visible set** of a user = union over their entidades of (entidade + all descendants):
  *above sees all that below sees*.
- Asset **visible iff** `admin` OR a grant row is `global` OR a grant's entidade ∈ visible set.
  **No grant rows ⇒ admin-only** (unassigned; triaged in Settings → Entidades).
- **See ⇒ can act**; role middleware (viewer/editor/admin) still gates the verb. Invisible
  rows are **404** on get/update/delete/restore (never 403 — existence isn't leaked).
- Creator rule: a non-admin may only pick a creator inside their visible set (403 otherwise)
  and may not produce an asset invisible to themselves (no creator, not global, no
  responsibles ⇒ 400). Responsibles are unrestricted (widening visibility is the owner's call,
  like `global`).

## 2. Asset taxonomy

| Root assets (own grants, `asset_type`) | Dependents (inherit from parent) |
|---|---|
| `host`, `dns`, `service`, `project`, `contact`, `tool` (external_tools), `ssh_key`, `api_catalog`, `secret` (**only** `scope='avulso' AND visibility='shared'`) | issues (→ `entity_type`/`entity_id`), releases (→ project; NULL project = unassigned), secrets with a parent (→ host/service/project/tool; personal stay owner-only), host sub-resources (alerts, chamados, scans, remote users, op logs, grafana, coolify, GLPI-by-host, SSH ops, AI), project sub-resources (wiki collection, GLPI tickets, gitlab links, AI analysis), dashboard counts, graph, trash |

Scan-discovered services copy their host's grants; tools synced from a service copy the
service's grants. Wiki "common collections" and non-project GLPI forms stay org-wide.

## 3. Schema (migrations v82, v83)

```
entidades(id, name, slug UNIQUE, parent_id → entidades RESTRICT, description, created_at, updated_at)
user_entidades(user_id → users CASCADE, entidade_id → entidades RESTRICT, is_primary)  -- ≤1 primary/user
asset_entidades(id, asset_type CHECK(9 types), asset_id, entidade_id → entidades RESTRICT NULL,
                relation CHECK('creator'|'responsible'|'global'), CHECK((relation='global') = (entidade_id IS NULL)))
  UNIQUE(asset_type, asset_id) WHERE relation='creator' · UNIQUE(...) WHERE relation='global'
  UNIQUE(asset_type, asset_id, entidade_id) WHERE relation='responsible'
```
Named `asset_type/asset_id` (not `entity_*`) to avoid colliding with "entidade". Backfill:
every host and host-linked service → creator **ETIPI** ("all VMs are created by ETIPI");
everything else unassigned. v83 drops the legacy free-text `host_entidades`.
Entidade FKs are RESTRICT: an entidade with children, members or grants cannot be deleted (409).

## 4. Enforcement (where the rule lives)

- `auth.RequireAuth` loads the caller's `store.Scope{Admin, EntidadeIDs, PrimaryEntidadeID}`
  once per request (recursive CTE over `user_entidades`+`entidades`) into `context.Context`.
- `store.VisibleExpr(ctx, assetType, idExpr)` / `VisibleExprDyn(ctx, typeExpr, idExpr)` return the
  SQL predicate (`EXISTS (… ae.relation='global' OR ae.entidade_id = ANY($ids))`) or `TRUE`
  for admin/system/**absent** scope. Every repo appends it to its `where` builder for
  List/Count/Get/Update/Delete/Restore. `store.CanSee(ctx, q, type, id)` for id-only handlers.
- Absent scope = unscoped (background jobs, startup migrations, tests).
  `// ponytail: flip to deny-by-default in store/scope.go if a repo is ever reached without RequireAuth.`
- `HostRepo.GetBySlug` / `ProjectRepo.Get` being scoped covers ~40 sub-resource handlers.
- Grants are written by `store.AssetEntidadeRepo.Replace` right after the root insert;
  `store.ResolveGrants` applies the creator rule (`api.resolveGrants` maps 403/400).

## 5. API

| Route | Role |
|---|---|
| `GET /api/entidades`, `GET /api/entidades/{id}` | auth |
| `POST /api/entidades`, `PUT/DELETE /api/entidades/{id}` (409 in use) | admin |
| `GET /api/entidades/unassigned?asset_type=&page=&per_page=` | admin |
| `POST /api/entidades/bulk-assign` `{asset_type, asset_ids[], creator_entidade_id?, responsible_entidade_ids?, is_global?}` | admin |
| `GET/PUT /api/assets/{type}/{id}/entidades` (404 if invisible) | auth / editor |
| `POST/PUT /api/users…` `+ entidade_ids[], primary_entidade_id`; `GET /api/users` and `GET /api/auth/me` return `entidades` | admin / auth |
| root-asset create/update bodies embed `creator_entidade_id`, `responsible_entidade_ids`, `is_global` (pointer semantics: absent = untouched); details return `entidades` | existing |
| `GET /api/hosts?entidade_id=` (replaces `entidade_responsavel`); bulk import items may carry grants | — |
| `GET /api/releases*` moved public → auth | — |

## 6. Frontend

`components/entidades/EntidadeScopeFields` (creator select from the user's subtree, responsibles
checklist, global toggle; `loadFrom` fetches current grants for edit forms) embedded in every
root-asset form; Settings → **Entidades** tab (tree CRUD + unassigned triage/bulk-assign);
user forms pick memberships + primary; hosts filter by entidade. `lib/entidades.ts` holds the
tree helpers (`node --test src/lib/entidades.test.ts`).

## 7. Deploy notes

- Ship is atomic (one branch). Right after deploy **non-admins see only what admins assign**:
  hosts/host-services are already ETIPI-created, everything else is unassigned. Assign users
  to entidades (Settings → Users) and triage (Settings → Entidades) first.
- `make dev` with the repo `.env` migrates the DSN it points at (prod) — run against a throwaway
  Postgres (`SSHCM_DB_DSN=…` overrides `.env`).
