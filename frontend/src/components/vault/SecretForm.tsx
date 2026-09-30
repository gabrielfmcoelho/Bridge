"use client";

import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI, servicesAPI, toolsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import Drawer from "@/components/ui/Drawer";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import Select from "@/components/ui/Select";
import Checkbox from "@/components/ui/Checkbox";
import RadioGroup from "@/components/ui/RadioGroup";
import FormField from "@/components/ui/FormField";
import TabBar from "@/components/ui/TabBar";
import Button from "@/components/ui/Button";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import EntityFormShell from "@/components/forms/EntityFormShell";
import FormSection from "@/components/forms/FormSection";
import RelationPicker from "@/components/forms/RelationPicker";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import { parseDotenv } from "@/lib/parseDotenv";
import { buildEnvTargets } from "@/lib/envTargets";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { AssetGrantsInput } from "@/lib/types";

export type SecretType = "password" | "cred" | "sshkey" | "app_login" | "env_var";
export type SecretScope = "service" | "host" | "tool" | "projeto" | "avulso";
type Visibility = "personal" | "shared";
interface EnvRow { name: string; value: string; description: string }

// Select always offers "--" (clear); the required pickers below ignore it.
const TYPES: SecretType[] = ["password", "cred", "sshkey", "app_login", "env_var"];
const TYPE_KEY: Record<SecretType, string> = {
  password: "typePassword", cred: "typeCred", sshkey: "typeSshkey", app_login: "typeAppLogin", env_var: "typeEnvVar",
};
const SCOPES: SecretScope[] = ["avulso", "projeto", "service", "host", "tool"];
const SCOPE_KEY: Record<SecretScope, string> = {
  avulso: "scopeAvulso", service: "scopeService", host: "scopeHost", tool: "scopeTool", projeto: "scopeProjeto",
};
const GROUP_RE = /^[a-z][a-z0-9-]*$/;
const VAR_RE = /^[A-Z_][A-Z0-9_]*$/;
const emptyRow = (): EnvRow => ({ name: "", value: "", description: "" });

/**
 * Create a secret — the sectioned drawer form hosts, DNS and serviços use:
 * what to keep (type + its content), where it lives (the asset it's attached
 * to), who sees it. Opened from an asset (defaultScope + defaultParentId) the
 * attachment is fixed and shown, not asked.
 */
export default function SecretForm({ defaultScope, defaultParentId, onDone, onCancel, onSubHeaderChange, onFooterChange }: {
  defaultScope?: SecretScope;
  defaultParentId?: number;
  onDone: () => void;
  onCancel: () => void;
  onSubHeaderChange?: (n: ReactNode) => void;
  onFooterChange?: (n: ReactNode) => void;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const qc = useQueryClient();
  const fixed = defaultParentId != null;

  const [type, setType] = useState<SecretType>("password");
  const [scope, setScope] = useState<SecretScope>(defaultScope ?? "avulso");
  const [parentId, setParentId] = useState<string>(fixed ? String(defaultParentId) : "");
  const [visibility, setVisibility] = useState<Visibility>(fixed ? "shared" : "personal");
  const [grants, setGrants] = useState<AssetGrantsInput>(() => defaultGrants(user));
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  // Per-type content.
  const [f, setF] = useState({ value: "", username: "", password: "", sshUser: "", privKey: "", pubKey: "", appName: "", appUrl: "", notes: "" });
  const setField = (k: keyof typeof f, v: string) => setF((x) => ({ ...x, [k]: v }));
  // env_var bundle.
  const [groupLabel, setGroupLabel] = useState("");
  const [rows, setRows] = useState<EnvRow[]>([emptyRow()]);
  const [envMode, setEnvMode] = useState<"rows" | "paste">("rows");
  const [pasteText, setPasteText] = useState("");
  const [pasteError, setPasteError] = useState("");
  const [syncServices, setSyncServices] = useState(false);
  const [syncServiceIds, setSyncServiceIds] = useState<number[]>([]);
  // password + avulso: reuse one credential across hosts.
  const [linkHostIds, setLinkHostIds] = useState<number[]>([]);
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  // Host credentials: a shared password or key servers can log in with.
  const hostCred = type === "password" || type === "sshkey";
  const linking = hostCred && scope === "avulso";
  const syncing = type === "env_var" && scope === "projeto";
  const options = useRelationOptions([
    ...(scope === "host" || linking ? ["hosts" as const] : []),
    ...(scope === "service" ? ["services" as const] : []),
    ...(scope === "projeto" ? ["projects" as const] : []),
  ]);
  const { data: tools = [] } = useQuery({ queryKey: ["tools"], queryFn: toolsAPI.list, enabled: scope === "tool" });
  // The sync checklist offers only the chosen project's services.
  const { data: services = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list, enabled: syncing && !!parentId });
  const projectServices = services.filter((s) => s.project_id === Number(parentId));

  const parentOptions = scope === "tool"
    ? tools.map((x) => ({ value: String(x.id), label: x.name }))
    : (scope === "host" ? options.hosts : scope === "service" ? options.services : scope === "projeto" ? options.projects : [])
      .map((o) => ({ value: String(o.id), label: o.secondary ? `${o.label} — ${o.secondary}` : o.label }));
  const parentName = parentOptions.find((o) => o.value === parentId)?.label;

  // -- validation: field errors, shown once the user tried to save --
  const filled = rows.filter((r) => r.name.trim() || r.value);
  const errors: Record<string, string> = {};
  if (scope !== "avulso" && !parentId) errors.parent = t("secretForm.errorPickParent");
  if (type === "env_var") {
    if (!GROUP_RE.test(groupLabel)) errors.groupLabel = t("secretForm.errorGroupLabel");
    if (syncing && syncServices && syncServiceIds.length === 0) errors.sync = t("secretForm.errorPickServices");
    const seen = new Set<string>();
    for (const r of filled) {
      const e = !r.name ? t("secretForm.errorVarName")
        : !VAR_RE.test(r.name) ? t("secretForm.errorVarNameFormat", { name: r.name })
        : !r.value ? t("secretForm.errorVarValue", { name: r.name })
        : seen.has(r.name) ? t("secretForm.errorVarDuplicate", { name: r.name }) : "";
      if (e) { errors.vars = e; break; }
      seen.add(r.name);
    }
    if (!filled.length && !errors.vars) errors.vars = t("secretForm.errorAddVar");
  } else {
    if (!name.trim()) errors.name = t("vault.errorNameRequired");
    if (type === "password" && !f.value) errors.value = t("vault.errorValueRequired");
    if (type === "cred") {
      if (!f.username) errors.username = t("form.required");
      if (!f.password) errors.password = t("form.required");
    }
    if (type === "sshkey" && !f.privKey) errors.privKey = t("vault.errorPrivateKeyRequired");
    if (type === "app_login") {
      if (!f.appName) errors.appName = t("form.required");
      if (!f.username) errors.username = t("form.required");
      if (!f.password) errors.password = t("form.required");
    }
  }
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const payload = (): string => {
    switch (type) {
      case "password": return JSON.stringify({ value: f.value });
      case "cred": return JSON.stringify({ username: f.username, password: f.password });
      case "sshkey": return JSON.stringify({ username: f.sshUser, private_key_pem: f.privKey, public_key: f.pubKey });
      case "app_login": return JSON.stringify({
        app_name: f.appName, username: f.username, password: f.password,
        ...(f.appUrl ? { url: f.appUrl } : {}), ...(f.notes ? { notes: f.notes } : {}),
      });
      default: return "";
    }
  };

  const create = useMutation({
    mutationFn: async () => {
      const parent = parentId ? Number(parentId) : undefined;
      if (type === "env_var") {
        // One transaction for the whole bundle; project + services in one write.
        const body: Parameters<typeof secretsAPI.envBulk>[0] = {
          visibility,
          group_label: groupLabel.trim(),
          vars: filled.map((r) => ({ name: r.name, value: r.value, description: r.description || undefined })),
        };
        if (syncing && syncServices && parent != null) body.targets = buildEnvTargets(parent, syncServiceIds);
        else { body.scope = scope; body.parent_id = parent; }
        return secretsAPI.envBulk(body);
      }
      const body: Parameters<typeof secretsAPI.create>[0] = {
        type, scope, visibility, name: name.trim(), description: description.trim() || undefined, payload: payload(),
        ...(type === "password" && f.username.trim() ? { username: f.username.trim() } : {}),
        ...(parent != null ? { parent_id: parent } : {}),
        ...(scope === "avulso" && visibility === "shared" ? grants : {}),
      };
      const created = await secretsAPI.create(body);
      if (linking && linkHostIds.length > 0) await secretsAPI.linkHosts(created.id, linkHostIds);
      return created;
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["secrets-all"] }); onDone(); },
    onError: (e) => setError(e instanceof Error ? e.message : t("form.saveFailed")),
  });

  const submit = () => {
    setAttempted(true);
    if (Object.keys(errors).length) return false;
    setError("");
    create.mutate();
  };

  const parsePaste = () => {
    const parsed = parseDotenv(pasteText);
    if (!parsed.length) { setPasteError(t("secretForm.pasteNoLines")); return; }
    setRows([...rows.filter((r) => r.name.trim() || r.value), ...parsed]);
    setPasteText("");
    setPasteError("");
    setEnvMode("rows");
  };
  const updateRow = (i: number, patch: Partial<EnvRow>) => setRows(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));

  const sections = [
    { id: "sec-what", label: t("secretForm.sectionWhat") },
    { id: "sec-where", label: t("secretForm.sectionWhere") },
    { id: "sec-who", label: t("secretForm.sectionWho") },
  ];

  return (
    <EntityFormShell
      id="secret-form"
      sections={sections}
      isEdit={false}
      isPending={create.isPending}
      submitLabel={type === "env_var" ? t("secretForm.saveBundle") : t("secretForm.create")}
      error={error}
      onSubmit={submit}
      onCancel={onCancel}
      onSubHeaderChange={onSubHeaderChange}
      onFooterChange={onFooterChange}
    >
      <FormSection id="sec-what" title={t("secretForm.sectionWhat")} description={t("secretForm.sectionWhatHint")} stack>
        <Select label={t("secretForm.type")} value={type} onChange={(e) => e.target.value && setType(e.target.value as SecretType)}
          options={TYPES.map((x) => ({ value: x, label: t(`secretForm.${TYPE_KEY[x]}`) }))}
          hint={hostCred ? t("secretForm.hostCredHint") : undefined} />

        {type !== "env_var" && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Input label={t("common.name")} required value={name} onChange={(e) => setName(e.target.value)}
              placeholder={t("secretForm.namePlaceholder")} hint={t("secretForm.nameHint")} error={err("name")} aria-invalid={!!err("name")} />
            <Input label={t("common.description")} value={description} onChange={(e) => setDescription(e.target.value)} hint={t("secretForm.descriptionHint")} />
          </div>
        )}

        {type === "password" && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Input label={t("share.fields.username")} autoComplete="off" value={f.username} placeholder="deploy"
              onChange={(e) => setField("username", e.target.value)} hint={t("secretForm.passwordUserHint")} />
            <Input label={t("secretForm.value")} required type="password" autoComplete="new-password"
              value={f.value} onChange={(e) => setField("value", e.target.value)} error={err("value")} aria-invalid={!!err("value")} />
          </div>
        )}

        {(type === "cred" || type === "app_login") && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {type === "app_login" && (
              <>
                <Input label={t("vault.appNameLabel")} required value={f.appName} onChange={(e) => setField("appName", e.target.value)}
                  placeholder="Jira" error={err("appName")} aria-invalid={!!err("appName")} />
                <Input label="URL" type="url" value={f.appUrl} onChange={(e) => setField("appUrl", e.target.value)}
                  placeholder="https://" hint={t("secretForm.appUrlHint")} />
              </>
            )}
            <Input label={t("share.fields.username")} required autoComplete="off" value={f.username}
              onChange={(e) => setField("username", e.target.value)} error={err("username")} aria-invalid={!!err("username")} />
            <Input label={t("share.fields.password")} required type="password" autoComplete="new-password" value={f.password}
              onChange={(e) => setField("password", e.target.value)} error={err("password")} aria-invalid={!!err("password")} />
            {type === "app_login" && (
              <div className="sm:col-span-2">
                <Input label={t("share.fields.notes")} value={f.notes} onChange={(e) => setField("notes", e.target.value)} />
              </div>
            )}
          </div>
        )}

        {type === "sshkey" && (
          <>
            <Input label={t("secretForm.sshUsername")} value={f.sshUser} onChange={(e) => setField("sshUser", e.target.value)}
              placeholder="deploy" hint={t("secretForm.sshUsernameHint")} />
            <Textarea label={t("vault.privateKeyPemLabel")} required rows={6} className="font-mono text-xs" value={f.privKey}
              onChange={(e) => setField("privKey", e.target.value)} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
              error={err("privKey")} aria-invalid={!!err("privKey")} />
            <Input label={t("vault.publicKeyLabel")} className="font-mono" value={f.pubKey} onChange={(e) => setField("pubKey", e.target.value)}
              placeholder="ssh-ed25519 AAAA…" hint={t("secretForm.publicKeyHint")} />
          </>
        )}

        {type === "env_var" && (
          <>
            <Input label={t("vault.groupLabel")} required value={groupLabel} onChange={(e) => setGroupLabel(e.target.value.toLowerCase())}
              placeholder="prod" hint={t("secretForm.groupLabelHint")} error={err("groupLabel")} aria-invalid={!!err("groupLabel")} />
            <FormField label={t("secretForm.variables")} hint={t("secretForm.txHint")} error={err("vars")}>
              <TabBar
                tabs={[{ key: "rows", label: t("secretForm.rows") }, { key: "paste", label: t("secretForm.paste") }]}
                activeTab={envMode}
                onChange={(k) => { setEnvMode(k as "rows" | "paste"); setPasteError(""); }}
              />
              {envMode === "paste" ? (
                <div className="space-y-2 pt-3">
                  <Textarea rows={8} className="font-mono text-xs" value={pasteText} onChange={(e) => setPasteText(e.target.value)}
                    placeholder={'KEY=xxxxx\nDB_URL="postgres://…"'} hint={t("secretForm.parseHint")} error={pasteError || undefined} />
                  <Button type="button" size="sm" variant="secondary" onClick={parsePaste}>{t("secretForm.parse")}</Button>
                </div>
              ) : (
                <div className="space-y-3 pt-3">
                  {rows.map((r, i) => (
                    // Stacked on phones, one line from sm up.
                    <div key={i} className="grid grid-cols-[1fr_auto] sm:grid-cols-[1fr_1fr_1fr_auto] gap-2 items-start pb-3 sm:pb-0 border-b sm:border-0 border-[var(--border-subtle)]">
                      <Input aria-label={t("common.name")} className="font-mono" value={r.name} placeholder="DB_URL"
                        onChange={(e) => updateRow(i, { name: e.target.value.toUpperCase() })} />
                      <IconButton className="sm:order-last" label={t("secretForm.removeVar")}
                        onClick={() => setRows(rows.length === 1 ? [emptyRow()] : rows.filter((_, j) => j !== i))}>
                        <Icon path={ICON_PATHS.trashOutline} />
                      </IconButton>
                      <div className="col-span-2 sm:col-span-1">
                        <Input aria-label={t("secretForm.value")} type="password" autoComplete="new-password" value={r.value}
                          placeholder={t("secretForm.valuePlaceholder")} onChange={(e) => updateRow(i, { value: e.target.value })} />
                      </div>
                      <div className="col-span-2 sm:col-span-1">
                        <Input aria-label={t("common.description")} value={r.description} placeholder={t("secretForm.optional")}
                          onChange={(e) => updateRow(i, { description: e.target.value })} />
                      </div>
                    </div>
                  ))}
                  <Button type="button" size="sm" variant="secondary" onClick={() => setRows([...rows, emptyRow()])}>
                    <Icon path={ICON_PATHS.plus} className="w-3.5 h-3.5" /> {t("secretForm.addVar")}
                  </Button>
                </div>
              )}
            </FormField>
          </>
        )}
      </FormSection>

      <FormSection id="sec-where" title={t("secretForm.sectionWhere")} description={t("secretForm.sectionWhereHint")} stack>
        {fixed ? (
          <FormField label={t("secretForm.scope")}>
            <p className="text-sm text-[var(--text-primary)] pt-1">
              {t(`secretForm.${SCOPE_KEY[scope]}`)}{parentName ? ` · ${parentName}` : ""}
            </p>
          </FormField>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Select label={t("secretForm.scope")} value={scope}
              onChange={(e) => { if (!e.target.value) return; setScope(e.target.value as SecretScope); setParentId(""); setSyncServiceIds([]); }}
              options={SCOPES.map((x) => ({ value: x, label: t(`secretForm.${SCOPE_KEY[x]}`) }))} />
            {scope !== "avulso" && (
              <Select label={t(`secretForm.parent.${scope}`)} required value={parentId}
                onChange={(e) => { setParentId(e.target.value); setSyncServiceIds([]); }}
                options={parentOptions} error={err("parent")} />
            )}
          </div>
        )}

        {syncing && parentId && (
          <FormField error={err("sync")}>
            <Checkbox label={t("secretForm.alsoSyncServices")} checked={syncServices}
              onChange={(v) => { setSyncServices(v); if (!v) setSyncServiceIds([]); }} />
            {syncServices && (
              <div className="mt-2 pl-6 flex flex-col gap-2">
                {projectServices.length === 0 ? (
                  <p className="text-xs text-[var(--text-muted)]">{t("secretForm.projectNoServices")}</p>
                ) : projectServices.map((s) => (
                  <Checkbox key={s.id} label={s.nickname} checked={syncServiceIds.includes(s.id)}
                    onChange={(v) => setSyncServiceIds((ids) => (v ? [...ids, s.id] : ids.filter((x) => x !== s.id)))} />
                ))}
              </div>
            )}
          </FormField>
        )}

        {linking && (
          <FormField hint={t("secretForm.linkHostsLoginHint")}>
            <RelationPicker label={t("secretForm.linkHosts")} options={options.hosts} selected={linkHostIds} onChange={setLinkHostIds}
              placeholder={t("secretForm.searchHosts")} />
          </FormField>
        )}
      </FormSection>

      <FormSection id="sec-who" title={t("secretForm.sectionWho")} stack>
        <RadioGroup name="secret-visibility" value={visibility} onChange={(v) => setVisibility(v as Visibility)}
          options={[
            { value: "shared", label: t("secretForm.visibilityShared") },
            { value: "personal", label: t("secretForm.visibilityPersonal") },
          ]} />
        <p className="text-xs text-[var(--text-muted)] -mt-2">
          {visibility === "personal" ? t("secretForm.visibilityPersonalHint")
            : scope === "avulso" ? t("secretForm.visibilitySharedAvulsoHint") : t("secretForm.visibilitySharedHint")}
        </p>
        {scope === "avulso" && visibility === "shared" && <EntidadeScopeFields value={grants} onChange={setGrants} compact />}
      </FormSection>
    </EntityFormShell>
  );
}

/** The create drawer: mounts the form only while open, so each opening starts clean. */
export function NewSecretDrawer({ open, onClose, defaultScope, defaultParentId }: {
  open: boolean;
  onClose: () => void;
  defaultScope?: SecretScope;
  defaultParentId?: number;
}) {
  const { t } = useLocale();
  const [subHeader, setSubHeader] = useState<ReactNode>(null);
  const [footer, setFooter] = useState<ReactNode>(null);
  return (
    <Drawer open={open} onClose={onClose} title={t("secretForm.title")} subHeader={subHeader} footer={footer}>
      {open && (
        <SecretForm defaultScope={defaultScope} defaultParentId={defaultParentId} onDone={onClose} onCancel={onClose}
          onSubHeaderChange={setSubHeader} onFooterChange={setFooter} />
      )}
    </Drawer>
  );
}
