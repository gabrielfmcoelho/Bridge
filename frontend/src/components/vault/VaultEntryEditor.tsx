"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import FormError from "@/components/ui/FormError";
import type { Secret } from "@/lib/types";

// VaultEntryEditor edits an existing secret in place. type/scope/visibility are
// immutable (the update endpoint only changes name/description/group_label/
// payload), so they're shown read-only. The current value is revealed on open
// and pre-filled into the type-specific fields so the operator can rotate it.
export default function VaultEntryEditor({ secret, onClose }: { secret: Secret | null; onClose: () => void }) {
  const qc = useQueryClient();
  const open = !!secret;

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [groupLabel, setGroupLabel] = useState("");
  // per-type payload fields
  const [valueField, setValueField] = useState("");
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
  const [error, setError] = useState<string | null>(null);

  // Reveal the current payload so we can pre-fill the editable value fields.
  const { data: reveal, isLoading: revealing } = useQuery({
    queryKey: ["secret-reveal-edit", secret?.id],
    queryFn: () => secretsAPI.reveal(secret!.id),
    enabled: open,
  });

  /* eslint-disable react-hooks/set-state-in-effect -- intentionally prefilling
     form fields from the async reveal payload; this is the external→state sync
     the rule over-flags. */
  useEffect(() => {
    if (!secret) return;
    setName(secret.name);
    setDescription(secret.description ?? "");
    setGroupLabel(secret.group_label ?? "");
    setError(null);
    if (!reveal) return;
    let p: Record<string, unknown> = {};
    try {
      p = JSON.parse(reveal.payload);
    } catch {
      p = { value: reveal.payload };
    }
    const str = (k: string) => (typeof p[k] === "string" ? (p[k] as string) : "");
    setValueField(str("value"));
    setCredUsername(str("username"));
    setCredPassword(str("password"));
    setSshUsername(str("username"));
    setSshPrivKey(str("private_key_pem"));
    setSshPubKey(str("public_key"));
    setAppName(str("app_name"));
    setAppURL(str("url"));
    setAppUsername(str("username"));
    setAppPassword(str("password"));
    setAppNotes(str("notes"));
  }, [secret, reveal]);
  /* eslint-enable react-hooks/set-state-in-effect */

  function buildPayload(): string {
    if (!secret) return "";
    switch (secret.type) {
      case "password":
        return JSON.stringify({ value: valueField });
      case "cred":
        return JSON.stringify({ username: credUsername, password: credPassword });
      case "sshkey":
        return JSON.stringify({ username: sshUsername, private_key_pem: sshPrivKey, public_key: sshPubKey });
      case "app_login": {
        const obj: Record<string, string> = { app_name: appName, username: appUsername, password: appPassword };
        if (appURL) obj.url = appURL;
        if (appNotes) obj.notes = appNotes;
        return JSON.stringify(obj);
      }
      case "env_var":
        return JSON.stringify(description ? { value: valueField, description } : { value: valueField });
      default:
        return JSON.stringify({ value: valueField });
    }
  }

  function validate(): string | null {
    if (!secret) return "no secret";
    if (!name.trim()) return "Name is required.";
    switch (secret.type) {
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
        if (!appName || !appUsername || !appPassword) return "App name, username, and password are required.";
        break;
    }
    return null;
  }

  const save = useMutation({
    mutationFn: async () => {
      const reason = validate();
      if (reason) throw new Error(reason);
      const body: { name?: string; description?: string; group_label?: string; payload?: string } = {
        name: name.trim(),
        description: description.trim() || undefined,
        payload: buildPayload(),
      };
      if (secret!.type === "env_var") body.group_label = groupLabel.trim() || undefined;
      return secretsAPI.update(secret!.id, body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["secrets-all"] });
      onClose();
    },
    onError: (e) => setError(e instanceof Error ? e.message : String(e)),
  });

  if (!secret) return null;

  return (
    <ResponsiveModal open={open} onClose={onClose} title="Edit secret">
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          setError(null);
          save.mutate();
        }}
      >
        <div className="flex items-center gap-2 flex-wrap">
          <Badge>{secret.type}</Badge>
          <Badge color={secret.visibility === "personal" ? "purple" : "amber"}>{secret.visibility}</Badge>
          <Badge>{secret.scope}</Badge>
          <span className="text-[10px] text-[var(--text-faint)]">type, scope &amp; visibility can&apos;t be changed</span>
        </div>

        <label className="text-xs font-medium text-[var(--text-muted)] block">
          {secret.type === "env_var" ? "Variable name" : "Name"}
          <Input value={name} onChange={(e) => setName(e.target.value)} className="mt-1" />
        </label>

        {secret.type === "env_var" && (
          <label className="text-xs font-medium text-[var(--text-muted)] block">
            Group (environment)
            <Input value={groupLabel} onChange={(e) => setGroupLabel(e.target.value)} className="mt-1" />
          </label>
        )}

        <label className="text-xs font-medium text-[var(--text-muted)] block">
          Description
          <Input value={description} onChange={(e) => setDescription(e.target.value)} className="mt-1" />
        </label>

        <Card>
          {revealing ? (
            <p className="text-xs text-[var(--text-muted)]">Revealing current value…</p>
          ) : (
            <div className="space-y-2">
              {(secret.type === "password" || secret.type === "env_var") && (
                <ValueField label="Value" value={valueField} onChange={setValueField} />
              )}
              {secret.type === "cred" && (
                <>
                  <ValueField label="Username" value={credUsername} onChange={setCredUsername} />
                  <ValueField label="Password" value={credPassword} onChange={setCredPassword} />
                </>
              )}
              {secret.type === "sshkey" && (
                <>
                  <ValueField label="Username" value={sshUsername} onChange={setSshUsername} />
                  <ValueField label="Private key (PEM)" value={sshPrivKey} onChange={setSshPrivKey} textarea />
                  <ValueField label="Public key" value={sshPubKey} onChange={setSshPubKey} textarea />
                </>
              )}
              {secret.type === "app_login" && (
                <>
                  <ValueField label="App name" value={appName} onChange={setAppName} />
                  <ValueField label="URL" value={appURL} onChange={setAppURL} />
                  <ValueField label="Username" value={appUsername} onChange={setAppUsername} />
                  <ValueField label="Password" value={appPassword} onChange={setAppPassword} />
                  <ValueField label="Notes" value={appNotes} onChange={setAppNotes} textarea />
                </>
              )}
            </div>
          )}
        </Card>

        {error && <FormError message={error} />}

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="ghost" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={save.isPending} disabled={revealing}>
            Save changes
          </Button>
        </div>
      </form>
    </ResponsiveModal>
  );
}

function ValueField({
  label,
  value,
  onChange,
  textarea,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  textarea?: boolean;
}) {
  return (
    <label className="text-xs font-medium text-[var(--text-muted)] block">
      {label}
      {textarea ? (
        <textarea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          rows={4}
          className="mt-1 w-full rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-surface)] p-2 font-mono text-xs text-[var(--text-primary)]"
        />
      ) : (
        <Input value={value} onChange={(e) => onChange(e.target.value)} className="mt-1 font-mono text-xs" />
      )}
    </label>
  );
}
