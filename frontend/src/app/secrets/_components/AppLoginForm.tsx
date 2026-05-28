"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";

interface AppLoginFormProps {
  // Defaults to avulso/personal — the most common shape for an app login
  // (a user storing their own credentials for an external app like Jira).
  // Callers attaching the secret to a service/host/tool override these.
  scope?: "service" | "host" | "tool" | "avulso";
  parentID?: number;
  visibility?: "shared" | "personal";
  // Optional: refetch this query key on successful save (e.g. the parent
  // page's /api/secrets list).
  invalidateQueryKey?: unknown[];
  onCreated?: (id: number) => void;
}

// FormRow is a thin label+input wrapper. Inlined here because the ui/Field
// component is a read-only display, not a form input wrapper.
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

// AppLoginForm builds a spec section-4.2 app_login payload and POSTs to
// /api/secrets. Required: app_name, username, password. Optional: url,
// notes. Validation matches ValidateAppLoginPayload on the backend so the
// browser surfaces the same rejection messages without a server round trip.
export default function AppLoginForm({
  scope = "avulso",
  parentID,
  visibility = "personal",
  invalidateQueryKey,
  onCreated,
}: AppLoginFormProps) {
  const qc = useQueryClient();

  const [appName, setAppName] = useState("");
  const [url, setUrl] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [notes, setNotes] = useState("");
  const [name, setName] = useState(""); // Secret.name (human label), defaults to app_name

  const create = useMutation({
    mutationFn: async () => {
      // Build the spec 4.2 app_login JSON shape. Keys with empty values are
      // omitted to keep the payload minimal — the optional fields are
      // tolerated on the server side but absence is cleaner than empty.
      const payload: Record<string, string> = {
        app_name: appName,
        username,
        password,
      };
      if (url) payload.url = url;
      if (notes) payload.notes = notes;

      const res = await secretsAPI.create({
        type: "app_login",
        scope,
        visibility,
        parent_id: parentID,
        name: (name || appName).trim(),
        payload: JSON.stringify(payload),
      });
      return res.id;
    },
    onSuccess: (id) => {
      if (invalidateQueryKey) {
        qc.invalidateQueries({ queryKey: invalidateQueryKey });
      }
      onCreated?.(id);
      // Reset for next entry — the form is typically used inline in a
      // detail panel where successive adds are common.
      setAppName("");
      setUrl("");
      setUsername("");
      setPassword("");
      setNotes("");
      setName("");
    },
  });

  const canSubmit = appName.trim() !== "" && username.trim() !== "" && password.trim() !== "";

  return (
    <Card>
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) create.mutate();
        }}
      >
        <FormRow label="App name" required>
          <Input value={appName} onChange={(e) => setAppName(e.target.value)} placeholder="Jira" required />
        </FormRow>

        <FormRow label="URL" hint="Optional — link to the app's login page">
          <Input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://example.atlassian.net"
            type="url"
          />
        </FormRow>

        <FormRow label="Username" required>
          <Input value={username} onChange={(e) => setUsername(e.target.value)} required autoComplete="off" />
        </FormRow>

        <FormRow label="Password" required>
          <Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="new-password"
          />
        </FormRow>

        <FormRow label="Notes" hint="Free-form, encrypted alongside the credentials">
          <Input value={notes} onChange={(e) => setNotes(e.target.value)} />
        </FormRow>

        <FormRow label="Display name" hint="Defaults to App name if left blank">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={appName} />
        </FormRow>

        <div className="flex items-center gap-2 pt-1">
          <Button type="submit" size="sm" disabled={!canSubmit || create.isPending}>
            {create.isPending ? "Saving..." : "Save app login"}
          </Button>
          {create.isError && <span className="text-xs text-red-400">{(create.error as Error).message}</span>}
          {create.isSuccess && <span className="text-xs text-emerald-400">Saved.</span>}
        </div>
      </form>
    </Card>
  );
}
