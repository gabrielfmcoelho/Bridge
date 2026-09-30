"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useSecretReveal } from "@/hooks/useSecretReveal";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import Field from "@/components/ui/Field";
import SectionHeading from "@/components/ui/SectionHeading";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import FormError from "@/components/ui/FormError";
import StatusAlert from "@/components/ui/StatusAlert";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import RelationPicker from "@/components/forms/RelationPicker";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";
import type { Secret } from "@/lib/types";

/** Where a secret lives, in words: "Serviço · api-gateway", "Avulso". */
export function scopeLabel(s: Secret, t: (k: string) => string) {
  const scope = t(`vault.scope.${s.scope}`);
  return s.parent_name ? `${scope} · ${s.parent_name}` : scope;
}

/**
 * One secret, everything about it in one place: its context, the value
 * (revealed on demand, hidden again after a while), the hosts using it when
 * it's a host credential, and its history. Actions in the footer.
 */
export default function SecretDetailDrawer({ secret, onClose, onEdit, onShare, canWrite, canDelete }: {
  secret: Secret | null;
  onClose: () => void;
  onEdit: (s: Secret) => void;
  onShare: (s: Secret) => void;
  canWrite: boolean;
  canDelete: boolean;
}) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const open = !!secret;
  const [error, setError] = useState("");

  const del = useMutation({
    mutationFn: () => secretsAPI.delete(secret!.id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["secrets-all"] }); onClose(); },
    onError: (e) => setError(e instanceof Error ? e.message : t("form.saveFailed")),
  });

  const footer = secret && (
    <div className="flex items-center gap-2">
      {canDelete && (
        <Button variant="ghost" size="sm" loading={del.isPending}
          onClick={async () => {
            if (await confirm({ title: t("confirm.deleteTitle", { name: `"${secret.name}"` }), message: t("vault.deleteHint"), danger: true, confirmLabel: t("common.delete") })) del.mutate();
          }}>
          {t("common.delete")}
        </Button>
      )}
      <div className="ml-auto flex items-center gap-2">
        <Button variant="secondary" onClick={() => onShare(secret)}>{t("common.share")}</Button>
        {canWrite && <Button onClick={() => onEdit(secret)}>{t("common.edit")}</Button>}
      </div>
    </div>
  );

  return (
    <Drawer open={open} onClose={onClose} title={secret?.name ?? ""} footer={footer || undefined}>
      {secret && <DetailBody key={secret.id} secret={secret} canWrite={canWrite} error={error} t={t} />}
    </Drawer>
  );
}

function DetailBody({ secret, canWrite, error, t }: { secret: Secret; canWrite: boolean; error: string; t: (k: string, v?: Record<string, string>) => string }) {
  const { locale } = useLocale();
  const reveal = useSecretReveal();
  const hostCred = secret.scope === "avulso" && (secret.type === "password" || secret.type === "sshkey");

  return (
    <div className="space-y-6">
      <FormError message={error} />
      <div className="grid grid-cols-2 gap-4">
        <Field label={t("secretForm.type")} value={t(`vault.type.${secret.type}`)} />
        <Field label={t("secretForm.scope")} value={scopeLabel(secret, t)} />
        <Field label={t("secretForm.visibility")} value={t(`vault.visibility.${secret.visibility}`)} />
        <Field label={t("share.fields.username")} value={secret.username ?? ""} mono />
        {secret.group_label && <Field label={t("vault.groupLabel")} value={secret.group_label} mono />}
        <Field label={t("vault.owner")} value={secret.owner_name ?? ""} />
        <Field label={t("vault.updated")} value={getTimeAgo(secret.updated_at, locale)} />
        {secret.ssh_fingerprint && <Field className="col-span-2" label={t("vault.fingerprint")} value={secret.ssh_fingerprint} mono />}
        {secret.description && <Field className="col-span-2" label={t("common.description")} value={secret.description} />}
      </div>

      <section className="space-y-2">
        <SectionHeading as="h3" className="!mb-0">{t("vault.value")}</SectionHeading>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="secondary" loading={reveal.loading}
            onClick={() => (reveal.revealed ? reveal.hide() : reveal.reveal(secret.id))}>
            <Icon path={reveal.revealed ? ICON_PATHS.eyeOff : ICON_PATHS.eye} className="w-3.5 h-3.5" />
            {reveal.revealed ? t("vault.hideIn", { s: String(Math.ceil(reveal.remainingMs / 1000)) }) : t("serviceCredentials.reveal")}
          </Button>
          {reveal.revealed && (
            <Button size="sm" variant="secondary" onClick={() => reveal.copy()}>
              <Icon path={ICON_PATHS.copy} className="w-3.5 h-3.5" />
              {reveal.copyState === "copied" ? t("vault.copiedLabel") : reveal.copyState === "cleared" ? t("vault.clearedLabel") : t("common.copy")}
            </Button>
          )}
        </div>
        {reveal.revealed && (
          <pre className="text-xs font-mono text-[var(--text-secondary)] bg-[var(--bg-elevated)] rounded-[var(--radius-md)] p-3 overflow-x-auto whitespace-pre-wrap break-all border border-[var(--border-subtle)]">
            {reveal.value}
          </pre>
        )}
        {reveal.error && <p className="text-xs text-[var(--danger)]">{reveal.error}</p>}
      </section>

      {(secret.dup_count ?? 0) > 1 && (
        <StatusAlert variant="info">
          <strong className="font-medium">{t("vault.repeatedTitle", { count: String(secret.dup_count) })}</strong> {t("vault.repeatedBody")}
        </StatusAlert>
      )}

      {hostCred && <LinkedHosts secret={secret} canWrite={canWrite} t={t} />}

      <History secretId={secret.id} t={t} />
    </div>
  );
}

function LinkedHosts({ secret, canWrite, t }: { secret: Secret; canWrite: boolean; t: (k: string, v?: Record<string, string>) => string }) {
  const qc = useQueryClient();
  const key = ["secret-linked-hosts", secret.id];
  const { data } = useQuery({ queryKey: key, queryFn: () => secretsAPI.listLinkedHosts(secret.id) });
  const options = useRelationOptions(["hosts"]);
  const [adding, setAdding] = useState<number[]>([]);
  const [error, setError] = useState("");
  const linked = new Set(data?.host_ids ?? []);
  const done = () => {
    qc.invalidateQueries({ queryKey: key });
    qc.invalidateQueries({ queryKey: ["secrets-all"] });
  };
  const onError = (e: unknown) => setError(e instanceof Error ? e.message : t("form.saveFailed"));
  const link = useMutation({ mutationFn: () => secretsAPI.linkHosts(secret.id, adding), onSuccess: () => { setAdding([]); done(); }, onError });
  const unlink = useMutation({ mutationFn: (hostId: number) => secretsAPI.unlinkHost(secret.id, hostId), onSuccess: done, onError });

  return (
    <section className="space-y-2">
      <SectionHeading as="h3" className="!mb-0" hint={t("vault.hostsHint")}>{t("vault.hostsUsing", { count: String(linked.size) })}</SectionHeading>
      <FormError message={error} />
      {linked.size > 0 && (
        <div className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
          <RowList>
            {options.hosts.filter((h) => linked.has(h.id)).map((h) => (
              <ListRow key={h.id}>
                <Icon path={ICON_PATHS.server} className="w-3.5 h-3.5 shrink-0 text-[var(--text-muted)]" />
                <RowText title={h.label} meta={h.secondary} />
                {canWrite && (
                  <IconButton label={t("vault.unlinkHost", { name: h.label })} onClick={() => unlink.mutate(h.id)}>
                    <Icon path={ICON_PATHS.close} />
                  </IconButton>
                )}
              </ListRow>
            ))}
          </RowList>
        </div>
      )}
      {canWrite && (
        <div className="space-y-2">
          <RelationPicker label={t("vault.linkMoreHosts")} options={options.hosts.filter((h) => !linked.has(h.id))}
            selected={adding} onChange={setAdding} placeholder={t("secretForm.searchHosts")} />
          {adding.length > 0 && (
            <Button size="sm" loading={link.isPending} onClick={() => link.mutate()}>
              {t("vault.linkHosts", { count: String(adding.length) })}
            </Button>
          )}
        </div>
      )}
    </section>
  );
}

const ACTION_TONE: Record<string, string> = {
  create: "var(--success)", restore: "var(--success)", update: "var(--warning)", reveal: "var(--info)",
  delete: "var(--danger)", share_revoke: "var(--danger)", share_create: "var(--accent)", share_redeem: "var(--accent)",
};

function History({ secretId, t }: { secretId: number; t: (k: string, v?: Record<string, string>) => string }) {
  const { locale } = useLocale();
  const [open, setOpen] = useState(false);
  const { data = [], isLoading } = useQuery({ queryKey: ["secret-history", secretId], queryFn: () => secretsAPI.history(secretId), enabled: open });
  return (
    <section className="space-y-2">
      <button type="button" onClick={() => setOpen((v) => !v)} aria-expanded={open}
        className="flex items-center gap-1.5 text-heading-xs font-semibold font-display text-[var(--text-primary)]">
        <Icon path={ICON_PATHS.chevronRight} className={`w-3.5 h-3.5 text-[var(--text-muted)] transition-transform ${open ? "rotate-90" : ""}`} />
        {t("vault.historyButton")}
      </button>
      {open && (isLoading ? <p className="text-xs text-[var(--text-muted)]">{t("common.loading")}</p> : data.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)]">{t("vault.historyEmpty")}</p>
      ) : (
        <ol className="space-y-1.5">
          {data.map((e) => (
            <li key={e.id} className="flex items-baseline gap-2 text-xs">
              <span className="w-2 h-2 rounded-full shrink-0 translate-y-[-1px]" style={{ background: ACTION_TONE[e.action] ?? "var(--text-muted)" }} />
              <span className="text-[var(--text-primary)]">{t(`vault.action.${e.action}`)}</span>
              <span className="text-[var(--text-muted)]">{e.actor_user_id ? `#${e.actor_user_id}` : ""}</span>
              <span className="ml-auto text-[var(--text-muted)] tabular-nums">{getTimeAgo(e.at, locale)}</span>
            </li>
          ))}
        </ol>
      ))}
    </section>
  );
}
