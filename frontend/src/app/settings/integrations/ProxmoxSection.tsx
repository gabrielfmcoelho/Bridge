"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { proxmoxAPI, type ProxmoxServer } from "@/lib/api";
import Button from "@/components/ui/Button";
import FormError from "@/components/ui/FormError";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import SectionCard from "@/components/ui/SectionCard";
import Toggle from "@/components/ui/Toggle";
import IntegrationCard from "./IntegrationCard";
import { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

// The card's toggle is the master switch (proxmox_enabled); each server is
// one independent cluster and saves on its own, not through the card's Save.
export default function ProxmoxSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("proxmox");
  const { data: servers } = useQuery({ queryKey: ["proxmox-servers"], queryFn: proxmoxAPI.listServers, retry: false });

  return (
    <IntegrationCard
      form={f}
      title="Proxmox VE"
      hint={t("settings.integrations.proxmox.hint")}
      enabledKey="proxmox_enabled"
      toggleLabel={t("settings.integrations.proxmox.ariaEnable")}
    >
      <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.proxmox.tokenHint")}</p>
      <SectionCard as="h3" variant="plain" title={t("settings.integrations.proxmox.serversTitle")} count={servers?.length ?? 0} className="pt-2">
        <ProxmoxServerList servers={servers ?? []} />
      </SectionCard>
    </IntegrationCard>
  );
}

type Draft = { name: string; base_url: string; token_id: string; token_secret: string; skip_verify: boolean };
const emptyDraft: Draft = { name: "", base_url: "", token_id: "", token_secret: "", skip_verify: false };

function ProxmoxServerList({ servers }: { servers: ProxmoxServer[] }) {
  const confirm = useConfirm();
  const { t } = useLocale();
  const queryClient = useQueryClient();
  // null = no form open; 0 = adding; otherwise the id being edited.
  const [editing, setEditing] = useState<number | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [error, setError] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<number, { ok: boolean; message: string }>>({});

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["proxmox-servers"] });
  const set = <K extends keyof Draft>(k: K, v: Draft[K]) => setDraft((d) => ({ ...d, [k]: v }));
  const close = () => { setEditing(null); setDraft(emptyDraft); setError(null); };

  const save = useMutation({
    // An empty token_secret on edit keeps the stored one.
    mutationFn: () => editing ? proxmoxAPI.updateServer(editing, draft) : proxmoxAPI.createServer(draft),
    onSuccess: () => { close(); invalidate(); },
    onError: (err: Error) => setError(err.message),
  });
  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) => proxmoxAPI.updateServer(id, { enabled }),
    onSettled: invalidate,
  });
  const remove = useMutation({ mutationFn: (id: number) => proxmoxAPI.deleteServer(id), onSuccess: invalidate });
  const test = useMutation({
    mutationFn: (id: number) => proxmoxAPI.test({ server_id: id }).then((res) => ({ id, res })),
    onSuccess: ({ id, res }) => setTestResults((prev) => ({
      ...prev,
      [id]: {
        ok: res.success,
        message: res.success
          ? t("settings.integrations.proxmox.connectedVersion", { version: res.version ?? "" })
          : failedLine(t, res.error),
      },
    })),
  });

  const form = (
    <div className="rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] p-3 space-y-3">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input label={t("common.name")} value={draft.name} onChange={(e) => set("name", e.target.value)} placeholder={t("settings.integrations.proxmox.serverNamePlaceholder")} />
        <Input label={t("settings.integrations.baseUrl")} value={draft.base_url} onChange={(e) => set("base_url", e.target.value)} placeholder="https://pve.example.gov.br:8006" />
        <Input label={t("settings.integrations.proxmox.tokenId")} value={draft.token_id} onChange={(e) => set("token_id", e.target.value)} placeholder="bridge@pve!sync" />
        <Input
          label={t("settings.integrations.proxmox.tokenSecret")}
          type="password"
          value={draft.token_secret}
          onChange={(e) => set("token_secret", e.target.value)}
          placeholder={editing ? t("settings.integrations.proxmox.keepSecret") : ""}
        />
        <NativeSelect label={t("settings.integrations.ldap.skipVerify")} value={String(draft.skip_verify)} onChange={(e) => set("skip_verify", e.target.value === "true")}>
          <option value="false">{t("common.no")}</option>
          <option value="true">{t("common.yes")}</option>
        </NativeSelect>
      </div>
      {error && <FormError message={error} />}
      <div className="flex gap-2">
        <Button
          type="button"
          size="sm"
          onClick={() => save.mutate()}
          loading={save.isPending}
          disabled={!draft.name.trim() || !draft.base_url.trim() || !draft.token_id.trim() || (!editing && !draft.token_secret.trim())}
        >
          {editing ? t("common.save") : t("common.add")}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={close}>{t("common.cancel")}</Button>
      </div>
    </div>
  );

  return (
    <div className="space-y-3">
      {servers.length === 0 && editing === null && (
        <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.proxmox.emptyServers")}</p>
      )}

      {servers.map((s) => editing === s.id ? <div key={s.id}>{form}</div> : (
        <div key={s.id} className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] px-3 py-2">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <div className="flex items-center gap-3 min-w-0">
              <Toggle
                checked={s.enabled}
                disabled={toggle.isPending}
                onChange={(enabled) => toggle.mutate({ id: s.id, enabled })}
                ariaLabel={t("settings.integrations.proxmox.ariaEnableServer", { name: s.name })}
              />
              <div className="min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">{s.name}</p>
                <p className="text-xs text-[var(--text-muted)] font-mono truncate">{s.base_url} · {s.token_id}</p>
                <p className="text-2xs text-[var(--text-muted)] mt-0.5">
                  {s.has_token ? t("settings.integrations.proxmox.tokenStored") : t("settings.integrations.proxmox.noToken")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button type="button" variant="ghost" size="sm" disabled={test.isPending} onClick={() => test.mutate(s.id)}>
                {t("settings.integrations.testConnection")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  setEditing(s.id);
                  setDraft({ name: s.name, base_url: s.base_url, token_id: s.token_id, token_secret: "", skip_verify: s.skip_verify });
                  setError(null);
                }}
              >
                {t("common.edit")}
              </Button>
              <Button
                type="button"
                variant="danger"
                size="sm"
                onClick={async () => {
                  if (!(await confirm({ title: t("confirm.deleteTitle", { name: `"${s.name}"` }), message: t("settings.integrations.proxmox.confirmDeleteServer"), danger: true, confirmLabel: t("common.delete") }))) return;
                  remove.mutate(s.id);
                }}
              >
                {t("common.delete")}
              </Button>
            </div>
          </div>
          {testResults[s.id] && (
            <p className={`text-xs mt-1 ${testResults[s.id].ok ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
              {testResults[s.id].message}
            </p>
          )}
        </div>
      ))}

      {editing === 0 ? form : editing === null && (
        <Button type="button" size="sm" variant="secondary" onClick={() => { setDraft(emptyDraft); setEditing(0); }}>
          {t("settings.integrations.proxmox.addServer")}
        </Button>
      )}
    </div>
  );
}
