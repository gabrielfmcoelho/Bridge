"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI, servicesAPI, hostsAPI, toolsAPI } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";

interface NewSecretModalProps {
  open: boolean;
  onClose: () => void;
}

type SecretType = "password" | "cred" | "sshkey" | "app_login" | "env_var";
type Scope = "service" | "host" | "tool" | "avulso";
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
export default function NewSecretModal({ open, onClose }: NewSecretModalProps) {
  const qc = useQueryClient();

  // Common metadata fields.
  const [type, setType] = useState<SecretType>("password");
  const [scope, setScope] = useState<Scope>("avulso");
  const [visibility, setVisibility] = useState<Visibility>("personal");
  const [parentID, setParentID] = useState<string>(""); // stored as string for Select compat
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

  // Parent list: fetch only when scope needs one. Each useQuery has
  // enabled: scope === ... so we don't hit /api/hosts when the user is
  // creating an avulso personal secret.
  const services = useQuery({
    queryKey: ["services-list"],
    queryFn: servicesAPI.list,
    enabled: open && scope === "service",
  });
  const hosts = useQuery({
    queryKey: ["hosts-list"],
    queryFn: () => hostsAPI.list(),
    enabled: open && scope === "host",
  });
  const tools = useQuery({
    queryKey: ["tools-list"],
    queryFn: toolsAPI.list,
    enabled: open && scope === "tool",
  });

  // Build the parent options array for whichever scope is active. Label
  // shape varies per parent: services + hosts use nickname; tools use name.
  const parentOptions = useMemo(() => {
    type Opt = { value: string; label: string };
    const empty: Opt = { value: "", label: scope === "avulso" ? "—" : `Select a ${scope}…` };
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
      default:
        return [empty];
    }
  }, [scope, services.data, hosts.data, tools.data]);

  const reset = () => {
    setType("password");
    setScope("avulso");
    setVisibility("personal");
    setParentID("");
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
      return `Pick a ${scope} to attach this secret to.`;
    }
    if (type === "env_var") {
      // env_var doesn't use the top-level `name` (each var has its own
      // name); only group_label is required at the bundle level.
      if (!groupLabel || !/^[a-z][a-z0-9-]*$/.test(groupLabel)) {
        return "Group label must match ^[a-z][a-z0-9-]*$ (lowercase only).";
      }
      const filled = envVars.filter((v) => v.name.trim() || v.value);
      if (filled.length === 0) return "Add at least one env var.";
      const seen = new Set<string>();
      for (const v of filled) {
        if (!v.name) return "Every env var needs a name.";
        if (!/^[A-Z_][A-Z0-9_]*$/.test(v.name)) {
          return `env_var name "${v.name}" must match ^[A-Z_][A-Z0-9_]*$ (uppercase only).`;
        }
        if (!v.value) return `env_var "${v.name}" needs a value.`;
        if (seen.has(v.name)) return `env_var "${v.name}" appears twice — names must be unique.`;
        seen.add(v.name);
      }
      return null;
    }
    if (!name.trim()) return "Name is required.";
    switch (type) {
      case "password":
        if (!valueField) return "Value is required.";
        break;
      case "cred":
        if (!credUsername || !credPassword) return "Username and password are required.";
        break;
      case "sshkey":
        if (!sshPrivKey) return "Private key is required.";
        break;
      case "app_login":
        if (!appName || !appUsername || !appPassword) {
          return "App name, username, and password are required.";
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
        return secretsAPI.envBulk({
          scope,
          parent_id: parsedParent,
          visibility,
          group_label: groupLabel.trim(),
          vars,
        });
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
      return secretsAPI.create(body);
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

  return (
    <ResponsiveModal open={open} onClose={handleClose} title="New secret">
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <Card>
          <div className="grid grid-cols-3 gap-2">
            <FormRow label="Type" required>
              <Select
                value={type}
                onChange={(e) => setType(e.target.value as SecretType)}
                options={[
                  { value: "password", label: "password" },
                  { value: "cred", label: "cred" },
                  { value: "sshkey", label: "sshkey" },
                  { value: "app_login", label: "app_login" },
                  { value: "env_var", label: "env_var (bulk)" },
                ]}
              />
            </FormRow>
            <FormRow label="Scope" required>
              <Select
                value={scope}
                onChange={(e) => {
                  setScope(e.target.value as Scope);
                  setParentID(""); // dropping the previous selection avoids stale FK reference
                }}
                options={[
                  { value: "avulso", label: "avulso" },
                  { value: "service", label: "service" },
                  { value: "host", label: "host" },
                  { value: "tool", label: "tool" },
                ]}
              />
            </FormRow>
            <FormRow label="Visibility" required>
              <Select
                value={visibility}
                onChange={(e) => setVisibility(e.target.value as Visibility)}
                options={[
                  { value: "personal", label: "personal" },
                  { value: "shared", label: "shared" },
                ]}
              />
            </FormRow>
          </div>

          {scope !== "avulso" && (
            <div className="mt-3">
              <FormRow label={`Parent ${scope}`} required hint="Pick from the list of existing rows.">
                <Select
                  value={parentID}
                  onChange={(e) => setParentID(e.target.value)}
                  options={parentOptions}
                />
              </FormRow>
              {(scope === "service" && services.isLoading) ||
              (scope === "host" && hosts.isLoading) ||
              (scope === "tool" && tools.isLoading) ? (
                <p className="text-[10px] text-[var(--text-faint)] mt-1">Loading {scope}s…</p>
              ) : null}
            </div>
          )}

          {/* env_var hides the single-name field — each var-row carries its own. */}
          {type !== "env_var" && (
            <div className="mt-3 space-y-3">
              <FormRow label="Name" required hint="Human-readable label.">
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="primary" />
              </FormRow>
              <FormRow label="Description" hint="Optional. Visible to anyone who can see the secret metadata.">
                <Input value={description} onChange={(e) => setDescription(e.target.value)} />
              </FormRow>
            </div>
          )}
        </Card>

        {/* Per-type payload section */}
        <Card>
          <h3 className="text-sm font-semibold mb-3">
            {type === "env_var" ? "Variables" : "Payload"}
          </h3>

          {type === "password" && (
            <FormRow label="Value" required>
              <Input
                type="password"
                value={valueField}
                onChange={(e) => setValueField(e.target.value)}
                autoComplete="new-password"
              />
            </FormRow>
          )}

          {type === "cred" && (
            <div className="space-y-3">
              <FormRow label="Username" required>
                <Input value={credUsername} onChange={(e) => setCredUsername(e.target.value)} autoComplete="off" />
              </FormRow>
              <FormRow label="Password" required>
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
              <FormRow label="SSH username" hint="The remote user this key authenticates as.">
                <Input value={sshUsername} onChange={(e) => setSshUsername(e.target.value)} placeholder="deploy" />
              </FormRow>
              <FormRow label="Private key (PEM)" required>
                <textarea
                  value={sshPrivKey}
                  onChange={(e) => setSshPrivKey(e.target.value)}
                  rows={6}
                  className="w-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2 text-xs font-mono text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                />
              </FormRow>
              <FormRow label="Public key" hint="Optional but recommended for fingerprint matching.">
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
              <FormRow label="App name" required>
                <Input value={appName} onChange={(e) => setAppName(e.target.value)} placeholder="Jira" />
              </FormRow>
              <FormRow label="URL" hint="Optional — link to the app's login page.">
                <Input
                  type="url"
                  value={appURL}
                  onChange={(e) => setAppURL(e.target.value)}
                  placeholder="https://example.atlassian.net"
                />
              </FormRow>
              <FormRow label="Username" required>
                <Input value={appUsername} onChange={(e) => setAppUsername(e.target.value)} autoComplete="off" />
              </FormRow>
              <FormRow label="Password" required>
                <Input
                  type="password"
                  value={appPassword}
                  onChange={(e) => setAppPassword(e.target.value)}
                  autoComplete="new-password"
                />
              </FormRow>
              <FormRow label="Notes">
                <Input value={appNotes} onChange={(e) => setAppNotes(e.target.value)} />
              </FormRow>
            </div>
          )}

          {type === "env_var" && (
            <div className="space-y-3">
              <FormRow
                label="Group label"
                required
                hint="Environment bucket. Lowercase + hyphens only (e.g. prod, staging, prod-eu-1)."
              >
                <Input
                  value={groupLabel}
                  onChange={(e) => setGroupLabel(e.target.value.toLowerCase())}
                  placeholder="prod"
                />
              </FormRow>

              <div className="space-y-2">
                <div className="grid grid-cols-12 gap-2 text-[10px] font-medium text-[var(--text-faint)] uppercase tracking-wider">
                  <span className="col-span-4">Name</span>
                  <span className="col-span-4">Value</span>
                  <span className="col-span-3">Description</span>
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
                        placeholder="value"
                      />
                    </div>
                    <div className="col-span-3">
                      <Input
                        value={row.description}
                        onChange={(e) => updateRow(idx, { description: e.target.value })}
                        placeholder="(optional)"
                      />
                    </div>
                    <div className="col-span-1 flex justify-end">
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => removeRow(idx)}
                      >
                        ✕
                      </Button>
                    </div>
                  </div>
                ))}
              </div>

              <div className="flex items-center gap-2">
                <Button type="button" size="sm" variant="secondary" onClick={addRow}>
                  + Add var
                </Button>
                <p className="text-[10px] text-[var(--text-faint)]">
                  All vars commit in one transaction — partial failure rolls back the whole batch.
                </p>
              </div>
            </div>
          )}
        </Card>

        <div className="flex items-center gap-2">
          <Button type="submit" size="sm" disabled={create.isPending}>
            {create.isPending ? "Saving..." : type === "env_var" ? "Save bundle" : "Create secret"}
          </Button>
          <Button type="button" size="sm" variant="ghost" onClick={handleClose}>
            Cancel
          </Button>
          {create.isError && (
            <span className="text-xs text-red-400">{(create.error as Error).message}</span>
          )}
        </div>
      </form>
    </ResponsiveModal>
  );
}
