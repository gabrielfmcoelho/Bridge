"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI, servicesAPI, hostsAPI, toolsAPI, projectsAPI } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TabBar from "@/components/ui/TabBar";
import { parseDotenv } from "@/lib/parseDotenv";
import { buildEnvTargets } from "@/lib/envTargets";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import type { AssetGrantsInput } from "@/lib/types";

interface NewSecretModalProps {
  open: boolean;
  onClose: () => void;
  /** Opened from an asset (a project's or service's Segredos tab): start
   *  attached to it, shared. */
  defaultScope?: Scope;
  defaultParentId?: number;
}

type SecretType = "password" | "cred" | "sshkey" | "app_login" | "env_var";
type Scope = "service" | "host" | "tool" | "projeto" | "avulso";
type Visibility = "personal" | "shared";

// FormRow is a thin label + input wrapper. ui/Field is read-only display;
// we need an input wrapper here.
function FormRow({
  label,
  required,
  hint,
  children,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <label className="text-xs font-medium text-[var(--text-muted)] block">
        {label}
        {required && <span className="text-red-400 ml-0.5">*</span>}
      </label>
      {children}
      {hint && <p className="text-[10px] text-[var(--text-faint)]">{hint}</p>}
    </div>
  );
}

// EnvVarRow is one row in the bulk env-var entry table. The parent owns
// the array of rows and forwards changes back so add/remove stays in
// the modal's state.
interface EnvVarRow {
  name: string;
  value: string;
  description: string;
}

// NewSecretModal — single entry point for adding any of the five secret
// types via the unified /api/secrets POST endpoint. Payload shape is
// per-type (spec §4.2); this component builds the JSON envelope and
// hands it to secretsAPI.create — EXCEPT env_var which uses the bulk
// endpoint (/api/secrets/env/bulk) so multiple vars commit atomically.
//
// Parent selection: when scope != avulso we fetch the relevant list
// (services / hosts / external_tools) and surface it as a dropdown so
// operators don't need to remember numeric IDs.
export default function NewSecretModal({ open, onClose, defaultScope, defaultParentId }: NewSecretModalProps) {
  const qc = useQueryClient();
  const { t } = useLocale();

  // Common metadata fields.
  const [type, setType] = useState<SecretType>("password");
  const [scope, setScope] = useState<Scope>(defaultScope ?? "avulso");
  const [visibility, setVisibility] = useState<Visibility>(defaultParentId ? "shared" : "personal");
  // Entidade grants apply only to shared avulso secrets (parented ones inherit
  // from their parent; personal ones are owner-only).
  const { user } = useAuth();
  const [grants, setGrants] = useState<AssetGrantsInput>(() => defaultGrants(user));
  const [parentID, setParentID] = useState<string>(defaultParentId ? String(defaultParentId) : ""); // stored as string for Select compat
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  // Per-type single-value payload fields.
  const [valueField, setValueField] = useState(""); // password
  const [credUsername, setCredUsername] = useState("");
  const [credPassword, setCredPassword] = useState("");
  const [sshUsername, setSshUsername] = useState("");
  const [sshPrivKey, setSshPrivKey] = useState("");
  const [sshPubKey, setSshPubKey] = useState("");
  const [appName, setAppName] = useState("");
  const [appURL, setAppURL] = useState("");
  const [appUsername, setAppUsername] = useState("");
  const [appPassword, setAppPassword] = useState("");
  const [appNotes, setAppNotes] = useState("");

  // env_var: bulk rows + the shared group_label.
  const [groupLabel, setGroupLabel] = useState("");
  const [envVars, setEnvVars] = useState<EnvVarRow[]>([{ name: "", value: "", description: "" }]);
  // env_var input mode: "rows" = per-var table; "paste" = a .env blob parsed
  // into rows before saving. Both feed the same envVars array.
  const [envInputMode, setEnvInputMode] = useState<"rows" | "paste">("rows");
  const [pasteText, setPasteText] = useState("");
  const [pasteHint, setPasteHint] = useState("");
  // env_var multi-target: when scope=projeto, optionally also sync the same
  // bundle to one or more of that project's services (one atomic write).
  const [alsoSyncServices, setAlsoSyncServices] = useState(false);
  const [syncServiceIDs, setSyncServiceIDs] = useState<number[]>([]);
  // password + scope=avulso: optionally link this new shared credential to
  // hosts in one step (create the credential, then reuse it across N hosts).
  const [linkHostIDs, setLinkHostIDs] = useState<number[]>([]);
  const [hostSearch, setHostSearch] = useState("");

  // Parent list: fetch only when scope needs one. Each useQuery has
  // enabled: scope === ... so we don't hit /api/hosts when the user is
  // creating an avulso personal secret.
  const services = useQuery({
    queryKey: ["services-list"],
    queryFn: servicesAPI.list,
    enabled:
      open &&
      (scope === "service" ||
        (type === "env_var" && scope === "projeto" && alsoSyncServices)),
  });
  const hosts = useQuery({
    queryKey: ["hosts-list"],
    queryFn: () => hostsAPI.list(),
    enabled:
      open &&
      (scope === "host" || (type === "password" && scope === "avulso")),
  });
  const tools = useQuery({
    queryKey: ["tools-list"],
    queryFn: toolsAPI.list,
    enabled: open && scope === "tool",
  });
  const projects = useQuery({
    queryKey: ["projects-list"],
    queryFn: projectsAPI.list,
    enabled: open && scope === "projeto",
  });

  // Build the parent options array for whichever scope is active. Label
  // shape varies per parent: services + hosts use nickname; tools use name.
  const parentOptions = useMemo(() => {
    type Opt = { value: string; label: string };
    const empty: Opt = { value: "", label: scope === "avulso" ? "—" : t("secretForm.select") };
    switch (scope) {
      case "service":
        return [empty, ...(services.data ?? []).map((s) => ({ value: String(s.id), label: s.nickname }))];
      case "host":
        return [
          empty,
          ...(hosts.data ?? []).map((h) => ({ value: String(h.id), label: `${h.nickname} (${h.oficial_slug})` })),
        ];
      case "tool":
        return [empty, ...(tools.data ?? []).map((t) => ({ value: String(t.id), label: t.name }))];
      case "projeto":
        return [empty, ...(projects.data ?? []).map((p) => ({ value: String(p.id), label: p.name }))];
      default:
        return [empty];
    }
  }, [scope, services.data, hosts.data, tools.data, projects.data, t]);

  // Services belonging to the selected project (parentID when scope=projeto).
  // The checklist below only offers these, so an out-of-project service is
  // unrepresentable — the backend guardServiceInProject stays the safety net.
  const projectServices = useMemo(
    () => (services.data ?? []).filter((s) => s.project_id === Number(parentID)),
    [services.data, parentID],
  );

  const reset = () => {
    setType("password");
    setScope(defaultScope ?? "avulso");
    setVisibility(defaultParentId ? "shared" : "personal");
    setParentID(defaultParentId ? String(defaultParentId) : "");
    setName("");
    setDescription("");
    setValueField("");
    setCredUsername("");
    setCredPassword("");
    setSshUsername("");
    setSshPrivKey("");
    setSshPubKey("");
    setAppName("");
    setAppURL("");
    setAppUsername("");
    setAppPassword("");
    setAppNotes("");
    setGroupLabel("");
    setEnvVars([{ name: "", value: "", description: "" }]);
    setEnvInputMode("rows");
    setPasteText("");
    setPasteHint("");
    setAlsoSyncServices(false);
    setSyncServiceIDs([]);
    setLinkHostIDs([]);
    setHostSearch("");
  };

  const buildPayload = (): string => {
    switch (type) {
      case "password":
        return JSON.stringify({ value: valueField });
      case "cred":
        return JSON.stringify({ username: credUsername, password: credPassword });
      case "sshkey":
        return JSON.stringify({
          username: sshUsername,
          private_key_pem: sshPrivKey,
          public_key: sshPubKey,
        });
      case "app_login": {
        const payload: Record<string, string> = {
          app_name: appName,
          username: appUsername,
          password: appPassword,
        };
        if (appURL) payload.url = appURL;
        if (appNotes) payload.notes = appNotes;
        return JSON.stringify(payload);
      }
      case "env_var":
        // env_var doesn't go through buildPayload — the bulk endpoint
        // ships rows of {name, value, description}. The branch in
        // create.mutationFn handles env_var separately.
        return "";
    }
  };

  // Per-type validation that runs before mutating. Matches the backend's
  // ValidateEnvVarName / ValidateAppLoginPayload (Phase 2 Task 2.1 + 2.4)
  // so users see the same rejection messages without a server round-trip
  // for the obvious cases.
  const validate = (): string | null => {
    if (scope !== "avulso" && !parentID) {
      return t("secretForm.errorPickParent");
    }
    if (type === "env_var") {
      // env_var doesn't use the top-level `name` (each var has its own
      // name); only group_label is required at the bundle level.
      if (!groupLabel || !/^[a-z][a-z0-9-]*$/.test(groupLabel)) {
        return t("secretForm.errorGroupLabel");
      }
      if (scope === "projeto" && alsoSyncServices && syncServiceIDs.length === 0) {
        return t("secretForm.errorPickServices");
      }
      const filled = envVars.filter((v) => v.name.trim() || v.value);
      if (filled.length === 0) return t("secretForm.errorAddVar");
      const seen = new Set<string>();
      for (const v of filled) {
        if (!v.name) return t("secretForm.errorVarName");
        if (!/^[A-Z_][A-Z0-9_]*$/.test(v.name)) {
          return t("secretForm.errorVarNameFormat", { name: v.name });
        }
        if (!v.value) return t("secretForm.errorVarValue", { name: v.name });
        if (seen.has(v.name)) return t("secretForm.errorVarDuplicate", { name: v.name });
        seen.add(v.name);
      }
      return null;
    }
    if (!name.trim()) return t("vault.errorNameRequired");
    switch (type) {
      case "password":
        if (!valueField) return t("vault.errorValueRequired");
        break;
      case "cred":
        if (!credUsername || !credPassword) return t("vault.errorUsernamePasswordRequired");
        break;
      case "sshkey":
        if (!sshPrivKey) return t("vault.errorPrivateKeyRequired");
        break;
      case "app_login":
        if (!appName || !appUsername || !appPassword) {
          return t("vault.errorAppLoginRequired");
        }
        break;
    }
    return null;
  };

  const create = useMutation({
    mutationFn: async () => {
      const reason = validate();
      if (reason) throw new Error(reason);
      const parsedParent = parentID ? Number(parentID) : undefined;

      if (type === "env_var") {
        // Bulk path — one tx for the whole batch (Plans.md Task 2.2).
        const vars = envVars
          .filter((v) => v.name.trim() && v.value)
          .map((v) => ({ name: v.name, value: v.value, description: v.description || undefined }));
        const payload: Parameters<typeof secretsAPI.envBulk>[0] = {
          visibility,
          group_label: groupLabel.trim(),
          vars,
        };
        // Project + services → multi-target (one atomic write to all). Any
        // other case keeps the legacy single scope/parent path.
        if (
          scope === "projeto" &&
          alsoSyncServices &&
          syncServiceIDs.length > 0 &&
          parsedParent != null
        ) {
          payload.targets = buildEnvTargets(parsedParent, syncServiceIDs);
        } else {
          payload.scope = scope;
          payload.parent_id = parsedParent;
        }
        return secretsAPI.envBulk(payload);
      }

      const body: Parameters<typeof secretsAPI.create>[0] = {
        type,
        scope,
        visibility,
        name: name.trim(),
        description: description.trim() || undefined,
        payload: buildPayload(),
      };
      if (parsedParent != null) {
        body.parent_id = parsedParent;
      }
      if (scope === "avulso" && visibility === "shared") {
        Object.assign(body, grants);
      }
      const created = await secretsAPI.create(body);
      // One-step: link the newly-created shared credential to the chosen hosts.
      if (type === "password" && scope === "avulso" && linkHostIDs.length > 0) {
        await secretsAPI.linkHosts(created.id, linkHostIDs);
      }
      return created;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["secrets-all"] });
      reset();
      onClose();
    },
  });

  const handleClose = () => {
    reset();
    onClose();
  };

  // env_var row manipulation helpers.
  const updateRow = (idx: number, patch: Partial<EnvVarRow>) => {
    setEnvVars(envVars.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
  };
  const addRow = () => setEnvVars([...envVars, { name: "", value: "", description: "" }]);
  const removeRow = (idx: number) => {
    if (envVars.length === 1) {
      // Clear instead of removing the only row so the table doesn't
      // disappear entirely.
      setEnvVars([{ name: "", value: "", description: "" }]);
      return;
    }
    setEnvVars(envVars.filter((_, i) => i !== idx));
  };

  // Parse the pasted .env blob into rows. Appends to any real rows already
  // typed (dropping a single blank starter row), then switches to the Rows
  // tab so the user reviews before saving. Parsed rows inherit the same
  // name/value/dup validation as hand-typed ones — no second code path.
  const parsePasteIntoRows = () => {
    const parsed = parseDotenv(pasteText);
    if (parsed.length === 0) {
      setPasteHint(t("secretForm.pasteNoLines"));
      return;
    }
    const existing = envVars.filter((v) => v.name.trim() || v.value);
    setEnvVars([...existing, ...parsed]);
    setPasteText("");
    setPasteHint("");
    setEnvInputMode("rows");
  };

  return (
    <ResponsiveModal open={open} onClose={handleClose} title={t("secretForm.title")}>
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <Card>
          <div className="grid grid-cols-3 gap-2">
            <FormRow label={t("secretForm.type")} required>
              <Select
                value={type}
                onChange={(e) => setType(e.target.value as SecretType)}
                options={[
                  { value: "password", label: t("secretForm.typePassword") },
                  { value: "cred", label: t("secretForm.typeCred") },
                  { value: "sshkey", label: t("secretForm.typeSshkey") },
                  { value: "app_login", label: t("secretForm.typeAppLogin") },
                  { value: "env_var", label: t("secretForm.typeEnvVar") },
                ]}
              />
            </FormRow>
            <FormRow label={t("secretForm.scope")} required>
              <Select
                value={scope}
                onChange={(e) => {
                  setScope(e.target.value as Scope);
                  setParentID(""); // dropping the previous selection avoids stale FK reference
                }}
                options={[
                  { value: "avulso", label: t("secretForm.scopeAvulso") },
                  { value: "service", label: t("secretForm.scopeService") },
                  { value: "host", label: t("secretForm.scopeHost") },
                  { value: "tool", label: t("secretForm.scopeTool") },
                  { value: "projeto", label: t("secretForm.scopeProjeto") },
                ]}
              />
            </FormRow>
            <FormRow label={t("secretForm.visibility")} required>
              <Select
                value={visibility}
                onChange={(e) => setVisibility(e.target.value as Visibility)}
                options={[
                  { value: "personal", label: t("secretForm.visibilityPersonal") },
                  { value: "shared", label: t("secretForm.visibilityShared") },
                ]}
              />
            </FormRow>
          </div>

          {scope === "avulso" && visibility === "shared" && (
            <div className="mt-3">
              <EntidadeScopeFields value={grants} onChange={setGrants} compact />
            </div>
          )}

          {scope !== "avulso" && (
            <div className="mt-3">
              <FormRow label={t(`secretForm.parent.${scope}`)} required hint={t("secretForm.parentHint")}>
                <Select
                  value={parentID}
                  onChange={(e) => {
                    setParentID(e.target.value);
                    // Different project → its service list changes; drop stale picks.
                    setSyncServiceIDs([]);
                  }}
                  options={parentOptions}
                />
              </FormRow>
              {(scope === "service" && services.isLoading) ||
              (scope === "host" && hosts.isLoading) ||
              (scope === "tool" && tools.isLoading) ||
              (scope === "projeto" && projects.isLoading) ? (
                <p className="text-[10px] text-[var(--text-faint)] mt-1">{t("common.loading")}</p>
              ) : null}

              {/* env_var only: sync the same bundle to services of this project. */}
              {type === "env_var" && scope === "projeto" && parentID && (
                <div className="mt-3">
                  <label className="flex items-center gap-2 text-xs text-[var(--text-muted)] cursor-pointer">
                    <input
                      type="checkbox"
                      checked={alsoSyncServices}
                      onChange={(e) => {
                        setAlsoSyncServices(e.target.checked);
                        if (!e.target.checked) setSyncServiceIDs([]);
                      }}
                    />
                    {t("secretForm.alsoSyncServices")}
                  </label>
                  {alsoSyncServices && (
                    <div className="mt-2 space-y-1 pl-6">
                      {services.isLoading ? (
                        <p className="text-[10px] text-[var(--text-faint)]">{t("common.loading")}</p>
                      ) : projectServices.length === 0 ? (
                        <p className="text-[10px] text-[var(--text-faint)]">
                          {t("secretForm.projectNoServices")}
                        </p>
                      ) : (
                        projectServices.map((s) => (
                          <label
                            key={s.id}
                            className="flex items-center gap-2 text-xs text-[var(--text-primary)] cursor-pointer"
                          >
                            <input
                              type="checkbox"
                              checked={syncServiceIDs.includes(s.id)}
                              onChange={(e) =>
                                setSyncServiceIDs((prev) =>
                                  e.target.checked
                                    ? [...prev, s.id]
                                    : prev.filter((id) => id !== s.id),
                                )
                              }
                            />
                            {s.nickname}
                          </label>
                        ))
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {/* env_var hides the single-name field — each var-row carries its own. */}
          {type !== "env_var" && (
            <div className="mt-3 space-y-3">
              <FormRow label={t("common.name")} required hint={t("secretForm.nameHint")}>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("secretForm.namePlaceholder")} />
              </FormRow>
              <FormRow label={t("common.description")} hint={t("secretForm.descriptionHint")}>
                <Input value={description} onChange={(e) => setDescription(e.target.value)} />
              </FormRow>
            </div>
          )}
        </Card>

        {/* Per-type payload section */}
        <Card>
          <h3 className="text-sm font-semibold mb-3">
            {type === "env_var" ? t("secretForm.variables") : t("secretForm.payload")}
          </h3>

          {type === "password" && (
            <div className="space-y-3">
              <FormRow label={t("secretForm.value")} required>
                <Input
                  type="password"
                  value={valueField}
                  onChange={(e) => setValueField(e.target.value)}
                  autoComplete="new-password"
                />
              </FormRow>
              {scope === "avulso" && (
                <div>
                  <label className="text-xs font-medium text-[var(--text-muted)] block mb-1">
                    {t("secretForm.linkHosts")}{" "}
                    <span className="text-[var(--text-faint)]">
                      {t("secretForm.linkHostsHint")}
                    </span>
                  </label>
                  <Input
                    value={hostSearch}
                    onChange={(e) => setHostSearch(e.target.value)}
                    placeholder={t("secretForm.searchHosts")}
                  />
                  <div className="mt-2 max-h-40 overflow-y-auto space-y-1 pr-1">
                    {hosts.isLoading ? (
                      <p className="text-[10px] text-[var(--text-faint)]">{t("common.loading")}</p>
                    ) : (
                      (hosts.data ?? [])
                        .filter(
                          (h) =>
                            hostSearch === "" ||
                            h.nickname.toLowerCase().includes(hostSearch.toLowerCase()),
                        )
                        .map((h) => (
                          <label
                            key={h.id}
                            className="flex items-center gap-2 text-xs text-[var(--text-primary)] cursor-pointer"
                          >
                            <input
                              type="checkbox"
                              checked={linkHostIDs.includes(h.id)}
                              onChange={(e) =>
                                setLinkHostIDs((prev) =>
                                  e.target.checked
                                    ? [...prev, h.id]
                                    : prev.filter((id) => id !== h.id),
                                )
                              }
                            />
                            {h.nickname}{" "}
                            <span className="text-[var(--text-faint)]">({h.user})</span>
                          </label>
                        ))
                    )}
                  </div>
                  {linkHostIDs.length > 0 && (
                    <p className="text-[10px] text-[var(--text-faint)] mt-1">
                      {t(linkHostIDs.length === 1 ? "secretForm.hostsWillUseOne" : "secretForm.hostsWillUseMany", { count: String(linkHostIDs.length) })}
                    </p>
                  )}
                </div>
              )}
            </div>
          )}

          {type === "cred" && (
            <div className="space-y-3">
              <FormRow label={t("share.fields.username")} required>
                <Input value={credUsername} onChange={(e) => setCredUsername(e.target.value)} autoComplete="off" />
              </FormRow>
              <FormRow label={t("share.fields.password")} required>
                <Input
                  type="password"
                  value={credPassword}
                  onChange={(e) => setCredPassword(e.target.value)}
                  autoComplete="new-password"
                />
              </FormRow>
            </div>
          )}

          {type === "sshkey" && (
            <div className="space-y-3">
              <FormRow label={t("secretForm.sshUsername")} hint={t("secretForm.sshUsernameHint")}>
                <Input value={sshUsername} onChange={(e) => setSshUsername(e.target.value)} placeholder="deploy" />
              </FormRow>
              <FormRow label={t("vault.privateKeyPemLabel")} required>
                <textarea
                  value={sshPrivKey}
                  onChange={(e) => setSshPrivKey(e.target.value)}
                  rows={6}
                  className="w-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2 text-xs font-mono text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                />
              </FormRow>
              <FormRow label={t("vault.publicKeyLabel")} hint={t("secretForm.publicKeyHint")}>
                <Input
                  value={sshPubKey}
                  onChange={(e) => setSshPubKey(e.target.value)}
                  placeholder="ssh-ed25519 AAAA…"
                />
              </FormRow>
            </div>
          )}

          {type === "app_login" && (
            <div className="space-y-3">
              <FormRow label={t("vault.appNameLabel")} required>
                <Input value={appName} onChange={(e) => setAppName(e.target.value)} placeholder="Jira" />
              </FormRow>
              <FormRow label="URL" hint={t("secretForm.appUrlHint")}>
                <Input
                  type="url"
                  value={appURL}
                  onChange={(e) => setAppURL(e.target.value)}
                  placeholder="https://example.atlassian.net"
                />
              </FormRow>
              <FormRow label={t("share.fields.username")} required>
                <Input value={appUsername} onChange={(e) => setAppUsername(e.target.value)} autoComplete="off" />
              </FormRow>
              <FormRow label={t("share.fields.password")} required>
                <Input
                  type="password"
                  value={appPassword}
                  onChange={(e) => setAppPassword(e.target.value)}
                  autoComplete="new-password"
                />
              </FormRow>
              <FormRow label={t("share.fields.notes")}>
                <Input value={appNotes} onChange={(e) => setAppNotes(e.target.value)} />
              </FormRow>
            </div>
          )}

          {type === "env_var" && (
            <div className="space-y-3">
              <FormRow
                label={t("vault.groupLabel")}
                required
                hint={t("secretForm.groupLabelHint")}
              >
                <Input
                  value={groupLabel}
                  onChange={(e) => setGroupLabel(e.target.value.toLowerCase())}
                  placeholder="prod"
                />
              </FormRow>

              <TabBar
                tabs={[
                  { key: "rows", label: t("secretForm.rows") },
                  { key: "paste", label: t("secretForm.paste") },
                ]}
                activeTab={envInputMode}
                onChange={(k) => {
                  setEnvInputMode(k as "rows" | "paste");
                  setPasteHint("");
                }}
              />

              {envInputMode === "paste" && (
                <div className="space-y-2">
                  <textarea
                    value={pasteText}
                    onChange={(e) => setPasteText(e.target.value)}
                    rows={8}
                    className="w-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2 text-xs font-mono text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
                    placeholder={'KEY=xxxxx #comment\nKEY2=yyyyy #comment2\nDB_URL="postgres://…"'}
                  />
                  <div className="flex items-center gap-2">
                    <Button type="button" size="sm" variant="secondary" onClick={parsePasteIntoRows}>
                      {t("secretForm.parse")}
                    </Button>
                    <p className="text-[10px] text-[var(--text-faint)]">
                      {t("secretForm.parseHint")}
                    </p>
                  </div>
                  {pasteHint && <p className="text-[10px] text-red-400">{pasteHint}</p>}
                </div>
              )}

              {envInputMode === "rows" && (
                <>
              <div className="space-y-2">
                <div className="grid grid-cols-12 gap-2 text-[10px] font-medium text-[var(--text-faint)] uppercase tracking-wider">
                  <span className="col-span-4">{t("common.name")}</span>
                  <span className="col-span-4">{t("secretForm.value")}</span>
                  <span className="col-span-3">{t("common.description")}</span>
                  <span className="col-span-1"></span>
                </div>
                {envVars.map((row, idx) => (
                  <div key={idx} className="grid grid-cols-12 gap-2 items-start">
                    <div className="col-span-4">
                      <Input
                        value={row.name}
                        onChange={(e) => updateRow(idx, { name: e.target.value.toUpperCase() })}
                        placeholder="DB_URL"
                      />
                    </div>
                    <div className="col-span-4">
                      <Input
                        type="password"
                        value={row.value}
                        onChange={(e) => updateRow(idx, { value: e.target.value })}
                        placeholder={t("secretForm.valuePlaceholder")}
                      />
                    </div>
                    <div className="col-span-3">
                      <Input
                        value={row.description}
                        onChange={(e) => updateRow(idx, { description: e.target.value })}
                        placeholder={t("secretForm.optional")}
                      />
                    </div>
                    <div className="col-span-1 flex justify-end">
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => removeRow(idx)}
                        aria-label={t("secretForm.removeVar")}
                      >
                        ✕
                      </Button>
                    </div>
                  </div>
                ))}
              </div>

              <div className="flex items-center gap-2">
                <Button type="button" size="sm" variant="secondary" onClick={addRow}>
                  + {t("secretForm.addVar")}
                </Button>
                <p className="text-[10px] text-[var(--text-faint)]">
                  {t("secretForm.txHint")}
                </p>
              </div>
                </>
              )}
            </div>
          )}
        </Card>

        <div className="flex items-center gap-2">
          <Button type="submit" size="sm" disabled={create.isPending}>
            {create.isPending ? t("secretForm.saving") : type === "env_var" ? t("secretForm.saveBundle") : t("secretForm.create")}
          </Button>
          <Button type="button" size="sm" variant="ghost" onClick={handleClose}>
            {t("common.cancel")}
          </Button>
          {create.isError && (
            <span className="text-xs text-red-400">{(create.error as Error).message}</span>
          )}
        </div>
      </form>
    </ResponsiveModal>
  );
}
