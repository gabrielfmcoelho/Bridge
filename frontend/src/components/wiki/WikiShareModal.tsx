"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Modal from "@/components/ui/Modal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import FormError from "@/components/ui/FormError";
import { shareBundlesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";

// A standalone share flow for wiki content — create a public share bundle that
// carries a single Outline document or collection (no API/secret involved).
// Reuses the same share-bundle plumbing as the per-API modal; the bundle item
// is keyed by the Outline UUID (ref_key), resolved live at redeem.
export type WikiShareTarget = {
  kind: "wiki_doc" | "wiki_collection";
  refKey: string;
  title: string;
};

export default function WikiShareModal({
  open,
  onClose,
  target,
}: {
  open: boolean;
  onClose: () => void;
  target: WikiShareTarget | null;
}) {
  const { t } = useLocale();
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [ttlHours, setTtlHours] = useState("24");
  const [passphrase, setPassphrase] = useState("");
  const [neverExpiry, setNeverExpiry] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [renewChoice, setRenewChoice] = useState<Record<number, number>>({});

  const linksKey = ["share-bundles", "wiki", target?.kind, target?.refKey] as const;
  const { data: existingLinks = [] } = useQuery({
    queryKey: linksKey,
    queryFn: () =>
      target ? shareBundlesAPI.listForItemKey(target.kind, target.refKey) : Promise.resolve([]),
    enabled: open && !!target,
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

  function reset() {
    setTitle("");
    setTtlHours("24");
    setPassphrase("");
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
    if (!target) return;
    setError(null);
    const ttl = Number(ttlHours);
    const ttlSeconds = neverExpiry ? -1 : ttl > 0 ? ttl * 3600 : undefined;
    setSubmitting(true);
    try {
      const res = await shareBundlesAPI.create({
        title: title.trim() || target.title,
        ttl_seconds: ttlSeconds,
        passphrase: passphrase.trim() || undefined,
        items: [{ type: target.kind, ref_key: target.refKey }],
      });
      setResult(`${window.location.origin}${res.url}`);
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

  return (
    <Modal open={open} onClose={close} title={t("atlas.apis.shareWikiTitle")}>
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
          <p className="text-xs text-[var(--text-muted)]">
            {target?.kind === "wiki_collection"
              ? t("share.wikiCollection")
              : t("share.wikiDoc")}
            {": "}
            <span className="text-[var(--text-secondary)]">{target?.title}</span>
          </p>

          <Input
            label={t("atlas.apis.shareTitleLabel")}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder={target?.title}
          />

          <div className="grid grid-cols-2 gap-3">
            <Input
              label={t("atlas.apis.ttlHours")}
              type="number"
              value={ttlHours}
              onChange={(e) => setTtlHours(e.target.value)}
              disabled={neverExpiry}
            />
            <Input
              label={t("atlas.apis.passphraseOptional")}
              type="password"
              value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
            />
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

          {error && <FormError message={error} />}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" type="button" onClick={close}>
              {t("common.cancel") || "Cancel"}
            </Button>
            <Button type="button" onClick={submit} loading={submitting} disabled={!target}>
              {submitting ? t("atlas.apis.creating") : t("atlas.apis.createLink")}
            </Button>
          </div>

          {/* Links already emitted for this doc/collection — renew or revoke. */}
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
                    link.expires_at != null && new Date(link.expires_at).getTime() < Date.now();
                  const revoked = link.revoked_at != null;
                  const exhausted =
                    link.max_views != null && link.view_count >= link.max_views;
                  const dead = archived || expired || revoked || exhausted;
                  const ttl = renewChoice[link.id] ?? 86400;
                  return (
                    <div
                      key={link.id}
                      className="text-xs p-2 rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]"
                    >
                      <div className="flex items-center justify-between gap-2">
                        <div className="min-w-0 flex-1">
                          {link.title && (
                            <div className="font-medium text-[var(--text-secondary)] truncate">
                              {link.title}
                            </div>
                          )}
                          <div className="flex items-center gap-2 flex-wrap">
                            <span className="text-[var(--text-secondary)]">
                              {link.view_count}
                              {link.max_views != null ? `/${link.max_views}` : ""}{" "}
                              {t("atlas.apis.views")}
                            </span>
                            {!dead ? (
                              <span className="text-[10px] text-emerald-400">
                                {t("atlas.apis.statusLive")}
                              </span>
                            ) : revoked ? (
                              <span className="text-[10px] text-red-400">
                                {t("atlas.apis.statusRevoked")}
                              </span>
                            ) : expired ? (
                              <span className="text-[10px] text-red-400">
                                {t("atlas.apis.statusExpired")}
                              </span>
                            ) : exhausted ? (
                              <span className="text-[10px] text-red-400">
                                {t("atlas.apis.statusExhausted")}
                              </span>
                            ) : (
                              <span className="text-[10px] text-[var(--text-muted)]">
                                {t("atlas.apis.statusArchived")}
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
