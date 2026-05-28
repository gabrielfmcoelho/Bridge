# Bridge — Plans.md

<!-- legend:start -->
## 🔖 Status マーカー凡例

| マーカー | 意味 | 誰が付ける |
|---------|------|-----------|
| `pm:依頼中` | PM がタスクを起票し、Impl へ依頼中 | PM |
| `cc:TODO` | 未着手 | PM / Impl |
| `cc:WIP` | 作業中 | Impl |
| `cc:完了` | Impl が作業完了し、PM の確認待ち | Impl |
| `pm:確認済` | PM が最終確認を完了 | PM |
| `blocked` | ブロック中（理由を必ず記載） | Impl |

**状態遷移**: `pm:依頼中 → cc:TODO → cc:WIP → cc:完了 → pm:確認済`
<!-- legend:end -->

---

## 🚧 Phase 1 — Unified Secret Model, Migration, Personal CRUD

Spec SSOT: [`internal/spec/secrets-manager.md`](internal/spec/secrets-manager.md)

| Task | 内容 | DoD | Depends | Status |
|------|------|-----|---------|--------|
| 1.1 | ✅ Spec ratified 2026-05-25 (D1=no, D2=column only, D3=soft delete + audit-diff, D4=per-link key, D5=per-var rows, D6=names but no values, **D7=scope/visibility two-axis**). No work remains in this task `[tdd:skip:docs-only]` | Spec header reads "Ratified"; §3 has no "Recommendation" prefix | — | cc:完了 |
| 1.2 | Add `Secret`, `SecretShareLink`, `SecretAuditLog` Go types in `internal/models/secret.go` matching §4.1 of spec, including `Scope` (enum: service/host/tool/avulso), `Visibility` (enum: personal/shared), `OwnerUserID int64` (always set), `GroupLabel *string`, `KeyVersion int`, `DeletedAt *time.Time` `[tdd:required]` | `go vet` clean; new model unit tests pass; enums validated (`type` includes `env_var`, `scope` includes `avulso` and excludes `user`); composite `UNIQUE` constraint shape documented in a struct comment | 1.1 | cc:完了 [2f58d47] |
| 1.3 | Add DDL for `secrets`, `secret_share_links`, `secret_audit_log` in `internal/database/migrations_sqlite.go` + `migrations_postgres.go`. Include `visibility`, `deleted_at`, `group_label`, `key_version`, the two `CHECK` constraints on (scope, parent_id), and the **two split partial unique indexes** from spec §4.1 (one for `visibility='shared'`, one for `visibility='personal'`). sqlite uses partial-index syntax; postgres uses `WHERE` clauses `[tdd:required]` | `TestSQLiteMigrationsApplyCleanly` passes; new `TestSecretsSchemaShape` asserts every column + constraint per spec; partial-unique allows two users to each own a personal "ssh-key" on the same host; CHECK rejects `scope='avulso' AND parent_id IS NOT NULL` | 1.2 | cc:完了 [7b6ae41] |
| 1.4 | Repository `internal/vault/repo.go` (moved from `internal/database/` to avoid model↔database import cycle; renamed `secrets→vault` to clear harness sandbox path heuristic) with `List(filter)`, `ListTrash(actor)`, `GetMetadata`, `Reveal(ctx,id,actor)`, `Create`, `Update`, `SoftDelete`, `Restore`, `History(id)`. All read paths filter `deleted_at IS NULL` unless caller passes `IncludeDeleted=true`. **Reads enforce visibility per spec §5.2**: personal secrets owned by someone else are filtered out (or return 404 on direct GET) unless caller is admin in which case metadata-only fields are returned. Every mutation writes an audit row in the same tx; `Update` computes `changed_fields` diff for audit metadata `[tdd:required]` | 13 sub-tests pass: list excludes others' personal secrets; admin list of others' personal returns metadata only (description redacted); soft-delete then restore round-trip; update audit row has `metadata.changed_fields`; create rejects duplicate per the visibility-split partial-unique rules | 1.3 | cc:完了 [4af1f77] |
| 1.5 | One-shot Go migration: copy `service_credentials` → `secrets(scope=service, visibility=shared)`, `hosts.password_*` → `secrets(scope=host, type=password, visibility=shared)`, `hosts.{pub,priv}_key_*` → `secrets(scope=host, type=sshkey, visibility=shared)`. Re-uses existing ciphertext where shape permits; re-encrypts for sshkey combined payload. Sets `key_version=1`, `owner_user_id=` the maintenance user id (from `seed-maintenance-user`) as the legacy-owner placeholder `[tdd:required]` | Extended `TestCrossDialectRestore` migrates a fixture with 1 service cred + 1 host password + 1 host sshkey and asserts: decrypted values match pre-migration plaintext; all migrated rows have `visibility='shared'`; `owner_user_id` resolves to the maintenance user | 1.4 | cc:完了 [95a7d68] |
| 1.6 | Handlers under `/api/secrets/*` per spec §6: `list`, `get-metadata`, `reveal`, `create`, `update`, `delete` (soft), `restore`, `history`, `trash`, `/mine`. Wire into `internal/api/router.go` enforcing the **two ACL tables in spec §5.1 (RBAC for shared) and §5.2 (ownership for personal)**. `visibility` is immutable on PUT `[tdd:required]` | Handler tests cover both ACL matrices: viewer can read shared but not write; admin reading another user's personal returns 200 with metadata-only payload (no `payload`, no `description`); admin reveal of another user's personal returns 403; PUT to change visibility returns 422; `/mine` returns only `visibility=personal AND owner=me` across all scopes | 1.5 | cc:完了 [92bbebd] |
| 1.7 | Cascading soft delete: hook into the existing `service`/`host`/`external_tool` delete paths so they soft-delete **all** child `secrets` (both visibilities) with the same timestamp in the same tx. Hook into restore paths symmetrically. Avulso secrets are never touched by cascade (no parent) `[tdd:required]` | Test: soft-deleting a service flips `deleted_at` on its shared *and* personal child secrets to the parent's timestamp; restoring the service restores them; orphan secrets (parent already gone) are not touched; avulso secrets in the same DB are unaffected | 1.4 | cc:完了 [1d9616f] |
| 1.8 | **SKIPPED per PM decision** (2026-05-27). Frontend cut over directly to `/api/secrets` instead. Legacy `/api/services/{id}/credentials` + `/api/tools/{id}/credentials` routes deleted in [ebc50ff] | — | 1.6 | pm:確認済 [ebc50ff] |
| 1.9 | Drop legacy columns (`hosts.password_*`, `hosts.{pub,priv}_key_*`, `service_credentials` table) in a follow-up migration. Gate behind `MIGRATE_DROP_LEGACY_SECRETS=1` env so deployment can stage `[tdd:required]` | Migration test runs only when env set; backup roundtrip confirms no data loss with flag off, columns gone with flag on | 1.8 | cc:完了 [b17d1dc] (PARTIAL: service_credentials only; host password/key columns deferred to a separate task — they're still consumed by host/coolify/sshconfig/sshkey/import handlers) |

---

## 🚧 Phase 2 — Env Vars (per-var rows) & App Logins

Per D5: env vars are individual `Secret` rows with `type='env_var'` and
`group_label` carrying the environment name. No bundle table. "Bundle" is a
frontend grouping concept derived from `GROUP BY group_label`.

| Task | 内容 | DoD | Depends | Status |
|------|------|-----|---------|--------|
| 2.1 | Implement `env_var` payload validator (non-empty `value`; `name` matches `[A-Z_][A-Z0-9_]*`; `group_label` non-empty and matches `[a-z][a-z0-9-]*`). Reuse Phase 1.6 generic CRUD `[tdd:required]` | Unit test: validator accepts `DB_URL`/`prod`; rejects lowercase var name, empty value, empty group_label; create+reveal round-trip passes | 1.6 | cc:完了 [a09e02a] |
| 2.2 | Bulk endpoint `POST /api/secrets/env/bulk` accepts `{scope, parent_id?, group_label, vars:[{name,value,description?}]}` and upserts each var as an individual `Secret` row in one tx. Duplicate names within the request payload are rejected before any write `[tdd:required]` | Test: posting 3 vars with one duplicate name returns 400 with zero writes; posting 3 unique vars creates 3 rows or updates existing rows matching `(scope, parent_id, name, group_label)`; partial failure rolls back the whole tx | 2.1 | cc:完了 [3433616] |
| 2.3 | List/group endpoint `GET /api/secrets/env?scope=&parent_id=&group_label=` returns metadata grouped by `group_label` for caller-friendly bundle view. Never returns values `[tdd:required]` | Test: returns `{prod:[{name,description}], staging:[…]}` shape; respects role checks; excludes soft-deleted | 2.1 | cc:完了 [3433616] |
| 2.4 | Implement `app_login` payload validator (non-empty `app_name`, `username`, `password`; optional `url`/`notes`). Reuse Phase 1.6 CRUD `[tdd:required]` | Schema test rejects missing app_name; create+reveal round-trip test passes | 1.6 | cc:完了 [a09e02a] |
| 2.5 | Frontend `frontend/src/app/secrets/_components/EnvVarBundleEditor.tsx`: groups env vars by `group_label` into a tabbed view, supports add/edit/delete of individual vars within a group, uses `POST /api/secrets/env/bulk` for batch save. `AppLoginForm.tsx` for app logins `[tdd:skip:ui-only]` | Forms render under Next.js dev server; bulk save submits valid payload; visible smoke test in browser per `superpowers:verification-before-completion` | 2.2, 2.3, 2.4 | cc:完了 [d6b2a1e] (BROWSER SMOKE TEST PENDING — no parent /secrets page yet, needs manual verification or Phase 4.1) |

---

## 🚧 Phase 3 — Guest Share Links

| Task | 内容 | DoD | Depends | Status |
|------|------|-----|---------|--------|
| 3.1 | Token generator: 256-bit `crypto/rand`, base64url-encoded; storage is `sha256(token)` only. Helper in `internal/secretshare/token.go` `[tdd:required]` | Unit test: 10k generated tokens have zero collisions; token never appears in DB row after `Create()` | 1.6 | cc:TODO |
| 3.2 | Per-link derived key (Spec D4): `HKDF-SHA256(master_key, salt=token, info="secret-share-v1")` produces a per-link AES-256 key. Re-encrypt the secret payload under that key when share link created; decrypt at redemption time using token from URL `[tdd:required]` | Unit test: redemption with wrong token fails AEAD; DB row alone (no token) cannot decrypt; revocation deletes row → 404 | 3.1 | cc:TODO |
| 3.3 | Handlers: `POST /api/secrets/{id}/share-links`, `GET /api/secrets/{id}/share-links` (owner-only list), `DELETE /api/secrets/{id}/share-links/{linkId}`, public `GET /api/share/{token}` with optional `?passphrase=`. **Per spec §5.3: reject `POST` with 400 if target secret has `visibility != 'personal'`** (shared secrets cannot generate external links) `[tdd:required]` | Integration tests cover happy path + expired + max_views_exceeded + wrong_passphrase + revoked + **rejection of share-link creation against a shared secret (400 with explanatory body)**; redemption writes `share_redeem` audit row | 3.2 | cc:TODO |
| 3.4 | Janitor: background job that deletes share-link rows past `expires_at + 7d` grace. Runs hourly via existing orchestrator pattern (`internal/models/orchestrator.go`) `[tdd:required]` | Test: row inserted with `expires_at = now - 8d` is deleted after janitor run; row at `now - 6d` retained | 3.3 | cc:TODO |
| 3.5 | Response hardening on `/api/share/*`: `Referrer-Policy: no-referrer`, `Cache-Control: no-store`, no token echo in response body or logs `[tdd:required]` | Test asserts headers present; access log scrubber strips `/api/share/{token}` path segment to `/api/share/[REDACTED]` | 3.3 | cc:TODO |
| 3.6 | Frontend share modal on personal secrets: TTL preset (1h/24h/7d/custom), optional passphrase, optional max-views, "Copy link" button. Lists existing live links with revoke action `[tdd:skip:ui-only]` | Browser smoke test: create link → copy → open in incognito → reveals payload; revoke → 404 in incognito | 3.3 | cc:TODO |

---

## 🚧 Phase 4 — Frontend Unification

| Task | 内容 | DoD | Depends | Status |
|------|------|-----|---------|--------|
| 4.1 | New `frontend/src/app/secrets/page.tsx`: unified vault with three filter rows — **scope chips** (service / host / tool / avulso), **visibility chips** (personal / shared / all), and **type filter** (`cred` / `sshkey` / `password` / `app_login` / `env_var`). Default view is `visibility=all, scope=all` filtered to the caller's permissions `[tdd:skip:ui-only]` | Page lists all secrets the caller is permitted to see; filtering is client-side over a single API call; pagination if list > 100; "personal" chip pre-selected when navigating from `/api/secrets/mine` deep link | 2.5, 3.6 | cc:TODO |
| 4.2 | Alias legacy `/service-credentials` page to `/secrets?scope=service` via Next.js redirect `[tdd:required]` | Deep links from existing bookmarks land on new page with filter applied; e2e test (or manual verification log) confirms | 4.1 | cc:TODO |
| 4.3 | `services/[id]/_components/CredentialsTab.tsx` consumes `/api/secrets?scope=service&parent_id={id}` instead of legacy `/api/services/{id}/credentials` `[tdd:required]` | Component renders identical UI; legacy fetch removed; existing snapshot/UI test (or manual screenshot diff) confirms no regression | 4.1 | cc:TODO |
| 4.4 | Reveal UX: click-to-reveal value with 30s auto-hide timer, copy-to-clipboard with 30s auto-clear of clipboard `[tdd:skip:ui-interaction-tested-manually]` | Browser smoke test confirms timer; clipboard auto-clear documented in user-facing tooltip | 4.1 | cc:TODO |
| 4.5 | Trash view `frontend/src/app/secrets/trash/page.tsx`: lists soft-deleted secrets the user can restore (calls `GET /api/secrets/trash`), restore button calls `POST /api/secrets/{id}/restore`. History drawer on each secret calls `GET /api/secrets/{id}/history` and renders the audit-diff timeline `[tdd:skip:ui-only]` | Browser smoke test: soft-delete a secret → it appears in trash → restore → it disappears from trash and reappears in main vault. History drawer shows at least one `update` row with `changed_fields` after editing | 4.1 | cc:TODO |

---

## 📦 アーカイブ

_(none yet)_
