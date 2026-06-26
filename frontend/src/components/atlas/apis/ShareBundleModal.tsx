"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Modal from "@/components/ui/Modal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import FormError from "@/components/ui/FormError";
import { shareBundlesAPI, secretsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { ApiCatalog } from "@/lib/types";

type Mode = "all" | "tags" | "operations";

export default function ShareBundleModal({
  open,
  onClose,
  api,
}: {
  open: boolean;
  onClose: () => void;
  api: ApiCatalog;
}) {
  const { t } = useLocale();
  const qc = useQueryClient();
  const [mode, setMode] = useState<Mode>("all");
  // Per-link renew duration (seconds) keyed by bundle id; default 24h.
  const [renewChoice, setRenewChoice] = useState<Record<number, number>>({});
  const [selTags, setSelTags] = useState<Set<string>>(new Set());
  const [selOps, setSelOps] = useState<Set<string>>(new Set());
  const [selSecrets, setSelSecrets] = useState<Set<number>>(new Set());
  const [ttlHours, setTtlHours] = useState("24");
  const [passphrase, setPassphrase] = useState("");
  // Optional: paste a token from a lost /share URL to rebuild that exact link.
  const [reuseToken, setReuseToken] = useState("");
  const [neverExpiry, setNeverExpiry] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const ops = useMemo(() => api.operations || [], [api.operations]);
  const tags = useMemo(() => {
    const s = new Set<string>();
    for (const op of ops) for (const tag of op.tags || []) s.add(tag);
    return Array.from(s).sort();
  }, [ops]);

  // Secrets in the same scope/project are the attachable set — both personal
  // (owner-only) and shared, since shared secrets are now externally
  // shareable by anyone who can reveal them. The backend re-checks access.
  const { data: secrets = [] } = useQuery({
    queryKey: ["secrets", "shareable", api.scope, api.parent_id],
    queryFn: () =>
      secretsAPI.list({
        scope: api.scope,
        parent_id: api.parent_id ?? undefined,
      }),
    enabled: open,
  });

  // Links already emitted for THIS API (server-side filtered; matches
  // multi-item bundles that include this api_doc too).
  const linksKey = ["share-bundles", "api", api.id] as const;
  const { data: existingLinks = [] } = useQuery({
    queryKey: linksKey,
    queryFn: () => shareBundlesAPI.listForItem("api_doc", api.id),
    enabled: open,
  });
  const renew = useMutation({
    mutationFn: ({ id, ttl }: { id: number; ttl: number }) =>
      shareBundlesAPI.renew(id, { ttl_seconds: ttl }),
    onSuccess: () => qc.invalidateQueries({ queryKey: linksKey }),
  });
  const revoke = useMutation({
    mutationFn: (id: number) => shareBundlesAPI.revoke(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: linksKey }),
  });

  function toggle<T>(set: Set<T>, val: T): Set<T> {
    const next = new Set(set);
    if (next.has(val)) next.delete(val);
    else next.add(val);
    return next;
  }

  function reset() {
    setMode("all");
    setSelTags(new Set());
    setSelOps(new Set());
    setSelSecrets(new Set());
    setTtlHours("24");
    setPassphrase("");
    setReuseToken("");
    setNeverExpiry(false);
    setError(null);
    setSubmitting(false);
    setResult(null);
    setCopied(false);
  }

  function close() {
    reset();
    onClose();
  }

  async function submit() {
    setError(null);
    if (mode === "tags" && selTags.size === 0) return setError(t("atlas.apis.selectSomething"));
    if (mode === "operations" && selOps.size === 0) return setError(t("atlas.apis.selectSomething"));

    const selector =
      mode === "all"
        ? { mode: "all" }
        : mode === "tags"
        ? { mode: "tags", tags: Array.from(selTags) }
        : { mode: "operations", op_keys: Array.from(selOps) };

    const items = [
      { type: "api_doc" as const, ref_id: api.id, selector },
      ...Array.from(selSecrets).map((id) => ({ type: "secret" as const, ref_id: id })),
    ];

    // ttl_seconds: -1 signals "never expires"; otherwise hours→seconds (blank
    // falls back to the backend's 24h default).
    const ttl = Number(ttlHours);
    const ttlSeconds = neverExpiry ? -1 : ttl > 0 ? ttl * 3600 : undefined;
    setSubmitting(true);
    try {
      const token = reuseToken.trim();
      if (token) {
        // Restore a lost link: rebuild under the token the holder still has, so
        // the exact /share/<token> URL works again.
        await shareBundlesAPI.reissue({
          token,
          title: api.name,
          ttl_seconds: ttlSeconds,
          passphrase: passphrase.trim() || undefined,
          items,
        });
        setResult(`${window.location.origin}/share/${token}`);
      } else {
        const res = await shareBundlesAPI.create({
          title: api.name,
          ttl_seconds: ttlSeconds,
          passphrase: passphrase.trim() || undefined,
          items,
        });
        setResult(`${window.location.origin}${res.url}`);
      }
      qc.invalidateQueries({ queryKey: linksKey });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  async function copy() {
    if (!result) return;
    await navigator.clipboard.writeText(result);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  const ModeBtn = ({ m, label }: { m: Mode; label: string }) => (
    <button
      type="button"
      onClick={() => setMode(m)}
      className={`px-3 py-1.5 text-xs rounded-[var(--radius-md)] border ${
        mode === m
          ? "border-[var(--accent)] text-[var(--accent)] bg-[var(--accent)]/10"
          : "border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
      }`}
    >
      {label}
    </button>
  );

  return (
    <Modal open={open} onClose={close} title={t("atlas.apis.shareTitle")}>
      {result ? (
        <div className="space-y-3">
          <p className="text-sm text-[var(--text-secondary)]">{t("atlas.apis.linkCreated")}</p>
          <div className="flex gap-2">
            <Input value={result} readOnly className="font-mono text-xs" />
            <Button type="button" onClick={copy}>
              {copied ? t("atlas.apis.copied") : t("atlas.apis.copyLink")}
            </Button>
          </div>
          <p className="text-xs text-amber-400">⚠ {t("atlas.apis.tokenOnce")}</p>
          <div className="flex justify-end pt-2">
            <Button variant="secondary" type="button" onClick={close}>
              {t("common.close") || "Close"}
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          <div>
            <label className="block text-sm mb-1.5 text-[var(--text-secondary)]">{t("atlas.apis.shareScopeLabel")}</label>
            <div className="flex gap-2">
              <ModeBtn m="all" label={t("atlas.apis.shareWhole")} />
              {tags.length > 0 && <ModeBtn m="tags" label={t("atlas.apis.shareByTags")} />}
              <ModeBtn m="operations" label={t("atlas.apis.shareByOps")} />
            </div>
          </div>

          {mode === "tags" && (
            <div className="max-h-40 overflow-y-auto space-y-1 border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-2">
              {tags.map((tag) => (
                <label key={tag} className="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
                  <input type="checkbox" checked={selTags.has(tag)} onChange={() => setSelTags((s) => toggle(s, tag))} />
                  {tag}
                </label>
              ))}
            </div>
          )}

          {mode === "operations" && (
            <div className="max-h-48 overflow-y-auto space-y-1 border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-2">
              {ops.map((op) => (
                <label key={op.op_key} className="flex items-center gap-2 text-xs text-[var(--text-secondary)]">
                  <input type="checkbox" checked={selOps.has(op.op_key)} onChange={() => setSelOps((s) => toggle(s, op.op_key))} />
                  <span className="font-bold w-12 inline-block">{op.method}</span>
                  <span className="font-mono truncate" style={{ fontFamily: "var(--font-mono)" }}>{op.path}</span>
                </label>
              ))}
            </div>
          )}

          <div>
            <label className="block text-sm mb-1.5 text-[var(--text-secondary)]">{t("atlas.apis.attachSecrets")}</label>
            {secrets.length === 0 ? (
              <p className="text-xs text-[var(--text-muted)]">{t("atlas.apis.noShareableSecrets")}</p>
            ) : (
              <div className="max-h-32 overflow-y-auto space-y-1 border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-2">
                {secrets.map((s) => (
                  <label key={s.id} className="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
                    <input type="checkbox" checked={selSecrets.has(s.id)} onChange={() => setSelSecrets((set) => toggle(set, s.id))} />
                    <span className="text-[10px] px-1.5 py-0.5 rounded border border-[var(--border-default)] bg-[var(--bg-overlay)] text-[var(--text-muted)]">{s.type}</span>
                    {s.name}
                  </label>
                ))}
              </div>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <Input
              label={t("atlas.apis.ttlHours")}
              type="number"
              value={ttlHours}
              onChange={(e) => setTtlHours(e.target.value)}
              disabled={neverExpiry}
            />
            <Input label={t("atlas.apis.passphraseOptional")} type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
          </div>

          <label className="flex items-center gap-2 text-xs text-[var(--text-secondary)]">
            <input
              type="checkbox"
              checked={neverExpiry}
              onChange={(e) => setNeverExpiry(e.target.checked)}
            />
            {t("atlas.apis.neverExpires")}
          </label>
          {neverExpiry && (
            <p className="-mt-2 text-[10px] text-amber-400">{t("atlas.apis.neverExpiresHint")}</p>
          )}

          <Input
            label={t("atlas.apis.reuseTokenLabel")}
            value={reuseToken}
            onChange={(e) => setReuseToken(e.target.value)}
            placeholder={t("atlas.apis.reuseTokenPlaceholder")}
            className="font-mono text-xs"
          />

          {error && <FormError message={error} />}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" type="button" onClick={close}>
              {t("common.cancel") || "Cancel"}
            </Button>
            <Button type="button" onClick={submit} loading={submitting}>
              {submitting
                ? t("atlas.apis.creating")
                : reuseToken.trim()
                  ? t("atlas.apis.restoreLink")
                  : t("atlas.apis.createLink")}
            </Button>
          </div>

          {/* Links already emitted for this API — renew (same URL) or revoke. */}
          <div className="border-t border-[var(--border-subtle)] pt-3">
            <label className="block text-sm mb-1.5 text-[var(--text-secondary)]">
              {t("atlas.apis.existingLinks")}
            </label>
            {existingLinks.length === 0 ? (
              <p className="text-xs text-[var(--text-muted)]">{t("atlas.apis.noLinks")}</p>
            ) : (
              <div className="max-h-48 overflow-y-auto space-y-2">
                {existingLinks.map((link) => {
                  const archived = link.deleted_at != null;
                  const expired =
                    link.expires_at != null &&
                    new Date(link.expires_at).getTime() < Date.now();
                  const revoked = link.revoked_at != null;
                  const exhausted =
                    link.max_views != null && link.view_count >= link.max_views;
                  const dead = archived || expired || revoked || exhausted;
                  const ttl = renewChoice[link.id] ?? 86400;
                  return (
                    <div
                      key={link.id}
                      className="flex items-center justify-between gap-2 text-xs p-2 rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]"
                    >
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-[var(--text-secondary)]">
                            {link.view_count}
                            {link.max_views != null ? `/${link.max_views}` : ""}{" "}
                            {t("atlas.apis.views")}
                          </span>
                          {link.has_passphrase && (
                            <span className="text-[10px] text-amber-400">
                              {t("atlas.apis.withPassphrase")}
                            </span>
                          )}
                          {archived && (
                            <span className="text-[10px] text-[var(--text-muted)]">
                              {t("atlas.apis.statusArchived")}
                            </span>
                          )}
                          {revoked && (
                            <span className="text-[10px] text-red-400">
                              {t("atlas.apis.statusRevoked")}
                            </span>
                          )}
                          {!archived && !revoked && expired && (
                            <span className="text-[10px] text-red-400">
                              {t("atlas.apis.statusExpired")}
                            </span>
                          )}
                          {!archived && !revoked && !expired && exhausted && (
                            <span className="text-[10px] text-red-400">
                              {t("atlas.apis.statusExhausted")}
                            </span>
                          )}
                          {!dead && (
                            <span className="text-[10px] text-emerald-400">
                              {t("atlas.apis.statusLive")}
                            </span>
                          )}
                        </div>
                        <div className="text-[10px] text-[var(--text-faint)] mt-0.5">
                          {link.expires_at
                            ? `${t("atlas.apis.expiresLabel")} ${new Date(link.expires_at).toLocaleString()}`
                            : t("atlas.apis.expiryNever")}
                        </div>
                      </div>
                      <div className="flex items-center gap-1 shrink-0">
                        <select
                          aria-label={t("atlas.apis.renewExtend")}
                          value={ttl}
                          onChange={(e) =>
                            setRenewChoice((m) => ({ ...m, [link.id]: Number(e.target.value) }))
                          }
                          className="text-[10px] bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-1 py-0.5 text-[var(--text-secondary)]"
                        >
                          <option value={3600}>1h</option>
                          <option value={86400}>24h</option>
                          <option value={604800}>7d</option>
                        </select>
                        <Button
                          size="sm"
                          variant="secondary"
                          type="button"
                          onClick={() => renew.mutate({ id: link.id, ttl })}
                          disabled={renew.isPending}
                        >
                          {renew.isPending ? t("atlas.apis.renewing") : t("atlas.apis.renew")}
                        </Button>
                        {!dead && (
                          <Button
                            size="sm"
                            variant="danger"
                            type="button"
                            onClick={() => revoke.mutate(link.id)}
                            disabled={revoke.isPending}
                          >
                            {t("atlas.apis.revoke")}
                          </Button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}
    </Modal>
  );
}
