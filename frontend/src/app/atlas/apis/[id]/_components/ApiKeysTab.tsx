"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiKeysAPI } from "@/lib/api";
import type { ApiCatalog, ApiKey } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import { useSecretReveal } from "@/hooks/useSecretReveal";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import Button from "@/components/ui/Button";
import CopyButton from "@/components/ui/CopyButton";
import Lozenge from "@/components/ui/Lozenge";
import Modal from "@/components/ui/Modal";
import RowActions from "@/components/ui/RowActions";
import StatusAlert from "@/components/ui/StatusAlert";
import Toggle from "@/components/ui/Toggle";
import EmptyState from "@/components/ui/EmptyState";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import SecretValue from "@/components/vault/SecretValue";
import { ICON_PATHS } from "@/lib/icon-paths";
import ApiKeyDrawer from "./ApiKeyDrawer";
import KeyManagementDrawer from "./KeyManagementDrawer";

const STATUS_APPEARANCE = { active: "success", grace: "moved", expired: "moved", revoked: "removed" } as const;

// Chaves de acesso: who holds a key to this API. Listing needs only to see
// the API; issuing, rotating, revoking and reading a key back need
// apis.keys.manage; the SEAD connection is admin-only.
export default function ApiKeysTab({ api, canManage, isAdmin }: { api: ApiCatalog; canManage: boolean; isAdmin: boolean }) {
  const { t, formatDate, formatDateTime } = useLocale();
  const confirm = useConfirm();
  const flag = useFlag();
  const qc = useQueryClient();
  const [showRevoked, setShowRevoked] = useState(false);
  const [drawer, setDrawer] = useState<{ editing: ApiKey | null } | null>(null);
  const [configuring, setConfiguring] = useState(false);
  const [issued, setIssued] = useState<{ label: string; plaintext: string } | null>(null);
  const [revealing, setRevealing] = useState<ApiKey | null>(null);
  const [usageOf, setUsageOf] = useState<ApiKey | null>(null);

  const mode = api.key_management;
  const sead = mode === "sead";
  const { data: keys = [], error: loadError } = useQuery({
    queryKey: ["api-keys", api.id, showRevoked],
    queryFn: () => apiKeysAPI.list(api.id, showRevoked),
    enabled: mode !== "none",
  });
  const refresh = () => qc.invalidateQueries({ queryKey: ["api-keys", api.id] });
  const fail = (title: string) => (err: unknown) =>
    flag({ appearance: "error", title, description: err instanceof Error ? err.message : undefined });

  const sync = useMutation({
    mutationFn: () => apiKeysAPI.sync(api.id),
    onSuccess: (r) => {
      refresh();
      flag({ appearance: "success", title: t("atlas.apis.keys.synced", { created: String(r.created), total: String(r.total) }) });
    },
    onError: fail(t("atlas.apis.keys.syncFailed")),
  });
  const revoke = useMutation({
    mutationFn: (k: ApiKey) => apiKeysAPI.revoke(api.id, k.id),
    onSuccess: () => { refresh(); flag({ appearance: "success", title: t("atlas.apis.keys.revoked") }); },
    onError: fail(t("atlas.apis.keys.revokeFailed")),
  });
  const rotate = useMutation({
    mutationFn: (k: ApiKey) => apiKeysAPI.rotate(api.id, k.id, 7),
    onSuccess: (r) => { refresh(); setIssued({ label: r.key.label, plaintext: r.plaintext }); },
    onError: fail(t("atlas.apis.keys.rotateFailed")),
  });

  const askRevoke = async (k: ApiKey) => {
    if (await confirm({ title: t("atlas.apis.keys.revokeConfirm", { name: k.label }), message: t("atlas.apis.keys.revokeConfirmBody"), danger: true, confirmLabel: t("atlas.apis.keys.revoke") })) {
      revoke.mutate(k);
    }
  };
  const askRotate = async (k: ApiKey) => {
    if (await confirm({ title: t("atlas.apis.keys.rotateConfirm", { name: k.label }), message: t("atlas.apis.keys.rotateConfirmBody"), confirmLabel: t("atlas.apis.keys.rotate") })) {
      rotate.mutate(k);
    }
  };

  const configButton = isAdmin && (
    <IconButton label={t("atlas.apis.keys.configure")} onClick={() => setConfiguring(true)}><Icon path={ICON_PATHS.gear} /></IconButton>
  );

  if (mode === "none") {
    return (
      <>
        <EmptyState
          compact
          icon="key"
          title={t("atlas.apis.keys.disabledTitle")}
          description={t("atlas.apis.keys.disabledDesc")}
          action={isAdmin ? <Button size="sm" variant="secondary" onClick={() => setConfiguring(true)}>{t("atlas.apis.keys.configure")}</Button> : undefined}
        />
        {configuring && <KeyManagementDrawer api={api} open onClose={() => setConfiguring(false)} />}
      </>
    );
  }

  return (
    <>
      {issued && (
        <div className="space-y-3">
          <StatusAlert variant="warning">{t("atlas.apis.keys.copyNow", { name: issued.label })}</StatusAlert>
          <div className="flex flex-col sm:flex-row gap-2 sm:items-center">
            <code className="flex-1 min-w-0 break-all font-mono text-xs px-3 py-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-default)] text-[var(--text-primary)]">
              {issued.plaintext}
            </code>
            <div className="flex gap-2 shrink-0">
              <CopyButton value={issued.plaintext} icon />
              <Button variant="ghost" onClick={() => setIssued(null)}>{t("atlas.apis.keys.done")}</Button>
            </div>
          </div>
        </div>
      )}

      {sead && !api.has_admin_key && (
        <StatusAlert variant="warning">{t("atlas.apis.keys.notConnected")}</StatusAlert>
      )}
      {!!loadError && <StatusAlert variant="error">{(loadError as Error).message}</StatusAlert>}

      <SectionCard
        as="h3"
        title={t("atlas.apis.keys.title")}
        description={t(`atlas.apis.keys.modeHint.${mode}`)}
        count={keys.length}
        body="flush"
        empty={keys.length === 0 ? t("atlas.apis.keys.empty") : undefined}
        controls={
          <span className="inline-flex items-center gap-2">
            <Toggle checked={showRevoked} onChange={setShowRevoked} ariaLabel={t("atlas.apis.keys.showRevoked")} />
            <span className="hidden sm:inline text-xs text-[var(--text-secondary)]">{t("atlas.apis.keys.showRevoked")}</span>
            {sead && canManage && (
              <IconButton label={t("atlas.apis.keys.sync")} onClick={() => sync.mutate()} disabled={sync.isPending}><Icon path={ICON_PATHS.refresh} /></IconButton>
            )}
            {configButton}
            {canManage && (
              <IconButton label={t(sead ? "atlas.apis.keys.issue" : "atlas.apis.keys.register")} onClick={() => setDrawer({ editing: null })}><Icon path={ICON_PATHS.plus} /></IconButton>
            )}
          </span>
        }
      >
        <RowList>
          {keys.map((k) => {
            const meta = [
              k.owner_contact_name || k.owner || t("atlas.apis.keys.noOwner"),
              k.expires_at ? t("atlas.apis.keys.expiresOn", { date: formatDate(k.expires_at) }) : t("atlas.apis.keys.noExpiry"),
              k.last_used_at ? t("atlas.apis.keys.lastUsedAt", { date: formatDateTime(k.last_used_at) }) : t("atlas.apis.keys.neverUsed"),
              ...(k.source === "sead" ? [t("atlas.apis.keys.uses", { count: String(k.lifetime_uses) })] : []),
            ];
            return (
              <ListRow key={k.id}>
                <Icon path={ICON_PATHS.keyOutline} className="w-3.5 h-3.5 shrink-0 text-[var(--accent)]" />
                <RowText title={k.label} mono meta={meta.map((m, i) => <span key={i}>{m}</span>)} />
                <span className="hidden sm:inline text-2xs text-[var(--text-muted)]">{t(`atlas.apis.keys.source.${k.source}`)}</span>
                <Lozenge appearance={STATUS_APPEARANCE[k.status]}>{t(`atlas.apis.keys.status.${k.status}`)}</Lozenge>
                <RowActions
                  name={k.label}
                  actions={[
                    { label: t("atlas.apis.keys.reveal"), icon: ICON_PATHS.eye, hidden: !canManage || !k.secret_id || k.status === "revoked", onClick: () => setRevealing(k) },
                    { label: t("atlas.apis.keys.usage"), icon: ICON_PATHS.clock, hidden: k.source !== "sead", onClick: () => setUsageOf(k) },
                    { label: t("atlas.apis.keys.rotate"), icon: ICON_PATHS.refresh, hidden: !canManage || k.source !== "sead" || k.status !== "active", onClick: () => askRotate(k) },
                    { label: t("common.edit"), icon: ICON_PATHS.pencil, hidden: !canManage, onClick: () => setDrawer({ editing: k }) },
                    { label: t("atlas.apis.keys.revoke"), icon: ICON_PATHS.trash, danger: true, hidden: !canManage || k.status === "revoked", onClick: () => askRevoke(k) },
                  ]}
                />
              </ListRow>
            );
          })}
        </RowList>
      </SectionCard>

      {drawer && (
        <ApiKeyDrawer api={api} editing={drawer.editing} open onClose={() => setDrawer(null)}
          onIssued={(label, plaintext) => setIssued({ label, plaintext })} />
      )}
      {configuring && <KeyManagementDrawer api={api} open onClose={() => setConfiguring(false)} />}
      {revealing && <RevealKeyModal apiKey={revealing} onClose={() => setRevealing(null)} />}
      {usageOf && <KeyUsageModal api={api} apiKey={usageOf} onClose={() => setUsageOf(null)} />}
    </>
  );
}

// RevealKeyModal reads the key back from the vault (audited, auto-hides).
function RevealKeyModal({ apiKey, onClose }: { apiKey: ApiKey; onClose: () => void }) {
  const { t } = useLocale();
  const reveal = useSecretReveal();
  const { reveal: doReveal } = reveal;
  useEffect(() => {
    if (apiKey.secret_id) doReveal(apiKey.secret_id);
  }, [apiKey.secret_id, doReveal]);
  return (
    <Modal open onClose={onClose} title={t("atlas.apis.keys.revealTitle", { name: apiKey.label })}>
      <div className="space-y-3">
        {reveal.loading && <p className="text-sm text-[var(--text-muted)]">{t("common.loading")}</p>}
        {reveal.error && <StatusAlert variant="error">{reveal.error}</StatusAlert>}
        <SecretValue type="api_key" reveal={reveal} />
        {reveal.revealed && (
          <p className="text-xs text-[var(--text-muted)]">{t("atlas.apis.keys.revealHint", { seconds: String(Math.ceil(reveal.remainingMs / 1000)) })}</p>
        )}
      </div>
    </Modal>
  );
}

// KeyUsageModal shows a SEAD key's requests per day, straight from the service.
function KeyUsageModal({ api, apiKey, onClose }: { api: ApiCatalog; apiKey: ApiKey; onClose: () => void }) {
  const { t } = useLocale();
  const { data, error, isLoading } = useQuery({
    queryKey: ["api-keys", api.id, "usage", apiKey.id],
    queryFn: () => apiKeysAPI.usage(api.id, apiKey.id, 30),
    retry: false,
  });
  const days = Object.entries(data?.daily ?? {}).sort(([a], [b]) => b.localeCompare(a));
  const max = Math.max(1, ...days.map(([, n]) => n));
  const label = (d: string) => `${d.slice(6, 8)}/${d.slice(4, 6)}`;
  return (
    <Modal open onClose={onClose} title={t("atlas.apis.keys.usageTitle", { name: apiKey.label })}>
      <div className="space-y-3">
        {isLoading && <p className="text-sm text-[var(--text-muted)]">{t("common.loading")}</p>}
        {!!error && <StatusAlert variant="error">{(error as Error).message}</StatusAlert>}
        {data && (
          <>
            <p className="text-sm text-[var(--text-secondary)]">{t("atlas.apis.keys.usageLifetime", { count: String(data.lifetime) })}</p>
            {days.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">{t("atlas.apis.keys.usageNone")}</p>
            ) : (
              <ul className="space-y-1">
                {days.map(([d, n]) => (
                  <li key={d} className="flex items-center gap-3 text-xs">
                    <span className="w-12 shrink-0 font-mono text-[var(--text-muted)]">{label(d)}</span>
                    <span className="flex-1 h-2 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
                      <span className="block h-full bg-[var(--accent)]" style={{ width: `${(n / max) * 100}%` }} />
                    </span>
                    <span className="w-12 shrink-0 text-right tabular-nums text-[var(--text-secondary)]">{n}</span>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </Modal>
  );
}
