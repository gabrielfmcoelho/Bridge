"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import PillButton from "@/components/ui/PillButton";
import FormField from "@/components/ui/FormField";
import FormError from "@/components/ui/FormError";
import StatusAlert from "@/components/ui/StatusAlert";
import SectionHeading from "@/components/ui/SectionHeading";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import RecipientField, { EMPTY_RECIPIENT, recipientPayload } from "@/components/share/RecipientField";

type TTL = "1h" | "24h" | "7d" | "custom";
const TTL_SECONDS: Record<Exclude<TTL, "custom">, number> = { "1h": 3600, "24h": 86400, "7d": 604800 };

// Links open on the configured public URL when there is one (the app may sit
// behind another host), else on this origin.
const shareOrigin = () => process.env.NEXT_PUBLIC_BASE_URL || window.location.origin;

/**
 * External share links for one secret: create one (expiry, optional
 * passphrase, view limit and recipient) and see or revoke the existing ones.
 * The URL is shown right after creating; later only an admin can reveal it
 * again (on /shares, audited).
 */
export default function ShareLinkDrawer({ secretID, onClose }: { secretID: number | null; onClose: () => void }) {
  const { t, locale } = useLocale();
  const qc = useQueryClient();
  const open = secretID != null;
  const [ttl, setTTL] = useState<TTL>("24h");
  const [customHours, setCustomHours] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [maxViews, setMaxViews] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [recipient, setRecipient] = useState(EMPTY_RECIPIENT);
  const [created, setCreated] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");

  const key = ["share-links", secretID];
  const { data: links = [] } = useQuery({ queryKey: key, queryFn: () => secretsAPI.listShareLinks(secretID as number), enabled: open });
  const onError = (e: unknown) => setError(e instanceof Error ? e.message : t("form.saveFailed"));

  const create = useMutation({
    mutationFn: () => secretsAPI.createShareLink(secretID as number, {
      ttl_seconds: ttl === "custom" ? Math.round((Number(customHours) || 0) * 3600) : TTL_SECONDS[ttl],
      max_views: Number(maxViews) || 0,
      passphrase: passphrase || undefined,
      title: title.trim() || undefined,
      description: description.trim() || undefined,
      ...recipientPayload(recipient),
    }),
    onSuccess: (res) => {
      setCreated(shareOrigin() + res.url);
      setCopied(false);
      setError("");
      setPassphrase(""); setMaxViews(""); setTitle(""); setDescription(""); setRecipient(EMPTY_RECIPIENT);
      qc.invalidateQueries({ queryKey: key });
    },
    onError,
  });
  const revoke = useMutation({
    mutationFn: (linkID: number) => secretsAPI.revokeShareLink(secretID as number, linkID),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
    onError,
  });

  const close = () => { setCreated(null); setError(""); onClose(); };
  const copy = async () => {
    if (!created) return;
    try { await navigator.clipboard.writeText(created); setCopied(true); } catch { /* select and copy by hand */ }
  };

  return (
    <Drawer open={open} onClose={close} title={t("share.drawerTitle")}
      footer={<div className="flex justify-end"><Button loading={create.isPending} disabled={ttl === "custom" && !Number(customHours)} onClick={() => create.mutate()}>{t("share.createLink")}</Button></div>}>
      <div className="space-y-6">
        <FormError message={error} />
        {created && (
          <StatusAlert variant="success">
            <p className="font-medium">{t("share.linkCreated")}</p>
            <p className="text-xs mt-0.5">{t("share.copyNow")}</p>
            <div className="mt-2 flex items-center gap-2">
              <code className="flex-1 min-w-0 truncate text-xs font-mono px-2 py-1 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-[var(--text-secondary)]">{created}</code>
              <Button size="sm" variant="secondary" onClick={copy}>{copied ? t("vault.copiedLabel") : t("common.copy")}</Button>
            </div>
          </StatusAlert>
        )}

        <section className="space-y-4">
          <SectionHeading as="h3" className="!mb-0">{t("share.newLink")}</SectionHeading>
          <Input label={t("share.nickname")} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("share.nicknamePlaceholder")} />
          <RecipientField value={recipient} onChange={setRecipient} enabled={open} />
          <Textarea label={t("common.description")} rows={2} value={description} onChange={(e) => setDescription(e.target.value)} hint={t("share.descriptionHint")} />
          <FormField label={t("share.expiresAfter")}>
            <div className="flex flex-wrap gap-1.5">
              {(["1h", "24h", "7d", "custom"] as const).map((p) => (
                <PillButton key={p} size="sm" active={ttl === p} onClick={() => setTTL(p)}>{t(`share.ttl.${p}`)}</PillButton>
              ))}
            </div>
            {ttl === "custom" && (
              <div className="mt-2 max-w-[12rem]">
                <Input aria-label={t("share.customHours")} type="number" min={1} value={customHours} onChange={(e) => setCustomHours(e.target.value)} placeholder={t("share.customHours")} />
              </div>
            )}
          </FormField>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Input label={t("share.passphrase")} type="password" autoComplete="new-password" value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)} hint={t("share.passphraseHint")} />
            <Input label={t("share.maxViews")} type="number" min={1} value={maxViews}
              onChange={(e) => setMaxViews(e.target.value)} hint={t("share.maxViewsHint")} />
          </div>
        </section>

        <section className="space-y-2">
          <SectionHeading as="h3" className="!mb-0" count={links.length}>{t("share.existingLinks")}</SectionHeading>
          {links.length === 0 ? <p className="text-sm text-[var(--text-muted)]">{t("share.noLinks")}</p> : (
            <div className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
              <RowList>
                {links.map((l) => {
                  const revoked = l.revoked_at != null;
                  const expired = new Date(l.expires_at) < new Date();
                  const exhausted = l.max_views != null && l.view_count >= l.max_views;
                  const state = revoked ? "revoked" : expired ? "expired" : exhausted ? "exhausted" : "";
                  return (
                    <ListRow key={l.id}>
                      <RowText
                        title={t("share.views", { count: String(l.view_count), max: l.max_views != null ? `/${l.max_views}` : "" })}
                        meta={<>
                          <span>{t(expired ? "share.expiredOn" : "share.expiresOn", { when: new Date(l.expires_at).toLocaleString(locale, { dateStyle: "short", timeStyle: "short" }) })}</span>
                          {l.has_passphrase && <span>{t("share.withPassphrase")}</span>}
                        </>}
                      />
                      {state ? (
                        <span className="text-xs text-[var(--danger)]">{t(`share.state.${state}`)}</span>
                      ) : (
                        <Button size="sm" variant="ghost" loading={revoke.isPending && revoke.variables === l.id} onClick={() => revoke.mutate(l.id)}>{t("share.revoke")}</Button>
                      )}
                    </ListRow>
                  );
                })}
              </RowList>
            </div>
          )}
        </section>
      </div>
    </Drawer>
  );
}
