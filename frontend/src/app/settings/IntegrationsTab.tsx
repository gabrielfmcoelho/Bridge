"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useEffect, useRef } from "react";
import { integrationsAPI, coolifyAPI, glpiAPI, outlineAPI, type GlpiTokenProfile } from "@/lib/api";
import CollectionMultiSelect from "@/components/wiki/CollectionMultiSelect";
import DropdownCatalogueSection from "@/components/glpi/DropdownCatalogueSection";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Toggle from "@/components/ui/Toggle";

export default function IntegrationsTab() {
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const activeProvider = data?.general?.auth_active_provider ?? "local";

  return (
    <div className="space-y-6">
      <GeneralAuthSection />
      {activeProvider === "ldap" && <LDAPSection />}
      {activeProvider === "keycloak" && <KeycloakSection />}
      <GitLabIntegrationSection ssoActive={activeProvider === "gitlab"} />
      <GrafanaIntegrationSection />
      <OutlineIntegrationSection />
      <GLPIIntegrationSection />
      <LLMSection />
      <CoolifySection />
    </div>
  );
}

const AUTH_PROVIDERS = [
  { value: "local", label: "Local Only", description: "Username & password stored locally", color: "#06b6d4" },
  { value: "ldap", label: "LDAP", description: "Authenticate against institutional LDAP directory", color: "#3b82f6" },
  { value: "keycloak", label: "Keycloak SSO", description: "PI Login via OAuth 2.0 / OpenID Connect", color: "#22c55e" },
  { value: "gitlab", label: "GitLab SSO", description: "Authenticate via GitLab OAuth", color: "#e24329" },
];

function GeneralAuthSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});

  useEffect(() => {
    if (data?.general) setForm(data.general);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("general", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));
  const activeProvider = form.auth_active_provider ?? "local";

  return (
    <Card>
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-1">Authentication Provider</h3>
      <p className="text-xs text-[var(--text-muted)] mb-4">Only one external auth provider can be active at a time. Local login is always available as fallback.</p>

      {/* Provider radio selector */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-2 mb-5">
        {AUTH_PROVIDERS.map((p) => (
          <label
            key={p.value}
            className={`flex items-start gap-3 p-3 rounded-[var(--radius-md)] border cursor-pointer transition-all ${
              activeProvider === p.value
                ? "border-[var(--accent)] bg-[var(--accent-muted)]"
                : "border-[var(--border-default)] bg-[var(--bg-elevated)] hover:border-[var(--border-hover)]"
            }`}
          >
            <input
              type="radio"
              name="auth_provider"
              value={p.value}
              checked={activeProvider === p.value}
              onChange={() => set("auth_active_provider", p.value)}
              className="mt-0.5 accent-[var(--accent)]"
            />
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: p.color }} />
                <span className="text-sm font-medium text-[var(--text-primary)]">{p.label}</span>
              </div>
              <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{p.description}</p>
            </div>
          </label>
        ))}
      </div>

      {/* General auth settings (only shown when external provider is active) */}
      {activeProvider !== "local" && (
        <div className="border-t border-[var(--border-subtle)] pt-4 mt-4">
          <p className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">External Auth Settings</p>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Auto-provision users</label>
              <select
                value={form.auth_auto_provision ?? "true"}
                onChange={(e) => set("auth_auto_provision", e.target.value)}
                className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
              >
                <option value="true">Enabled</option>
                <option value="false">Disabled</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Default role for new users</label>
              <select
                value={form.auth_default_role ?? "viewer"}
                onChange={(e) => set("auth_default_role", e.target.value)}
                className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
              >
                <option value="viewer">Viewer</option>
                <option value="editor">Editor</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Sync roles from external groups</label>
              <select
                value={form.auth_role_sync_enabled ?? "false"}
                onChange={(e) => set("auth_role_sync_enabled", e.target.value)}
                className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
              >
                <option value="false">Disabled</option>
                <option value="true">Enabled</option>
              </select>
            </div>
          </div>
        </div>
      )}

      <div className="mt-4 flex justify-end">
        <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
          Save
        </Button>
      </div>
    </Card>
  );
}

function LDAPSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});
  const [testResult, setTestResult] = useState<{ success: boolean; error?: string } | null>(null);

  useEffect(() => {
    if (data?.ldap) setForm(data.ldap);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("ldap", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const testMutation = useMutation({
    mutationFn: integrationsAPI.testLDAP,
    onSuccess: (result) => setTestResult(result),
    onError: () => setTestResult({ success: false, error: "Request failed" }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));

  return (
    <Card>
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4">LDAP Configuration</h3>
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label="Host"
              value={form.auth_ldap_host ?? ""}
              onChange={(e) => set("auth_ldap_host", e.target.value)}
              placeholder="ldaps://ldap.example.com"
            />
            <Input
              label="Port"
              value={form.auth_ldap_port ?? "636"}
              onChange={(e) => set("auth_ldap_port", e.target.value)}
              placeholder="636"
            />
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">TLS</label>
                <select
                  value={form.auth_ldap_use_tls ?? "true"}
                  onChange={(e) => set("auth_ldap_use_tls", e.target.value)}
                  className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
                >
                  <option value="true">Yes</option>
                  <option value="false">No</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">Skip Verify</label>
                <select
                  value={form.auth_ldap_skip_verify ?? "false"}
                  onChange={(e) => set("auth_ldap_skip_verify", e.target.value)}
                  className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
                >
                  <option value="false">No</option>
                  <option value="true">Yes</option>
                </select>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Base DN"
              value={form.auth_ldap_base_dn ?? ""}
              onChange={(e) => set("auth_ldap_base_dn", e.target.value)}
              placeholder="dc=example,dc=com"
            />
            <Input
              label="Bind DN"
              value={form.auth_ldap_bind_dn ?? ""}
              onChange={(e) => set("auth_ldap_bind_dn", e.target.value)}
              placeholder="cn=admin,dc=example,dc=com"
            />
          </div>

          <Input
            label="Bind Password"
            type="password"
            value={form.auth_ldap_bind_password ?? ""}
            onChange={(e) => set("auth_ldap_bind_password", e.target.value)}
            placeholder="••••••••"
          />

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="User Filter"
              value={form.auth_ldap_user_filter ?? "(mail=%s)"}
              onChange={(e) => set("auth_ldap_user_filter", e.target.value)}
              placeholder="(mail=%s)"
            />
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">Fallback to local auth</label>
              <select
                value={form.auth_ldap_fallback_to_local ?? "true"}
                onChange={(e) => set("auth_ldap_fallback_to_local", e.target.value)}
                className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
              >
                <option value="true">Enabled</option>
                <option value="false">Disabled</option>
              </select>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label="Username Attribute"
              value={form.auth_ldap_username_attr ?? "uid"}
              onChange={(e) => set("auth_ldap_username_attr", e.target.value)}
              placeholder="uid"
            />
            <Input
              label="Display Name Attribute"
              value={form.auth_ldap_display_name_attr ?? "cn"}
              onChange={(e) => set("auth_ldap_display_name_attr", e.target.value)}
              placeholder="cn"
            />
            <Input
              label="Email Attribute"
              value={form.auth_ldap_email_attr ?? "mail"}
              onChange={(e) => set("auth_ldap_email_attr", e.target.value)}
              placeholder="mail"
            />
          </div>

          <div className="flex items-center gap-3">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              Save LDAP Settings
            </Button>
            <Button
              variant="secondary"
              onClick={() => { setTestResult(null); testMutation.mutate(); }}
              loading={testMutation.isPending}
            >
              Test Connection
            </Button>
            {testResult && (
              <span className={`text-sm ${testResult.success ? "text-emerald-400" : "text-red-400"}`}>
                {testResult.success ? "Connection successful" : testResult.error}
              </span>
            )}
          </div>
        </div>
    </Card>
  );
}

function KeycloakSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});

  useEffect(() => {
    if (data?.keycloak) setForm(data.keycloak);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("keycloak", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));

  // Environment presets from PI Login docs.
  const presets: Record<string, { url: string; realm: string }> = {
    dev: { url: "https://dev-login.pi.gov.br", realm: "pi" },
    homolog: { url: "https://homolog-login.pi.gov.br", realm: "pi" },
    prod: { url: "https://login.pi.gov.br", realm: "pi" },
  };

  const applyPreset = (env: string) => {
    const preset = presets[env];
    if (preset) {
      setForm((f) => ({
        ...f,
        auth_keycloak_base_url: preset.url,
        auth_keycloak_realm: preset.realm,
      }));
    }
  };

  // Derive the callback URL to display.
  const callbackURL = typeof window !== "undefined"
    ? `${window.location.origin}/api/auth/oauth/keycloak/callback`
    : "/api/auth/oauth/keycloak/callback";

  return (
    <Card>
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4">Keycloak SSO Configuration</h3>
        <div className="space-y-4">
          {/* Environment presets */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1.5">Environment Presets</label>
            <div className="flex gap-2">
              {Object.keys(presets).map((env) => (
                <button
                  key={env}
                  type="button"
                  onClick={() => applyPreset(env)}
                  className="px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-overlay)] border border-[var(--border-default)] transition-all capitalize"
                >
                  {env}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Base URL"
              value={form.auth_keycloak_base_url ?? ""}
              onChange={(e) => set("auth_keycloak_base_url", e.target.value)}
              placeholder="https://login.pi.gov.br"
            />
            <Input
              label="Realm"
              value={form.auth_keycloak_realm ?? "pi"}
              onChange={(e) => set("auth_keycloak_realm", e.target.value)}
              placeholder="pi"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Client ID"
              value={form.auth_keycloak_client_id ?? ""}
              onChange={(e) => set("auth_keycloak_client_id", e.target.value)}
              placeholder="my-app-client"
            />
            <Input
              label="Client Secret"
              type="password"
              value={form.auth_keycloak_client_secret ?? ""}
              onChange={(e) => set("auth_keycloak_client_secret", e.target.value)}
              placeholder="••••••••"
            />
          </div>

          {/* Callback URL (read-only) */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">Redirect URI (add this to Keycloak client)</label>
            <div className="flex items-center gap-2">
              <code className="flex-1 bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-xs text-[var(--text-secondary)] font-mono break-all">
                {callbackURL}
              </code>
            </div>
          </div>

          <div className="flex justify-start">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              Save Keycloak Settings
            </Button>
          </div>
        </div>
    </Card>
  );
}

function GitLabIntegrationSection({ ssoActive }: { ssoActive: boolean }) {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });

  // Form state + a snapshot of the server-loaded values so Cancel can revert and
  // Save can compute the dirty subset (only those fields are PUT — an untouched
  // secret is never transmitted, which prevents the wipe-on-save bug end-to-end).
  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [mode, setMode] = useState<"view" | "edit">("view");
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const initialRef = useRef<Record<string, string>>({});

  useEffect(() => {
    if (data?.gitlab) {
      setForm(data.gitlab);
      initialRef.current = { ...data.gitlab };
      setDirty({});
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("gitlab", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["integrations"] });
      setDirty({});
      setMode("view");
    },
  });

  const clearSecretMutation = useMutation({
    mutationFn: (key: string) => integrationsAPI.clearSecret("gitlab", key),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const testMutation = useMutation({
    mutationFn: () => {
      const typedToken = form.gitlab_code_service_token ?? "";
      const isMaskedPlaceholder = typedToken === "" || typedToken === "••••••••";
      return integrationsAPI.testGitLabCode({
        base_url: form.auth_gitlab_base_url ?? "",
        token: isMaskedPlaceholder ? "" : typedToken,
      });
    },
    onSuccess: (res) => {
      setTestResult({
        success: res.success,
        message: res.success
          ? `Connected as ${res.username ?? "(unknown)"}`
          : res.error || "Connection failed",
      });
    },
    onError: (err: Error) => setTestResult({ success: false, message: err.message }),
  });

  const set = (key: string, value: string) => {
    setForm((f) => ({ ...f, [key]: value }));
    // Only mark dirty if the value actually differs from the initial snapshot —
    // re-typing the same value shouldn't create a dirty field.
    setDirty((d) => {
      const isDifferent = value !== (initialRef.current[key] ?? "");
      if (d[key] === isDifferent) return d;
      return { ...d, [key]: isDifferent };
    });
  };

  const handleCancel = () => {
    setForm({ ...initialRef.current });
    setDirty({});
    setTestResult(null);
    setMode("view");
  };

  const handleSave = () => {
    // Only send fields the user actually changed — unchanged secrets stay untouched on the server.
    const payload: Record<string, string> = {};
    for (const [key, changed] of Object.entries(dirty)) {
      if (changed) payload[key] = form[key] ?? "";
    }
    if (Object.keys(payload).length === 0) {
      setMode("view");
      return;
    }
    mutation.mutate(payload);
  };

  const handleClearSecret = (key: string, label: string) => {
    if (!confirm(`Clear the stored ${label}? This cannot be undone — you'll need to re-enter it.`)) return;
    clearSecretMutation.mutate(key);
    // Also reset the in-form value so the placeholder reflects "no secret".
    setForm((f) => ({ ...f, [key]: "" }));
  };

  const isView = mode === "view";
  const isEdit = mode === "edit";
  const hasDirty = Object.values(dirty).some(Boolean);

  const ssoEnabled = form.auth_gitlab_enabled === "true";
  const codeEnabled = form.gitlab_integration_enabled === "true";

  const callbackURL = typeof window !== "undefined"
    ? `${window.location.origin}/api/auth/oauth/gitlab/callback`
    : "/api/auth/oauth/gitlab/callback";

  return (
    <Card>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">GitLab Integration</h3>
          <p className="text-[11px] text-[var(--text-muted)] mt-0.5">
            Shared connection credentials used by every GitLab-backed feature below.
          </p>
        </div>
        {isView && (
          <Button size="sm" variant="secondary" onClick={() => setMode("edit")}>
            Edit
          </Button>
        )}
      </div>

      {/* Shared credentials */}
      <div className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Input
            label="GitLab Base URL"
            value={form.auth_gitlab_base_url ?? "https://gitlab.com"}
            onChange={(e) => set("auth_gitlab_base_url", e.target.value)}
            placeholder="https://gitlab.com"
            disabled={isView}
          />
          <Input
            label="Application ID (Client ID)"
            value={form.auth_gitlab_client_id ?? ""}
            onChange={(e) => set("auth_gitlab_client_id", e.target.value)}
            placeholder="your-app-id"
            disabled={isView}
          />
        </div>
        <SecretInputWithClear
          label="Client Secret"
          value={form.auth_gitlab_client_secret ?? ""}
          onChange={(v) => set("auth_gitlab_client_secret", v)}
          disabled={isView}
          canClear={isEdit && (initialRef.current.auth_gitlab_client_secret === "••••••••")}
          onClear={() => handleClearSecret("auth_gitlab_client_secret", "Client Secret")}
        />
      </div>

      {/* SSO subsection */}
      <details className="mt-5 border-t border-[var(--border-default)] pt-4" open={ssoActive || ssoEnabled}>
        <summary className="cursor-pointer list-none flex items-center justify-between gap-3">
          <div className="min-w-0">
            <span className="text-sm font-semibold text-[var(--text-primary)]">SSO / Authentication</span>
            <span className="ml-2 text-[11px] text-[var(--text-muted)]">
              {ssoEnabled
                ? (ssoActive ? "Active — GitLab is the selected provider" : "Enabled — select GitLab above to make it active")
                : "Disabled"}
            </span>
          </div>
          <Toggle
            checked={ssoEnabled}
            onChange={(v) => set("auth_gitlab_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel="Enable GitLab SSO"
          />
        </summary>
        <div className="mt-4 space-y-3">
          <label className="block text-xs text-[var(--text-muted)] mb-1">Redirect URI (add this to the GitLab application)</label>
          <code className="block bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-xs text-[var(--text-secondary)] font-mono break-all">
            {callbackURL}
          </code>
          <p className="text-xs text-[var(--text-muted)]">
            Uses the shared Application ID and Client Secret above. Requires <code className="text-[var(--text-secondary)]">read_user</code> and <code className="text-[var(--text-secondary)]">api</code> scopes.
            Login via GitLab is blocked unless both this switch is ON and GitLab is selected in the General Auth provider.
          </p>
        </div>
      </details>

      {/* Code Management subsection */}
      <details className="mt-4 border-t border-[var(--border-default)] pt-4" open={codeEnabled}>
        <summary className="cursor-pointer list-none flex items-center justify-between gap-3">
          <div className="min-w-0">
            <span className="text-sm font-semibold text-[var(--text-primary)]">Code Management</span>
            <span className="ml-2 text-[11px] text-[var(--text-muted)]">
              Track commits across linked GitLab repos and subgroups per project.
            </span>
          </div>
          <Toggle
            checked={codeEnabled}
            onChange={(v) => set("gitlab_integration_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel="Enable GitLab Code Management"
          />
        </summary>
        {codeEnabled && (
          <div className="mt-4 space-y-4">
            <SecretInputWithClear
              label="Service Access Token"
              value={form.gitlab_code_service_token ?? ""}
              onChange={(v) => set("gitlab_code_service_token", v)}
              disabled={isView}
              canClear={isEdit && (initialRef.current.gitlab_code_service_token === "••••••••")}
              onClear={() => handleClearSecret("gitlab_code_service_token", "Service Access Token")}
            />
            <p className="text-xs text-[var(--text-muted)] -mt-2">
              A Personal / Group / Project Access Token with <code className="text-[var(--text-secondary)]">read_api</code> scope. Used for all REST calls — one token for every viewer.
            </p>
            <Input
              label="Default Branch (optional)"
              value={form.gitlab_code_default_ref ?? ""}
              onChange={(e) => set("gitlab_code_default_ref", e.target.value)}
              placeholder="main"
              disabled={isView}
            />
            <div className="flex items-center gap-3">
              <Button
                variant="secondary"
                onClick={() => { setTestResult(null); testMutation.mutate(); }}
                loading={testMutation.isPending}
              >
                Test connection
              </Button>
              {testResult && (
                <span className={`text-xs ${testResult.success ? "text-green-400" : "text-red-400"}`}>
                  {testResult.message}
                </span>
              )}
            </div>
          </div>
        )}
      </details>

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-[11px] text-[var(--text-muted)]">
          {isView ? "Read-only — click Edit to change settings." : hasDirty ? "Unsaved changes." : "No changes."}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button variant="secondary" onClick={handleCancel}>Cancel</Button>
              <Button onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                Save changes
              </Button>
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

// SecretInputWithClear renders a masked password input plus an optional small Clear
// button shown only in edit mode when the server already had a value stored. Clear
// goes through the dedicated DELETE endpoint so stored ciphers are only wiped on
// explicit admin intent — never as a side-effect of saving.
function parseCollectionCSV(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

function OutlineIntegrationSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [mode, setMode] = useState<"view" | "edit">("view");
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [fallbackToText, setFallbackToText] = useState(false);
  const initialRef = useRef<Record<string, string>>({});

  // Collections for the multiselect — loaded only when the integration is already
  // enabled+configured, since the endpoint requires a working token to hit Outline.
  const outlineReady = data?.outline?.outline_enabled === "true"
    && !!data?.outline?.outline_base_url
    && data?.outline?.outline_api_token === "••••••••";

  const { data: collectionsEnv, isLoading: collectionsLoading, isError: collectionsError } = useQuery({
    queryKey: ["outline-workspace-collections"],
    queryFn: outlineAPI.listWorkspaceCollections,
    enabled: outlineReady,
    retry: false,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (data?.outline) {
      setForm(data.outline);
      initialRef.current = { ...data.outline };
      setDirty({});
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("outline", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["integrations"] });
      setDirty({});
      setMode("view");
    },
  });

  const clearSecretMutation = useMutation({
    mutationFn: (key: string) => integrationsAPI.clearSecret("outline", key),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const testMutation = useMutation({
    mutationFn: () => {
      const typed = form.outline_api_token ?? "";
      const masked = typed === "" || typed === "••••••••";
      return integrationsAPI.testOutline({
        base_url: form.outline_base_url ?? "",
        token: masked ? "" : typed,
      });
    },
    onSuccess: (res) => {
      if (!res.success) {
        setTestResult({ success: false, message: `Connection failed: ${res.error || "unknown error"}` });
        return;
      }
      const parts: string[] = [];
      if (res.workspace) parts.push(res.workspace);
      if (res.user) parts.push(`as ${res.user}`);
      setTestResult({ success: true, message: parts.join(" · ") || "Connected" });
    },
    onError: (err: Error) => setTestResult({ success: false, message: err.message }),
  });

  const set = (key: string, value: string) => {
    setForm((f) => ({ ...f, [key]: value }));
    setDirty((d) => {
      const isDifferent = value !== (initialRef.current[key] ?? "");
      if (d[key] === isDifferent) return d;
      return { ...d, [key]: isDifferent };
    });
  };

  const handleCancel = () => {
    setForm({ ...initialRef.current });
    setDirty({});
    setTestResult(null);
    setMode("view");
  };

  const handleSave = () => {
    const payload: Record<string, string> = {};
    for (const [key, changed] of Object.entries(dirty)) {
      if (changed) payload[key] = form[key] ?? "";
    }
    if (Object.keys(payload).length === 0) {
      setMode("view");
      return;
    }
    mutation.mutate(payload);
  };

  const handleClearSecret = (key: string, label: string) => {
    if (!confirm(`Clear the stored ${label}? This cannot be undone — you'll need to re-enter it.`)) return;
    clearSecretMutation.mutate(key);
    setForm((f) => ({ ...f, [key]: "" }));
  };

  const isView = mode === "view";
  const isEdit = mode === "edit";
  const hasDirty = Object.values(dirty).some(Boolean);
  const enabled = form.outline_enabled === "true";

  return (
    <Card>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0 flex items-center gap-3 flex-wrap">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">Outline (wiki)</h3>
          <Toggle
            checked={enabled}
            onChange={(v) => set("outline_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel="Enable Outline integration"
          />
          <p className="text-[11px] text-[var(--text-muted)]">
            Per-project Wiki tab + site-wide common wiki at <code className="text-[var(--text-secondary)]">/wiki</code>.
          </p>
        </div>
        {isView && (
          <Button type="button" size="sm" variant="secondary" onClick={() => setMode("edit")}>
            Edit
          </Button>
        )}
      </div>

      {enabled && (
        <div className="space-y-4">
          <Input
            label="Base URL"
            value={form.outline_base_url ?? ""}
            onChange={(e) => set("outline_base_url", e.target.value)}
            placeholder="https://wiki.example.org"
            disabled={isView}
          />
          <SecretInputWithClear
            label="API Token"
            value={form.outline_api_token ?? ""}
            onChange={(v) => set("outline_api_token", v)}
            disabled={isView}
            canClear={isEdit && initialRef.current.outline_api_token === "••••••••"}
            onClear={() => handleClearSecret("outline_api_token", "API Token")}
          />
          <p className="text-[11px] text-[var(--text-muted)] -mt-2">
            Mint in Outline → Settings → API. The token inherits its creator&apos;s visibility — sshcm can only see what that user can see.
          </p>

          {collectionsError || fallbackToText ? (
            <>
              <Input
                label="Common collection IDs (comma-separated)"
                value={form.outline_common_collection_id ?? ""}
                onChange={(e) => set("outline_common_collection_id", e.target.value)}
                placeholder="uuid-1, uuid-2, uuid-3"
                disabled={isView}
              />
              <p className="text-[11px] text-[var(--text-muted)] -mt-2">
                {collectionsError
                  ? "Couldn't fetch the collection list from Outline — paste UUIDs manually. "
                  : "Manual input: "}
                Feeds the sidebar <code className="text-[var(--text-secondary)]">/wiki</code> page.{" "}
                {!collectionsError && (
                  <button
                    type="button"
                    className="text-[var(--accent)] hover:underline"
                    onClick={() => setFallbackToText(false)}
                  >
                    Switch back to picker
                  </button>
                )}
              </p>
            </>
          ) : (
            <>
              <CollectionMultiSelect
                label="Common collections"
                collections={collectionsEnv?.collections ?? []}
                value={parseCollectionCSV(form.outline_common_collection_id ?? "")}
                onChange={(ids) => set("outline_common_collection_id", ids.join(", "))}
                loading={collectionsLoading}
                disabled={isView}
                emptyHint={
                  outlineReady
                    ? "Click + to add collections to /wiki"
                    : "Save base URL + API token first, then reload to pick collections"
                }
              />
              <p className="text-[11px] text-[var(--text-muted)] -mt-2">
                Feeds the sidebar <code className="text-[var(--text-secondary)]">/wiki</code> page — each selected collection becomes a section in the left nav.{" "}
                {isEdit && (
                  <button
                    type="button"
                    className="text-[var(--accent)] hover:underline"
                    onClick={() => setFallbackToText(true)}
                  >
                    Paste UUIDs instead
                  </button>
                )}
              </p>
            </>
          )}

          <div className="flex items-center gap-3 flex-wrap">
            <Button
              type="button"
              variant="secondary"
              onClick={() => { setTestResult(null); testMutation.mutate(); }}
              loading={testMutation.isPending}
              disabled={isView}
            >
              Test connection
            </Button>
            {testResult && (
              <span className={`text-xs ${testResult.success ? "text-green-400" : "text-red-400"}`}>
                {testResult.message}
              </span>
            )}
          </div>
        </div>
      )}

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-[11px] text-[var(--text-muted)]">
          {isView ? "Read-only — click Edit to change settings." : hasDirty ? "Unsaved changes." : "No changes."}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button type="button" variant="secondary" onClick={handleCancel}>Cancel</Button>
              <Button type="button" onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                Save changes
              </Button>
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

function GLPIIntegrationSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const { data: profiles } = useQuery({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    retry: false,
  });
  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [mode, setMode] = useState<"view" | "edit">("view");
  const initialRef = useRef<Record<string, string>>({});

  useEffect(() => {
    if (data?.glpi) {
      setForm(data.glpi);
      initialRef.current = { ...data.glpi };
      setDirty({});
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("glpi", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["integrations"] });
      setDirty({});
      setMode("view");
    },
  });

  const clearSecretMutation = useMutation({
    mutationFn: (key: string) => integrationsAPI.clearSecret("glpi", key),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const set = (key: string, value: string) => {
    setForm((f) => ({ ...f, [key]: value }));
    setDirty((d) => {
      const isDifferent = value !== (initialRef.current[key] ?? "");
      if (d[key] === isDifferent) return d;
      return { ...d, [key]: isDifferent };
    });
  };

  const handleCancel = () => {
    setForm({ ...initialRef.current });
    setDirty({});
    setMode("view");
  };

  const handleSave = () => {
    const payload: Record<string, string> = {};
    for (const [key, changed] of Object.entries(dirty)) {
      if (changed) payload[key] = form[key] ?? "";
    }
    if (Object.keys(payload).length === 0) {
      setMode("view");
      return;
    }
    mutation.mutate(payload);
  };

  const handleClearSecret = (key: string, label: string) => {
    if (!confirm(`Clear the stored ${label}? This cannot be undone.`)) return;
    clearSecretMutation.mutate(key);
    setForm((f) => ({ ...f, [key]: "" }));
  };

  const isView = mode === "view";
  const isEdit = mode === "edit";
  const hasDirty = Object.values(dirty).some(Boolean);
  const enabled = form.glpi_enabled === "true";

  return (
    <Card>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0 flex items-center gap-3 flex-wrap">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">GLPI (chamados)</h3>
          <Toggle
            checked={enabled}
            onChange={(v) => set("glpi_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel="Enable GLPI integration"
          />
          <p className="text-[11px] text-[var(--text-muted)]">
            One App-Token + N named user profiles. Link a profile to each project.
          </p>
        </div>
        {isView && (
          <Button type="button" size="sm" variant="secondary" onClick={() => setMode("edit")}>
            Edit
          </Button>
        )}
      </div>

      {enabled && (
        <div className="space-y-4">
          <Input
            label="Base URL"
            value={form.glpi_base_url ?? ""}
            onChange={(e) => set("glpi_base_url", e.target.value)}
            placeholder="https://glpi.example.org"
            disabled={isView}
          />
          <SecretInputWithClear
            label="App-Token (optional)"
            value={form.glpi_app_token ?? ""}
            onChange={(v) => set("glpi_app_token", v)}
            disabled={isView}
            canClear={isEdit && initialRef.current.glpi_app_token === "••••••••"}
            onClear={() => handleClearSecret("glpi_app_token", "App-Token")}
          />
          <p className="text-[11px] text-[var(--text-muted)] -mt-2">
            GLPI → Setup → General → API. <strong>Only required when your GLPI instance demands it</strong> — many deployments accept the per-user token on its own. Leave blank if you only have user tokens.
          </p>
          <Input
            label="Default entity ID (fallback when a profile/project has none)"
            type="number"
            value={form.glpi_default_entity_id ?? "0"}
            onChange={(e) => set("glpi_default_entity_id", e.target.value)}
            disabled={isView}
          />

          <DropdownCatalogueSection />

          <details className="border-t border-[var(--border-default)] pt-4" open>
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              Token profiles ({profiles?.length ?? 0})
            </summary>
            <div className="mt-3">
              <GlpiProfileList profiles={profiles ?? []} disabled={isView} />
            </div>
          </details>
        </div>
      )}

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-[11px] text-[var(--text-muted)]">
          {isView ? "Read-only — click Edit to change." : hasDirty ? "Unsaved changes." : "No changes."}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button type="button" variant="secondary" onClick={handleCancel}>Cancel</Button>
              <Button type="button" onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                Save changes
              </Button>
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

// GlpiProfileList — inline CRUD for GLPI user-token profiles. Lives inside the
// GLPI integration card. Each profile is one named GLPI account (user-token).
function GlpiProfileList({ profiles, disabled }: { profiles: GlpiTokenProfile[]; disabled: boolean }) {
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [newToken, setNewToken] = useState("");
  const [newEntity, setNewEntity] = useState("0");
  const [error, setError] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<number, { ok: boolean; message: string }>>({});

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["glpi-profiles"] });

  const createMutation = useMutation({
    mutationFn: () => glpiAPI.createProfile({
      name: newName.trim(),
      description: newDesc.trim(),
      user_token: newToken.trim(),
      default_entity_id: parseInt(newEntity || "0", 10),
    }),
    onSuccess: () => {
      setNewName(""); setNewDesc(""); setNewToken(""); setNewEntity("0");
      setAdding(false); setError(null);
      invalidate();
    },
    onError: (err: Error) => setError(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => glpiAPI.deleteProfile(id),
    onSuccess: invalidate,
  });

  const testMutation = useMutation({
    mutationFn: (id: number) => glpiAPI.testProfile(id).then((res) => ({ id, res })),
    onSuccess: ({ id, res }) => {
      setTestResults((prev) => ({
        ...prev,
        [id]: {
          ok: res.success,
          message: res.success
            ? `Connected (${res.profiles?.slice(0, 3).join(", ") || "no profiles"})`
            : (res.error || "Failed"),
        },
      }));
    },
  });

  return (
    <div className="space-y-3">
      {profiles.length === 0 && !adding && (
        <p className="text-xs text-[var(--text-muted)]">No profiles yet. Add one to enable per-project ticket access.</p>
      )}

      {profiles.map((p) => (
        <div key={p.id} className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] px-3 py-2">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <div className="min-w-0">
              <p className="text-sm font-medium text-[var(--text-primary)]">{p.name}</p>
              {p.description && <p className="text-[11px] text-[var(--text-muted)]">{p.description}</p>}
              <p className="text-[10px] text-[var(--text-faint)] mt-0.5">
                {p.has_token ? "token stored" : "no token"} · entity #{p.default_entity_id}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                disabled={disabled || testMutation.isPending}
                onClick={() => testMutation.mutate(p.id)}
                className="text-[11px] text-[var(--accent)] hover:underline disabled:opacity-40"
              >
                Test
              </button>
              <button
                type="button"
                disabled={disabled}
                onClick={() => {
                  if (!confirm(`Delete profile "${p.name}"? Any project pointing at it will become unassigned.`)) return;
                  deleteMutation.mutate(p.id);
                }}
                className="text-[11px] text-red-400 hover:text-red-300 disabled:opacity-40"
              >
                Delete
              </button>
            </div>
          </div>
          {testResults[p.id] && (
            <p className={`text-[11px] mt-1 ${testResults[p.id].ok ? "text-green-400" : "text-red-400"}`}>
              {testResults[p.id].message}
            </p>
          )}
        </div>
      ))}

      {adding ? (
        <div className="rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] p-3 space-y-3">
          <Input label="Name" value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="e.g. Infra team" />
          <Input label="Description (optional)" value={newDesc} onChange={(e) => setNewDesc(e.target.value)} />
          <Input label="User token" type="password" value={newToken} onChange={(e) => setNewToken(e.target.value)} placeholder="GLPI personal API token" />
          <Input label="Default entity ID" type="number" value={newEntity} onChange={(e) => setNewEntity(e.target.value)} />
          {error && <p className="text-xs text-red-400">{error}</p>}
          <div className="flex gap-2">
            <Button type="button" size="sm" onClick={() => createMutation.mutate()} loading={createMutation.isPending} disabled={!newName.trim() || !newToken.trim()}>
              Add
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => { setAdding(false); setError(null); }}>
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <Button type="button" size="sm" variant="secondary" onClick={() => setAdding(true)} disabled={disabled}>
          + Add profile
        </Button>
      )}
    </div>
  );
}

function SecretInputWithClear({
  label,
  value,
  onChange,
  disabled,
  canClear,
  onClear,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  canClear?: boolean;
  onClear: () => void;
}) {
  return (
    <div className="space-y-1">
      <div className="flex items-end justify-between gap-3">
        <div className="flex-1">
          <Input
            label={label}
            type="password"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder="••••••••"
            disabled={disabled}
          />
        </div>
        {canClear && (
          <button
            type="button"
            onClick={onClear}
            className="text-[11px] text-red-400 hover:text-red-300 transition-colors pb-2.5 whitespace-nowrap"
          >
            Clear stored
          </button>
        )}
      </div>
    </div>
  );
}


function GrafanaIntegrationSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [mode, setMode] = useState<"view" | "edit">("view");
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [webhookSecretCopied, setWebhookSecretCopied] = useState(false);
  const initialRef = useRef<Record<string, string>>({});

  useEffect(() => {
    if (data?.grafana) {
      setForm(data.grafana);
      initialRef.current = { ...data.grafana };
      setDirty({});
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("grafana", values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["integrations"] });
      setDirty({});
      setMode("view");
    },
  });

  const clearSecretMutation = useMutation({
    mutationFn: (key: string) => integrationsAPI.clearSecret("grafana", key),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const testMutation = useMutation({
    mutationFn: () => {
      const typed = form.grafana_api_token ?? "";
      const masked = typed === "" || typed === "••••••••";
      return integrationsAPI.testGrafana({
        base_url: form.grafana_base_url ?? "",
        token: masked ? "" : typed,
      });
    },
    onSuccess: (res) => {
      if (!res.success) {
        const stage = res.stage === "auth" ? "Auth failed" : "Connection failed";
        setTestResult({ success: false, message: `${stage}: ${res.error || "unknown error"}` });
        return;
      }
      const parts: string[] = [];
      if (res.version) parts.push(`Grafana ${res.version}`);
      if (res.user) parts.push(`as ${res.user}${res.name ? ` (${res.name})` : ""}`);
      if (res.org_id !== undefined) parts.push(`org ${res.org_id}`);
      setTestResult({ success: true, message: parts.join(" · ") || "Connected" });
    },
    onError: (err: Error) => setTestResult({ success: false, message: err.message }),
  });

  const set = (key: string, value: string) => {
    setForm((f) => ({ ...f, [key]: value }));
    setDirty((d) => {
      const isDifferent = value !== (initialRef.current[key] ?? "");
      if (d[key] === isDifferent) return d;
      return { ...d, [key]: isDifferent };
    });
  };

  const handleCancel = () => {
    setForm({ ...initialRef.current });
    setDirty({});
    setTestResult(null);
    setMode("view");
  };

  const handleSave = () => {
    const payload: Record<string, string> = {};
    for (const [key, changed] of Object.entries(dirty)) {
      if (changed) payload[key] = form[key] ?? "";
    }
    if (Object.keys(payload).length === 0) {
      setMode("view");
      return;
    }
    mutation.mutate(payload);
  };

  const handleClearSecret = (key: string, label: string) => {
    if (!confirm(`Clear the stored ${label}? This cannot be undone — you'll need to re-enter it.`)) return;
    clearSecretMutation.mutate(key);
    setForm((f) => ({ ...f, [key]: "" }));
  };

  const isView = mode === "view";
  const isEdit = mode === "edit";
  const hasDirty = Object.values(dirty).some(Boolean);
  const enabled = form.grafana_enabled === "true";

  return (
    <Card>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0 flex items-center gap-3 flex-wrap">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">Grafana</h3>
          <Toggle
            checked={enabled}
            onChange={(v) => set("grafana_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel="Enable Grafana integration"
          />
          <p className="text-[11px] text-[var(--text-muted)]">
            Embedded dashboards, live metrics, alert ingestion, agent install.
          </p>
        </div>
        {isView && (
          <Button type="button" size="sm" variant="secondary" onClick={() => setMode("edit")}>
            Edit
          </Button>
        )}
      </div>

      {enabled && (
        <div className="space-y-4">
          {/* Connection */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="Base URL"
              value={form.grafana_base_url ?? ""}
              onChange={(e) => set("grafana_base_url", e.target.value)}
              placeholder="https://grafana.example.org"
              disabled={isView}
            />
            <Input
              label="Datasource UID (Prometheus, for live KPIs)"
              value={form.grafana_datasource_uid ?? ""}
              onChange={(e) => set("grafana_datasource_uid", e.target.value)}
              placeholder="prometheus"
              disabled={isView}
            />
          </div>
          <SecretInputWithClear
            label="API Token"
            value={form.grafana_api_token ?? ""}
            onChange={(v) => set("grafana_api_token", v)}
            disabled={isView}
            canClear={isEdit && initialRef.current.grafana_api_token === "••••••••"}
            onClear={() => handleClearSecret("grafana_api_token", "API Token")}
          />

          {/* Default dashboards */}
          <details className="border-t border-[var(--border-default)] pt-4" open>
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              Default dashboards
            </summary>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
              <Input
                label="Default host dashboard UID"
                value={form.grafana_host_default_dashboard_uid ?? ""}
                onChange={(e) => set("grafana_host_default_dashboard_uid", e.target.value)}
                placeholder="node-exporter-full"
                disabled={isView}
              />
              <Input
                label="Default service dashboard UID"
                value={form.grafana_service_default_dashboard_uid ?? ""}
                onChange={(e) => set("grafana_service_default_dashboard_uid", e.target.value)}
                placeholder="service-overview"
                disabled={isView}
              />
            </div>
            <p className="text-[11px] text-[var(--text-muted)] mt-2">
              Per-host and per-service UIDs override these. Dashboard variable <code className="text-[var(--text-secondary)]">var-host</code> / <code className="text-[var(--text-secondary)]">var-service</code> receives the slug/nickname.
            </p>
          </details>

          {/* Remote write creds (for Agent install) */}
          <details className="border-t border-[var(--border-default)] pt-4">
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              Prometheus remote_write (Grafana Agent)
            </summary>
            <div className="space-y-3 mt-3">
              <Input
                label="Remote write URL"
                value={form.grafana_prom_remote_write_url ?? ""}
                onChange={(e) => set("grafana_prom_remote_write_url", e.target.value)}
                placeholder="https://prometheus.example.org/api/v1/write"
                disabled={isView}
              />
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Input
                  label="Remote write username"
                  value={form.grafana_prom_remote_write_username ?? ""}
                  onChange={(e) => set("grafana_prom_remote_write_username", e.target.value)}
                  placeholder="scrape-user"
                  disabled={isView}
                />
                <SecretInputWithClear
                  label="Remote write password"
                  value={form.grafana_prom_remote_write_password ?? ""}
                  onChange={(v) => set("grafana_prom_remote_write_password", v)}
                  disabled={isView}
                  canClear={isEdit && initialRef.current.grafana_prom_remote_write_password === "••••••••"}
                  onClear={() => handleClearSecret("grafana_prom_remote_write_password", "remote_write password")}
                />
              </div>
              <p className="text-[11px] text-[var(--text-muted)]">
                Used by the Grafana Agent installed on hosts. Leave blank if Prometheus accepts anonymous writes inside your network.
              </p>
            </div>
          </details>

          {/* Webhook secret (for alert ingestion) */}
          <details className="border-t border-[var(--border-default)] pt-4">
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              Alert webhook
            </summary>
            <div className="space-y-3 mt-3">
              <SecretInputWithClear
                label="Webhook HMAC secret"
                value={form.grafana_webhook_secret ?? ""}
                onChange={(v) => set("grafana_webhook_secret", v)}
                disabled={isView}
                canClear={isEdit && initialRef.current.grafana_webhook_secret === "••••••••"}
                onClear={() => handleClearSecret("grafana_webhook_secret", "webhook secret")}
              />
              {isEdit && (() => {
                const currentSecret = form.grafana_webhook_secret ?? "";
                const hasPlaintext = currentSecret !== "" && currentSecret !== "••••••••";
                return (
                  <div className="flex items-center flex-wrap gap-3 -mt-2">
                    <button
                      type="button"
                      onClick={() => {
                        // 32 random bytes → 64 hex chars. More than enough entropy
                        // for HMAC-SHA256 keying, and hex keeps it copy-paste safe.
                        const bytes = new Uint8Array(32);
                        crypto.getRandomValues(bytes);
                        const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
                        set("grafana_webhook_secret", hex);
                        setWebhookSecretCopied(false);
                      }}
                      className="text-[11px] text-[var(--accent)] hover:underline"
                    >
                      Generate random secret
                    </button>
                    <button
                      type="button"
                      disabled={!hasPlaintext}
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(currentSecret);
                          setWebhookSecretCopied(true);
                          setTimeout(() => setWebhookSecretCopied(false), 1800);
                        } catch {
                          // Clipboard API may fail on non-secure contexts (http://). Fall back to prompt.
                          window.prompt("Copy the webhook secret below:", currentSecret);
                        }
                      }}
                      className="text-[11px] text-[var(--text-muted)] hover:text-[var(--text-primary)] disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:text-[var(--text-muted)]"
                      title={hasPlaintext ? "Copy to clipboard" : "Generate a secret first"}
                    >
                      {webhookSecretCopied ? "Copied ✓" : "Copy"}
                    </button>
                    <span className="text-[10px] text-[var(--text-faint)]">
                      Copy it now — after saving, sshcm only shows the masked form.
                    </span>
                  </div>
                );
              })()}
              <p className="text-[11px] text-[var(--text-muted)]">
                Configure a Grafana contact point posting JSON to <code className="text-[var(--text-secondary)]">/api/webhooks/grafana/alerts</code> with header <code className="text-[var(--text-secondary)]">X-Sshcm-Signature: sha256=&lt;hmac&gt;</code>.
              </p>
            </div>
          </details>

          <div className="flex items-center gap-3 flex-wrap">
            <Button
              type="button"
              variant="secondary"
              onClick={() => { setTestResult(null); testMutation.mutate(); }}
              loading={testMutation.isPending}
              disabled={isView}
            >
              Test connection
            </Button>
            {testResult && (
              <span className={`text-xs ${testResult.success ? "text-green-400" : "text-red-400"}`}>
                {testResult.message}
              </span>
            )}
          </div>
        </div>
      )}

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-[11px] text-[var(--text-muted)]">
          {isView ? "Read-only — click Edit to change settings." : hasDirty ? "Unsaved changes." : "No changes."}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button type="button" variant="secondary" onClick={handleCancel}>Cancel</Button>
              <Button type="button" onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                Save changes
              </Button>
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

function LLMSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);

  useEffect(() => {
    if (data?.llm) setForm(data.llm);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("llm", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const testMutation = useMutation({
    mutationFn: () => {
      const typedKey = form.llm_api_key ?? "";
      const isMaskedPlaceholder = typedKey === "" || typedKey === "••••••••";
      return integrationsAPI.testLLM({
        base_url: form.llm_base_url ?? "",
        api_key: isMaskedPlaceholder ? "" : typedKey,
        model: form.llm_model_text ?? "",
      });
    },
    onSuccess: (res) => {
      if (!res.success) {
        const stageLabel = res.stage === "chat" ? "Chat failed" : "Connection failed";
        setTestResult({ success: false, message: `${stageLabel}: ${res.error || "unknown error"}` });
        return;
      }
      // Compose a friendly summary: endpoint → model catalog → chat round-trip.
      const parts: string[] = [];
      parts.push(`Endpoint OK (${res.models_count ?? 0} models)`);
      if (res.model) {
        parts.push(res.model_available
          ? `model "${res.model}" available`
          : `model "${res.model}" NOT in list`);
      }
      if (res.chat_ok) {
        const replyPreview = res.chat_reply ? ` — replied: "${res.chat_reply}"` : "";
        parts.push(`chat OK${replyPreview}`);
      } else if (res.warning) {
        parts.push(res.warning);
      }
      setTestResult({ success: true, message: parts.join(" · ") });
    },
    onError: (err: Error) => setTestResult({ success: false, message: err.message }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));
  const enabled = form.llm_enabled === "true";

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">AI Assistant (LLM)</h3>
        <Toggle
          checked={enabled}
          onChange={(v) => set("llm_enabled", v ? "true" : "false")}
          ariaLabel="Enable LLM"
        />
      </div>

      {enabled && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label="API Base URL"
              value={form.llm_base_url ?? ""}
              onChange={(e) => set("llm_base_url", e.target.value)}
              placeholder="https://api.sobdemanda.mandu.piaui.pro/v1"
            />
            <Input
              label="API Key"
              type="password"
              value={form.llm_api_key ?? ""}
              onChange={(e) => set("llm_api_key", e.target.value)}
              placeholder="••••••••"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label="Text Model"
              value={form.llm_model_text ?? "Qwen/Qwen3-30B-A3B"}
              onChange={(e) => set("llm_model_text", e.target.value)}
              placeholder="Qwen/Qwen3-30B-A3B"
            />
            <Input
              label="Vision Model"
              value={form.llm_model_vision ?? "Qwen/Qwen3-VL-30B-A3B-Thinking"}
              onChange={(e) => set("llm_model_vision", e.target.value)}
              placeholder="Qwen/Qwen3-VL-30B-A3B-Thinking"
            />
            <Input
              label="Max Tokens"
              type="number"
              value={form.llm_max_tokens ?? "2000"}
              onChange={(e) => set("llm_max_tokens", e.target.value)}
              placeholder="2000"
            />
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              Save LLM Settings
            </Button>
            <Button
              variant="secondary"
              onClick={() => { setTestResult(null); testMutation.mutate(); }}
              loading={testMutation.isPending}
            >
              Test connection
            </Button>
            {testResult && (
              <span className={`text-xs ${testResult.success ? "text-green-400" : "text-red-400"}`}>
                {testResult.message}
              </span>
            )}
          </div>
        </div>
      )}
    </Card>
  );
}

function CoolifySection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});
  const [testResult, setTestResult] = useState<{ success: boolean; error?: string } | null>(null);

  useEffect(() => {
    if (data?.coolify) setForm(data.coolify);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("coolify", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));
  const enabled = form.coolify_enabled === "true";

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <div>
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">Coolify</h3>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">Self-hosting platform — manage servers directly from SSHCM.</p>
        </div>
        <Toggle
          checked={enabled}
          onChange={(v) => set("coolify_enabled", v ? "true" : "false")}
          ariaLabel="Enable Coolify"
        />
      </div>

      {enabled && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label="Base URL"
              value={form.coolify_base_url ?? ""}
              onChange={(e) => set("coolify_base_url", e.target.value)}
              placeholder="https://coolify.example.com"
            />
            <Input
              label="API Token"
              type="password"
              value={form.coolify_api_token ?? ""}
              onChange={(e) => set("coolify_api_token", e.target.value)}
              placeholder="••••••••"
            />
            <Input
              label="SSH User"
              value={form.coolify_default_user ?? ""}
              onChange={(e) => set("coolify_default_user", e.target.value)}
              placeholder="root"
            />
          </div>
          <p className="text-[10px] text-[var(--text-faint)]">
            SSH User is used when registering servers in Coolify. Coolify does not accept dots in usernames. Defaults to &quot;root&quot;.
          </p>

          <div className="flex items-center gap-2">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              Save Coolify Settings
            </Button>
            <Button variant="secondary" onClick={() => { setTestResult(null); coolifyAPI.testConnection().then(setTestResult).catch(err => setTestResult({ success: false, error: err instanceof Error ? err.message : "Failed" })); }} loading={false}>
              Test Connection
            </Button>
          </div>
          {testResult && (
            <div className={`rounded-[var(--radius-sm)] px-3 py-2 text-xs ${testResult.success ? "bg-emerald-500/10 border border-emerald-500/25 text-emerald-400" : "bg-red-500/10 border border-red-500/25 text-red-400"}`}>
              {testResult.success ? "Connection successful" : `Connection failed: ${testResult.error}`}
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
