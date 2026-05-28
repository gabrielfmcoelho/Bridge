"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
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

// NewSecretModal — single entry point for adding any of the five secret
// types via the unified /api/secrets POST endpoint. Payload shape is
// per-type (spec §4.2); this component builds the JSON envelope and
// hands it to secretsAPI.create.
//
// Defaults bias toward "personal avulso" because that's the most common
// shape for a quick personal note. Operators creating shared secrets
// against a service/host/tool fill the parent_id field.
export default function NewSecretModal({ open, onClose }: NewSecretModalProps) {
  const qc = useQueryClient();

  // Common metadata fields.
  const [type, setType] = useState<SecretType>("password");
  const [scope, setScope] = useState<Scope>("avulso");
  const [visibility, setVisibility] = useState<Visibility>("personal");
  const [parentID, setParentID] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  // Per-type payload fields. Only the ones relevant to the active type
  // contribute to the final JSON.
  const [valueField, setValueField] = useState(""); // password / env_var.value
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
  const [groupLabel, setGroupLabel] = useState(""); // env_var

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
        return JSON.stringify({ value: valueField });
    }
  };

  // Per-type validation that runs before mutating. Matches the backend's
  // ValidateEnvVarName / ValidateAppLoginPayload (Phase 2 Task 2.1 + 2.4)
  // so users see the same rejection messages without a server round-trip
  // for the obvious cases.
  const validate = (): string | null => {
    if (!name.trim()) return "Name is required.";
    if (scope !== "avulso" && !parentID.trim()) {
      return `Parent ID is required when scope is ${scope}.`;
    }
    switch (type) {
      case "password":
      case "env_var":
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
    if (type === "env_var") {
      if (!/^[A-Z_][A-Z0-9_]*$/.test(name)) {
        return "env_var name must match ^[A-Z_][A-Z0-9_]*$ (uppercase only).";
      }
      if (!groupLabel || !/^[a-z][a-z0-9-]*$/.test(groupLabel)) {
        return "env_var group label must match ^[a-z][a-z0-9-]*$ (lowercase only).";
      }
    }
    return null;
  };

  const create = useMutation({
    mutationFn: async () => {
      const reason = validate();
      if (reason) throw new Error(reason);
      const body: Parameters<typeof secretsAPI.create>[0] = {
        type,
        scope,
        visibility,
        name: name.trim(),
        description: description.trim() || undefined,
        payload: buildPayload(),
      };
      if (scope !== "avulso") {
        body.parent_id = Number(parentID);
      }
      if (type === "env_var") {
        body.group_label = groupLabel.trim();
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
                  { value: "env_var", label: "env_var" },
                ]}
              />
            </FormRow>
            <FormRow label="Scope" required>
              <Select
                value={scope}
                onChange={(e) => setScope(e.target.value as Scope)}
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
              <FormRow
                label={`Parent ${scope} ID`}
                required
                hint="Numeric id of the parent service/host/tool this secret attaches to."
              >
                <Input
                  type="number"
                  value={parentID}
                  onChange={(e) => setParentID(e.target.value)}
                  placeholder="e.g. 42"
                />
              </FormRow>
            </div>
          )}

          <div className="mt-3 space-y-3">
            <FormRow
              label="Name"
              required
              hint={
                type === "env_var"
                  ? "Uppercase only (e.g. DB_URL). Matches ^[A-Z_][A-Z0-9_]*$."
                  : "Human-readable label."
              }
            >
              <Input
                value={name}
                onChange={(e) => setName(type === "env_var" ? e.target.value.toUpperCase() : e.target.value)}
                placeholder={type === "env_var" ? "DB_URL" : "primary"}
              />
            </FormRow>
            <FormRow label="Description" hint="Optional. Visible to anyone who can see the secret metadata.">
              <Input value={description} onChange={(e) => setDescription(e.target.value)} />
            </FormRow>
          </div>
        </Card>

        {/* Per-type payload section */}
        <Card>
          <h3 className="text-sm font-semibold mb-3">Payload</h3>
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
              <FormRow label="Value" required>
                <Input
                  type="password"
                  value={valueField}
                  onChange={(e) => setValueField(e.target.value)}
                  autoComplete="new-password"
                />
              </FormRow>
              <p className="text-[10px] text-[var(--text-faint)]">
                Tip: for bulk env-var saves use the EnvVarBundleEditor (multi-var tabbed view).
              </p>
            </div>
          )}
        </Card>

        <div className="flex items-center gap-2">
          <Button type="submit" size="sm" disabled={create.isPending}>
            {create.isPending ? "Saving..." : "Create secret"}
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
