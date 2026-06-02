"use client";

import { use, useCallback, useEffect, useState } from "react";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Card from "@/components/ui/Card";
import ApiReference from "@/components/atlas/apis/ApiReference";
import type { BundlePayload } from "@/lib/types";

// Public reveal page. Reached for BOTH kinds of share link:
//   - a single secret  (ShareLinkModal → GET /api/share/{token})
//   - a heterogeneous bundle (ShareBundleModal → GET /api/share-bundle/{token})
// The URL is identical (/share/{token}), so we probe the bundle endpoint
// first and fall back to the secret endpoint on 404. The token IS the
// credential; an optional passphrase gates the reveal on top of it.
//
// Both endpoints return:
//   200 { ... }   -> reveal
//   401 { error } -> passphrase required or incorrect
//   404 { error } -> expired / revoked / exhausted / unknown / wrong-kind
// We branch on HTTP status, never message text.

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "";

type ViewState =
  | { kind: "loading" }
  | { kind: "needsPassphrase"; incorrect: boolean }
  | { kind: "revealed"; payload: string }
  | { kind: "bundle"; payload: BundlePayload }
  | { kind: "gone" }
  | { kind: "error"; message: string };

function withPass(path: string, passphrase?: string): string {
  const qs = passphrase ? `?passphrase=${encodeURIComponent(passphrase)}` : "";
  return `${API_BASE}${path}${qs}`;
}

async function redeem(token: string, passphrase?: string): Promise<ViewState> {
  const enc = encodeURIComponent(token);

  // 1) Try the bundle endpoint first.
  let bundleRes: Response;
  try {
    bundleRes = await fetch(withPass(`/api/share-bundle/${enc}`, passphrase));
  } catch {
    return { kind: "error", message: "Could not reach the server. Check your connection and try again." };
  }
  if (bundleRes.ok) {
    const data = (await bundleRes.json().catch(() => null)) as BundlePayload | null;
    if (data) return { kind: "bundle", payload: data };
  } else if (bundleRes.status === 401) {
    return { kind: "needsPassphrase", incorrect: Boolean(passphrase) };
  } else if (bundleRes.status !== 404) {
    const body = await bundleRes.json().catch(() => ({}));
    return { kind: "error", message: body.error || `Request failed (${bundleRes.status}).` };
  }

  // 2) Not a bundle (404) — fall back to a single-secret link.
  let res: Response;
  try {
    res = await fetch(withPass(`/api/share/${enc}`, passphrase));
  } catch {
    return { kind: "error", message: "Could not reach the server. Check your connection and try again." };
  }
  if (res.ok) {
    const data = await res.json().catch(() => ({}));
    return { kind: "revealed", payload: typeof data.payload === "string" ? data.payload : "" };
  }
  if (res.status === 401) return { kind: "needsPassphrase", incorrect: Boolean(passphrase) };
  if (res.status === 404) return { kind: "gone" };
  const body = await res.json().catch(() => ({}));
  return { kind: "error", message: body.error || `Request failed (${res.status}).` };
}

// Some payloads are JSON (env-var bundles, app logins). Pretty-print those so
// they're readable; otherwise show the raw string.
function prettyPayload(payload: string): string {
  const trimmed = payload.trim();
  if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2);
    } catch {
      /* not JSON — fall through */
    }
  }
  return payload;
}

export default function SharedSecretPage(props: { params: Promise<{ token: string }> }) {
  // Next 16: route params arrive as a Promise, unwrapped with React's use().
  const { token } = use(props.params);

  const [state, setState] = useState<ViewState>({ kind: "loading" });
  const [passphrase, setPassphrase] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);

  const attempt = useCallback(
    async (pass?: string) => {
      const next = await redeem(token, pass);
      setState(next);
    },
    [token],
  );

  useEffect(() => {
    void attempt();
  }, [attempt]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!passphrase) return;
    setSubmitting(true);
    await attempt(passphrase);
    setSubmitting(false);
  }

  async function handleCopy(value: string, key: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      setTimeout(() => setCopied(null), 2000);
    } catch {
      /* clipboard unavailable — user can still select manually */
    }
  }

  // Bundles get a full-width layout (API docs need room); everything else
  // stays in the narrow centered card.
  if (state.kind === "bundle") {
    const b = state.payload;
    return (
      <main className="min-h-screen p-6" style={{ background: "var(--bg-base)" }}>
        <div className="max-w-5xl mx-auto space-y-5">
          <div>
            <h1 className="text-lg font-semibold text-[var(--text-primary)]">{b.title || "Shared bundle"}</h1>
            <p className="text-xs text-[var(--text-muted)]">Someone shared this with you through a short-lived link.</p>
          </div>

          {b.secrets.length > 0 && (
            <div className="space-y-3">
              <h2 className="text-sm font-semibold text-[var(--text-secondary)]">Secrets</h2>
              {b.secrets.map((s, i) => (
                <Card key={i}>
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium text-[var(--text-primary)]">
                      {s.name} <span className="text-xs text-[var(--text-muted)]">({s.type})</span>
                    </span>
                    <Button variant="secondary" size="sm" onClick={() => handleCopy(s.payload, `s-${i}`)}>
                      {copied === `s-${i}` ? "Copied!" : "Copy"}
                    </Button>
                  </div>
                  <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 font-mono text-xs text-[var(--text-primary)]">
                    {prettyPayload(s.payload)}
                  </pre>
                </Card>
              ))}
            </div>
          )}

          {b.api_docs.map((doc, i) => (
            <div key={i} className="space-y-2">
              <div className="flex items-center gap-3">
                <h2 className="text-sm font-semibold text-[var(--text-secondary)]">{doc.name}</h2>
                {doc.external_url && (
                  <a href={doc.external_url} target="_blank" rel="noopener noreferrer" className="text-xs text-[var(--accent)] hover:underline">
                    Open externally ↗
                  </a>
                )}
              </div>
              <Card className="p-0 overflow-hidden">
                <ApiReference content={doc.spec} showSidebar={false} hideTestRequest />
              </Card>
            </div>
          ))}

          {b.secrets.length === 0 && b.api_docs.length === 0 && (
            <p className="text-sm text-amber-400">This link no longer has any available content.</p>
          )}
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen flex items-center justify-center p-6">
      <div className="w-full max-w-md">
        <Card>
          <h1 className="text-base font-semibold text-[var(--text-primary)] mb-1">Shared secret</h1>
          <p className="text-xs text-[var(--text-muted)] mb-5">
            Someone shared a secret with you through a short-lived link.
          </p>

          {state.kind === "loading" && <p className="text-sm text-[var(--text-muted)]">Loading…</p>}

          {state.kind === "needsPassphrase" && (
            <form onSubmit={handleSubmit} className="space-y-3">
              <p className="text-sm text-[var(--text-secondary)]">
                This link is protected. Enter the passphrase to reveal it.
              </p>
              <Input
                type="password"
                autoFocus
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder="Passphrase"
                autoComplete="off"
                error={state.incorrect ? "Incorrect passphrase. Please try again." : undefined}
              />
              <Button type="submit" loading={submitting} disabled={!passphrase}>
                Unlock
              </Button>
            </form>
          )}

          {state.kind === "revealed" && (
            <div className="space-y-3">
              <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 font-mono text-xs text-[var(--text-primary)]">
                {prettyPayload(state.payload)}
              </pre>
              <Button variant="secondary" size="sm" onClick={() => handleCopy(state.payload, "secret")}>
                {copied === "secret" ? "Copied!" : "Copy to clipboard"}
              </Button>
            </div>
          )}

          {state.kind === "gone" && (
            <p className="text-sm text-amber-400">
              This link has expired or is no longer valid. Ask the sender for a new one.
            </p>
          )}

          {state.kind === "error" && <p className="text-sm text-red-400">{state.message}</p>}
        </Card>
      </div>
    </main>
  );
}
