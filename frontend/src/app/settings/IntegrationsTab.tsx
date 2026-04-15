"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useEffect } from "react";
import { integrationsAPI, coolifyAPI } from "@/lib/api";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";

export default function IntegrationsTab() {
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const activeProvider = data?.general?.auth_active_provider ?? "local";

  return (
    <div className="space-y-6">
      <GeneralAuthSection />
      {activeProvider === "ldap" && <LDAPSection />}
      {activeProvider === "keycloak" && <KeycloakSection />}
      {activeProvider === "gitlab" && <GitLabAuthSection />}
      <GitLabCodeSection />
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

function GitLabAuthSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});

  useEffect(() => {
    if (data?.gitlab) setForm(data.gitlab);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("gitlab", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));

  const callbackURL = typeof window !== "undefined"
    ? `${window.location.origin}/api/auth/oauth/gitlab/callback`
    : "/api/auth/oauth/gitlab/callback";

  return (
    <Card>
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4">GitLab SSO Configuration</h3>
      <div className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Input
            label="GitLab Base URL"
            value={form.auth_gitlab_base_url ?? "https://gitlab.com"}
            onChange={(e) => set("auth_gitlab_base_url", e.target.value)}
            placeholder="https://gitlab.com"
          />
          <Input
            label="Application ID (Client ID)"
            value={form.auth_gitlab_client_id ?? ""}
            onChange={(e) => set("auth_gitlab_client_id", e.target.value)}
            placeholder="your-app-id"
          />
        </div>

        <Input
          label="Client Secret"
          type="password"
          value={form.auth_gitlab_client_secret ?? ""}
          onChange={(e) => set("auth_gitlab_client_secret", e.target.value)}
          placeholder="••••••••"
        />

        <div>
          <label className="block text-xs text-[var(--text-muted)] mb-1">Redirect URI (add this to GitLab application)</label>
          <code className="block bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-xs text-[var(--text-secondary)] font-mono break-all">
            {callbackURL}
          </code>
        </div>

        <div className="flex justify-start">
          <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
            Save GitLab Auth Settings
          </Button>
        </div>
      </div>
    </Card>
  );
}

function GitLabCodeSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});

  useEffect(() => {
    if (data?.gitlab) setForm(data.gitlab);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("gitlab", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));
  const enabled = form.gitlab_integration_enabled === "true";

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <div>
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">GitLab Code Management</h3>
          <p className="text-[11px] text-[var(--text-muted)] mt-0.5">Track commits and issues from linked GitLab projects. Works independently of the auth provider.</p>
        </div>
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => set("gitlab_integration_enabled", e.target.checked ? "true" : "false")}
            className="sr-only peer"
          />
          <div className="w-9 h-5 bg-[var(--bg-overlay)] peer-focus:outline-none rounded-full peer peer-checked:bg-[var(--accent)] transition-all after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full" />
        </label>
      </div>

      {enabled && (
        <div className="space-y-4">
          <Input
            label="GitLab Base URL"
            value={form.auth_gitlab_base_url ?? "https://gitlab.com"}
            onChange={(e) => set("auth_gitlab_base_url", e.target.value)}
            placeholder="https://gitlab.com"
          />
          <p className="text-xs text-[var(--text-muted)]">
            Users can connect their GitLab accounts via personal access tokens in their profile. Projects can be linked to GitLab repositories for commit and issue tracking.
          </p>
          <div className="flex justify-start">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              Save
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
}

function LLMSection() {
  const queryClient = useQueryClient();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const [form, setForm] = useState<Record<string, string>>({});

  useEffect(() => {
    if (data?.llm) setForm(data.llm);
  }, [data]);

  const mutation = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update("llm", values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["integrations"] }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));
  const enabled = form.llm_enabled === "true";

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">AI Assistant (LLM)</h3>
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => set("llm_enabled", e.target.checked ? "true" : "false")}
            className="sr-only peer"
          />
          <div className="w-9 h-5 bg-[var(--bg-overlay)] peer-focus:outline-none rounded-full peer peer-checked:bg-[var(--accent)] transition-all after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full" />
        </label>
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

          <div className="flex justify-start">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              Save LLM Settings
            </Button>
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
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => set("coolify_enabled", e.target.checked ? "true" : "false")}
            className="sr-only peer"
          />
          <div className="w-9 h-5 bg-[var(--bg-overlay)] peer-focus:outline-none rounded-full peer peer-checked:bg-[var(--accent)] transition-all after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full" />
        </label>
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
