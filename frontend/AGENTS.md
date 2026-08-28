<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

<!-- BEGIN:secrets-path-edit-workaround -->
## A guardrail false positive under `src/app/secrets/`

Rule **R02** of the `claude-code-harness` plugin's guardrail
(`go/internal/guardrail/rules.go:150`) denies `Write`/`Edit` on paths that
look credential-shaped — `.env`, `.git/`, `*.pem`, `*.key`, `id_rsa` and
similar. Its path pattern also matches any path segment literally named
`secret`/`secrets`, so it denies edits under `src/app/secrets/` too, even
though that directory holds ordinary React source — the *path* contains the
word "secrets", not a credential.

**Never route around a guardrail** (e.g. editing a copy elsewhere and
shell-`cp`-ing it back into place). A blocked write is a signal to stop, not
a puzzle to solve around the tool. If a legitimate edit under this path is
denied, do one of:

- Ask the user to allow that specific path for the session.
- Hand the user the diff and have them apply it.

This is the same call the project's own later work made when it hit the
identical guard on `ShareLinkModal.tsx` — see
`docs/superpowers/specs/2026-06-26-share-metadata-audit-index-design.md:39`.

The route itself is already structured to shrink this surface:
`src/app/secrets/page.tsx` is a one-line re-export of
`@/components/vault/VaultPage`, and the page-level logic lives in
`src/components/vault/` — outside the guarded path. That split is
intentional; don't "fix" the re-export back into `src/app/secrets/`.

The modal/drawer components (`NewSecretModal.tsx`, `HistoryDrawer.tsx`,
`ShareLinkModal.tsx`, etc.) still live under `src/app/secrets/_components/`,
so they still sit inside the guarded path and R02 will still legitimately
fire on them — that's not a bug to work around, it's the same
ask-to-allow-or-hand-the-diff flow above.
<!-- END:secrets-path-edit-workaround -->
