# Spec — Secrets Manager (evolution of credentials module)

Status: **Ratified (2026-05-25)** — D1–D6 locked, see §3
Owner: Bridge backend
Last updated: 2026-05-25

## 1. Goal

Evolve the existing service-scoped `ServiceCredential` module into a unified secrets
manager that handles credentials, SSH keys, standalone passwords, app logins, and
environment-variable bundles, with per-user ownership and guest share links.

## 2. Scope decisions (locked)

| Decision | Choice |
|---|---|
| Data model | **Unified `Secret` table** with `type` + `scope` columns. Migrate existing `service_credentials`, `hosts.password_*`, and `hosts.{pub,priv}_key_*` into it. |
| Share-link security | **Token + expiry + multi-view** (configurable `max_views`, optional passphrase). |
| Delivery | **Phased** (4 phases, see Plans.md). |
| Encryption-at-rest | AES-256-GCM (unchanged). Key source unchanged: `SSHCM_SECRET_KEY` → keyfile → generate. |

## 3. Ratified decisions (2026-05-25)

| # | Question | Decision |
|---|---|---|
| D1 | Can `admin` reveal personal secrets owned by other users? | **No.** No break-glass in any phase of this spec; if required later, opened as a new spec. |
| D2 | Key rotation strategy | **`key_version INT NOT NULL DEFAULT 1` column added now**; no rotation tool in scope. Lazy re-encryption on next write when a future tool bumps the master version. |
| D3 | Soft vs hard delete | **Soft delete.** `deleted_at TIMESTAMP NULL` column on `secrets`. All read paths filter `deleted_at IS NULL` by default. `POST /api/secrets/{id}/restore` un-deletes. `DELETE` on a service/host cascades to **soft-deleting** its child secrets (restoring the parent restores the children atomically). Hard purge endpoint deferred to a follow-up spec. |
| D3b | Modification visibility | **Audit-log-driven diff view** in Phase 1: every `update` audit row gets `metadata.changed_fields = ["name", "payload", …]` (field names only, no values). **Full payload version history** (decryptable prior values) is out of scope for this spec — flagged in §9 as a deliberate follow-up so it gets its own threat-model review. |
| D4 | Share-link crypto: master-key decrypt or per-link key? | **Per-link derived key**: HKDF-SHA256(`master_key`, salt=`raw_token`, info=`"secret-share-v1"`) → AES-256 key for share payload. Raw token lives only in URL; DB stores `token_hash` and HKDF salt is the raw token, so DB compromise alone cannot decrypt. |
| D5 | Env-var bundles: single payload or per-var rows? | **Per-var rows.** Each env var is its own `Secret` with `type='env_var'` and a new `group_label TEXT NULL` column carrying the environment (`"prod"`, `"staging"`, …). "Bundle" is a frontend grouping concept only; backend just sees a flat list filtered by `group_label`. Unique constraint widens to `(scope, parent_id, name, group_label)`. |
| D6 | Admin metadata visibility for personal secrets | **Names + types + scope visible** to admin (compliance/inventory). Values, audit details, and share-link tokens not. Owner sees full. |
| D7 | Scope vs personal: one enum or two axes? | **Two orthogonal axes.** `scope ∈ {service, host, tool, avulso}` describes *what the secret is attached to* (avulso = unattached). `visibility ∈ {personal, shared}` describes *who can see it* — `personal` = owner-only, `shared` = role-based ACL. Any combination is valid (e.g. a `host`-scoped `personal` SSH key, an `avulso` `shared` team password). **External share links are only valid when `visibility='personal'`** (shared secrets are already internally accessible — there is no use case for sharing them externally). |

## 4. Data model

### 4.1 Tables

#### `secrets`
```
id                 BIGSERIAL PK
type               ENUM('cred', 'sshkey', 'password', 'app_login', 'env_var')
scope              ENUM('service', 'host', 'tool', 'avulso')  -- where it lives (D7)
visibility         ENUM('personal', 'shared')                  -- who can see it (D7)
parent_id          BIGINT NULL    -- polymorphic FK by scope: service.id | host.id | external_tool.id; NULL for avulso
owner_user_id      BIGINT NOT NULL -- creator; ALWAYS set, regardless of scope/visibility
name               TEXT NOT NULL  -- human label; role_name for service/host scope, var name for env_var
group_label        TEXT NULL      -- environment ("prod","staging",…) for env_var; reserved/null for other types
description        TEXT NULL
payload_ciphertext BYTEA NOT NULL
payload_nonce      BYTEA NOT NULL
key_version        INT NOT NULL DEFAULT 1
created_by         BIGINT NOT NULL
created_at         TIMESTAMP
updated_at         TIMESTAMP
deleted_at         TIMESTAMP NULL -- soft delete; all read paths filter IS NULL by default

CHECK (scope = 'avulso' OR parent_id IS NOT NULL)  -- scoped secrets must have a parent
CHECK (scope <> 'avulso' OR parent_id IS NULL)     -- avulso must not

-- Shared secrets are unique per parent+name+group; personal secrets are unique per owner+parent+name+group
-- (two users can each have their own personal "SSH key" on the same host)
UNIQUE (scope, parent_id, name, group_label)              WHERE visibility='shared'   AND deleted_at IS NULL
UNIQUE (scope, parent_id, owner_user_id, name, group_label) WHERE visibility='personal' AND deleted_at IS NULL

INDEX  (owner_user_id, visibility) WHERE deleted_at IS NULL
INDEX  (scope, parent_id) WHERE deleted_at IS NULL
INDEX  (deleted_at) WHERE deleted_at IS NOT NULL  -- trash listing
```

Cascading soft delete (D3): when a `services`/`hosts`/`external_tools` row is
soft-deleted, all `secrets` rows with matching `(scope, parent_id)` get their
`deleted_at` set to the same timestamp. `avulso` secrets are never cascaded
(no parent). Restoring the parent restores the children in the same tx.

#### `secret_share_links`
```
id              BIGSERIAL PK
secret_id       BIGINT NOT NULL FK secrets(id) ON DELETE CASCADE
token_hash      BYTEA NOT NULL   -- sha256(raw_token); raw_token never stored
expires_at      TIMESTAMP NOT NULL
passphrase_hash BYTEA NULL       -- argon2id(passphrase) if set
max_views       INT NULL         -- null = unlimited until expiry
view_count      INT NOT NULL DEFAULT 0
created_by      BIGINT NOT NULL
created_at      TIMESTAMP
revoked_at      TIMESTAMP NULL

UNIQUE (token_hash)
INDEX  (secret_id)
INDEX  (expires_at)              -- for janitor
```

#### `secret_audit_log`
```
id             BIGSERIAL PK
secret_id      BIGINT NOT NULL  -- not FK: log survives secret deletion
action         ENUM('create','reveal','update','delete','restore','share_create','share_redeem','share_revoke')
actor_user_id  BIGINT NULL      -- null for anonymous share redemption
actor_ip       TEXT NULL
share_link_id  BIGINT NULL      -- set for share_* actions
metadata       JSONB NULL       -- update: {"changed_fields":["name","payload"]}; share_create: {"max_views":5,"passphrase":true}
at             TIMESTAMP

INDEX (secret_id, at)
INDEX (actor_user_id, at)
```

### 4.2 Encrypted payload shapes (per `type`)

```json
// type=cred                 -- username + password against a host
{"username":"alice","password":"…"}

// type=sshkey               -- SSH keypair
{"username":"alice","private_key_pem":"…","public_key":"ssh-ed25519 …","passphrase":"…?"}

// type=password             -- standalone password (no username)
{"value":"…"}

// type=app_login            -- credentials for an external app
{"app_name":"Jira","url":"https://…","username":"…","password":"…","notes":"…?"}

// type=env_var              -- single environment variable; `name` column = var name, `group_label` = environment
{"value":"…","description":"…?"}
```

Payload is JSON, then AES-256-GCM encrypted whole, then stored in `payload_ciphertext`.

## 5. Access control

Authorization decomposes along the two D7 axes. Let `me` = caller.

### 5.1 visibility=shared (RBAC governs)

| Caller role | scope=service/host/tool | scope=avulso |
|---|---|---|
| viewer | list + reveal | list + reveal |
| editor | list + reveal + create + update | same |
| admin  | list + reveal + create + update + delete + restore | same |

### 5.2 visibility=personal (ownership governs)

| Caller | owner_user_id = me | owner_user_id = other |
|---|---|---|
| any role | full CRUD + reveal + share-link mgmt + restore | **D6**: admin sees name/type/scope only, no reveal, no audit, no share-link list. All other roles: 404. |

### 5.3 Other rules

- **Share-link creation** (`POST /api/secrets/{id}/share-links`): rejected with 400 if `visibility != 'personal'` (D7). Shared secrets have no external-sharing use case.
- **Share-link redemption** (`GET /api/share/{token}`) bypasses caller-role rules — the token itself is the capability. Reveal still writes `secret_audit_log`.
- **Cascading soft delete** of a parent service/host/tool soft-deletes *all* its child secrets regardless of visibility (otherwise restoring the parent would leave orphan-restored personal secrets behind).

## 6. API surface (Phase 1–3)

```
# New unified surface (Phase 1)
GET    /api/secrets?scope=&parent_id=&type=&group_label=&visibility=&include_deleted=  list metadata (never plaintext)
POST   /api/secrets                              create (encrypts payload; body sets scope, visibility, parent_id?, …)
GET    /api/secrets/{id}                         metadata only
GET    /api/secrets/{id}/reveal                  decrypts, logs audit
PUT    /api/secrets/{id}                         update name/description/payload (logs changed_fields); visibility immutable
DELETE /api/secrets/{id}                         soft delete (sets deleted_at)
POST   /api/secrets/{id}/restore                 un-delete (owner or original scope role)
GET    /api/secrets/{id}/history                 audit-log diff view (who/when/changed_fields)
GET    /api/secrets/trash                        list soft-deleted items the caller can restore

# Personal convenience (Phase 1)
GET    /api/secrets/mine                         alias for visibility=personal & owner_user_id=me (any scope)

# Share links (Phase 3)
POST   /api/secrets/{id}/share-links             body: {ttl_seconds, max_views?, passphrase?}
GET    /api/secrets/{id}/share-links             list (owner only)
DELETE /api/secrets/{id}/share-links/{linkId}    revoke
GET    /api/share/{token}                        public; body: {passphrase?}; returns plaintext + remaining_views

# Backward compat shims (Phase 1.7, removed in 6 months or major version bump)
GET/POST/DELETE /api/services/{id}/credentials   delegates to /api/secrets with scope=service
GET            /api/tools/{id}/credentials       same
```

## 7. Migration from legacy tables

Single irreversible (in Go) migration in Phase 1.5:

1. Read each `service_credentials` row → INSERT into `secrets` (scope=service, type=cred|app_login per heuristic on `role_name`, parent_id=service_id, payload re-wrapped as `{"username":role_name, "ciphertext":<existing>}` — actually preserve ciphertext+nonce as-is and copy `role_name` into `name`).
2. Read each `hosts.password_*` non-null → INSERT (scope=host, type=password, parent_id=host_id, name='ssh-password'). Then drop columns in follow-up migration.
3. Read each `hosts.{pub,priv}_key_*` non-null → INSERT (scope=host, type=sshkey, name='ssh-key', payload re-encrypted to combined `{public_key, private_key}` JSON).
4. Verify with extended `TestCrossDialectRestore` covering all three migrated shapes.
5. Drop legacy columns (`service_credentials.*`, `hosts.password_*`, `hosts.{pub,priv}_key_*`) in a **separate migration** committed in Phase 1.8 after compat shim is exercised in dev.

## 8. Threat model (non-exhaustive)

| Threat | Mitigation |
|---|---|
| DB dump → all secrets readable | AES-256-GCM with key NOT in DB (env or keyfile). Already in place. |
| Lost encryption key on redeploy | Existing `KeySource` detection (commit 4b13c94). No new exposure. |
| Share-link token leak via referer/browser history/server log | Token in URL **path**, not query string. `Referrer-Policy: no-referrer` on share endpoints. Server log scrubber strips tokens from access log. |
| DB compromise reveals share-link target | Per-link derived key (D4): even with DB + master key, the link is unreadable without the token. |
| Brute force `token_hash` lookup | 256-bit token = infeasible. Optional passphrase + argon2id raises bar further. |
| Admin abuse on personal secrets | D1 + D6: admins see metadata only. Break-glass not in MVP. |
| Audit log tampering | Append-only via DB role permissions in Phase 1.8. |

## 9. Out of scope (explicitly deferred)

- **Full payload version history** (decryptable prior values per secret) — D3b. Audit-log diffs ship in Phase 1, but rolling back to a prior *value* needs ciphertext snapshots and its own threat-model review (snapshot storage = attack surface). Tracked as a separate spec.
- **Hard purge endpoint** (D3) — soft-deleted secrets currently live forever. A scheduled or manual purge with admin gating is a follow-up.
- Key rotation tooling (D2 column added; tool deferred).
- Break-glass admin reveal (D1) — never. If business needs it, opens as a new spec.
- Email notification on share-link redemption (Phase 3 stretch).
- HSM / KMS integration (existing key sourcing unchanged).
- Per-secret access logs visible to owner UI (Phase 4+).
- Federated sharing across Bridge instances.

## 10. References

- Existing encryption: `internal/database/encrypt.go`
- Existing credential model: `internal/models/service_credential.go`
- Existing host secret fields: `internal/models/host.go`
- Existing handlers: `internal/api/service_handlers.go:229-420`
- Relevant commits: `4b13c94` (key-loss guard), `ec72e40` (SSH auth policy)
