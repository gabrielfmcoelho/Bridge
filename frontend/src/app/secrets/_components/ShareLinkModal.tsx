"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Card from "@/components/ui/Card";

interface ShareLinkModalProps {
  // Personal-only — the backend rejects share-link creation for shared
  // secrets with 400 (spec section 5.3). The modal stays unmounted on
  // non-personal secrets; this prop is here so the parent component can
  // pass the id once and let the modal handle everything.
  secretID: number | null;
  onClose: () => void;
}

type TTLPreset = "1h" | "24h" | "7d" | "custom";

const TTL_SECONDS: Record<Exclude<TTLPreset, "custom">, number> = {
  "1h": 3600,
  "24h": 24 * 3600,
  "7d": 7 * 24 * 3600,
};

function shareBaseUrl(): string {
  const configured_url = process.env.NEXT_PUBLIC_BASE_URL;
  if (!configured_url) {
    throw new Error("NEXT_PUBLIC_BASE_URL is not defined");
  }
  return configured_url;
}

// ShareLinkModal — Phase 3 Task 3.6. Personal secrets only. Surfaces TTL
// preset selection + optional passphrase + optional max-views, lists
// existing live + revoked links so the owner can copy a previously-issued
// URL or revoke one.
//
// The raw token is emitted ONLY by POST /api/secrets/{id}/share-links and
// never resurfaces. After the modal closes, the URL is unrecoverable —
// the copy-to-clipboard button right after creation is the operator's
// last chance.
export default function ShareLinkModal({ secretID, onClose }: ShareLinkModalProps) {
  const qc = useQueryClient();
  const open = secretID != null;

  const [ttlPreset, setTTLPreset] = useState<TTLPreset>("24h");
  const [customTTLSeconds, setCustomTTLSeconds] = useState<string>("");
  const [passphrase, setPassphrase] = useState("");
  const [maxViews, setMaxViews] = useState<string>("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  // Last successful create — holds the raw token + url for the
  // copy-to-clipboard affordance. Cleared when the user creates another or
  // closes the modal.
  const [justCreated, setJustCreated] = useState<{ token: string; url: string } | null>(null);
  const [copyState, setCopyState] = useState<"idle" | "copied">("idle");

  const { data: links = [], refetch } = useQuery({
    queryKey: ["share-links", secretID],
    queryFn: () => secretID ? secretsAPI.listShareLinks(secretID) : Promise.resolve([]),
    enabled: open,
  });

  const create = useMutation({
    mutationFn: async () => {
      if (secretID == null) throw new Error("no secret selected");
      let ttlSeconds = 0;
      if (ttlPreset === "custom") {
        ttlSeconds = Number(customTTLSeconds) || 0;
      } else {
        ttlSeconds = TTL_SECONDS[ttlPreset];
      }
      const res = await secretsAPI.createShareLink(secretID, {
        ttl_seconds: ttlSeconds,
        max_views: Number(maxViews) || 0,
        passphrase: passphrase || undefined,
        title: title.trim() || undefined,
        description: description.trim() || undefined,
      });
      return res;
    },
    onSuccess: (res) => {
      setJustCreated({ token: res.token, url: res.url });
      setCopyState("idle");
      qc.invalidateQueries({ queryKey: ["share-links", secretID] });
      // Reset the form for the next link.
      setPassphrase("");
      setMaxViews("");
      setTitle("");
      setDescription("");
    },
  });

  const revoke = useMutation({
    mutationFn: async (linkID: number) => {
      if (secretID == null) return;
      await secretsAPI.revokeShareLink(secretID, linkID);
    },
    onSuccess: () => {
      refetch();
    },
  });

  const handleCopy = async () => {
    if (!justCreated) return;
    try {
      const fullURL = shareBaseUrl() + justCreated.url;
      await navigator.clipboard.writeText(fullURL);
      setCopyState("copied");
      setTimeout(() => setCopyState("idle"), 2000);
    } catch {
      // No-op on clipboard failure — user can still select + copy manually.
    }
  };

  const handleClose = () => {
    setJustCreated(null);
    setCopyState("idle");
    onClose();
  };

  return (
    <ResponsiveModal open={open} onClose={handleClose} title="Share link">
      <div className="space-y-4">
        {/* Just-created banner: only chance the raw token appears in the UI. */}
        {justCreated && (
          <Card>
            <div className="flex items-start gap-2">
              <svg className="w-4 h-4 text-emerald-400 mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75" />
              </svg>
              <div className="flex-1 min-w-0">
                <p className="text-xs font-medium text-emerald-400 mb-1">Link created</p>
                <p className="text-[10px] text-[var(--text-faint)] mb-2">
                  Copy this URL now — it will not be shown again.
                </p>
                <div className="flex gap-2 items-center">
                  <code className="text-[10px] text-[var(--text-secondary)] flex-1 truncate font-mono bg-[var(--bg-surface)] px-2 py-1 rounded border border-[var(--border-subtle)]">
                    {window.location.origin}{justCreated.url}
                  </code>
                  <Button size="sm" variant="secondary" onClick={handleCopy}>
                    {copyState === "copied" ? "Copied!" : "Copy"}
                  </Button>
                </div>
              </div>
            </div>
          </Card>
        )}

        {/* Create form */}
        <Card>
          <h3 className="text-sm font-semibold mb-3">New share link</h3>
          <div className="space-y-3">
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1.5 block">
                Nickname (optional)
              </label>
              <Input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="A label for this link"
              />
            </div>

            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1.5 block">
                Description (optional)
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Shown to whoever opens the link"
                rows={2}
                className="w-full rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-faint)] focus:border-[var(--accent)] focus:outline-none resize-y"
              />
            </div>

            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1.5 block">Expires after</label>
              <div className="flex flex-wrap gap-2">
                {(["1h", "24h", "7d", "custom"] as const).map((preset) => (
                  <button
                    key={preset}
                    type="button"
                    onClick={() => setTTLPreset(preset)}
                    className={`px-3 py-1 rounded-[var(--radius-md)] text-xs border transition-colors ${ttlPreset === preset
                      ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/50"
                      : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-subtle)] hover:text-[var(--text-secondary)]"
                      }`}
                  >
                    {preset}
                  </button>
                ))}
              </div>
              {ttlPreset === "custom" && (
                <div className="mt-2">
                  <Input
                    type="number"
                    value={customTTLSeconds}
                    onChange={(e) => setCustomTTLSeconds(e.target.value)}
                    placeholder="Seconds (e.g. 1800 = 30min)"
                  />
                </div>
              )}
            </div>

            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1.5 block">
                Passphrase (optional)
              </label>
              <Input
                type="password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder="Guest must enter to redeem"
                autoComplete="new-password"
              />
            </div>

            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1.5 block">
                Max views (optional)
              </label>
              <Input
                type="number"
                value={maxViews}
                onChange={(e) => setMaxViews(e.target.value)}
                placeholder="Leave blank for unlimited (within TTL)"
              />
            </div>

            <div className="flex items-center gap-2">
              <Button
                size="sm"
                onClick={() => create.mutate()}
                disabled={create.isPending}
              >
                {create.isPending ? "Creating..." : "Create link"}
              </Button>
              {create.isError && (
                <span className="text-xs text-red-400">{(create.error as Error).message}</span>
              )}
            </div>
          </div>
        </Card>

        {/* Existing links */}
        <Card>
          <h3 className="text-sm font-semibold mb-3">Existing links</h3>
          {links.length === 0 ? (
            <p className="text-xs text-[var(--text-faint)]">No share links yet.</p>
          ) : (
            <div className="space-y-2">
              {links.map((link) => {
                const expired = new Date(link.expires_at) < new Date();
                const revoked = link.revoked_at != null;
                const maxReached =
                  link.max_views != null && link.view_count >= link.max_views;
                const dead = revoked || expired || maxReached;

                return (
                  <div
                    key={link.id}
                    className="flex items-center justify-between text-xs p-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)]"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-[var(--text-secondary)]">
                          {link.view_count}{link.max_views != null ? `/${link.max_views}` : ""} views
                        </span>
                        {link.has_passphrase && (
                          <span className="text-[10px] text-amber-400">passphrase</span>
                        )}
                        {revoked && <span className="text-[10px] text-red-400">revoked</span>}
                        {!revoked && expired && <span className="text-[10px] text-red-400">expired</span>}
                        {!revoked && !expired && maxReached && (
                          <span className="text-[10px] text-red-400">views exhausted</span>
                        )}
                      </div>
                      <div className="text-[10px] text-[var(--text-faint)] mt-0.5">
                        expires {new Date(link.expires_at).toLocaleString()}
                      </div>
                    </div>
                    {!dead && (
                      <Button
                        size="sm"
                        variant="danger"
                        onClick={() => revoke.mutate(link.id)}
                        disabled={revoke.isPending}
                      >
                        Revoke
                      </Button>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </Card>
      </div>
    </ResponsiveModal>
  );
}
