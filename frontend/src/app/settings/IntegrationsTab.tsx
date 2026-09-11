"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useEffect, useRef } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI, coolifyAPI, glpiAPI, outlineAPI, type GlpiTokenProfile } from "@/lib/api";
import CollectionMultiSelect from "@/components/wiki/CollectionMultiSelect";
import DropdownCatalogueSection from "@/components/glpi/DropdownCatalogueSection";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Toggle from "@/components/ui/Toggle";
import NativeSelect from "@/components/ui/NativeSelect";

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
  { value: "local", labelKey: "settings.integrations.auth.providerLocalLabel", descKey: "settings.integrations.auth.providerLocalDesc", color: "#06b6d4" },
  { value: "ldap", labelKey: "settings.integrations.auth.providerLdapLabel", descKey: "settings.integrations.auth.providerLdapDesc", color: "#3b82f6" },
  { value: "keycloak", labelKey: "settings.integrations.auth.providerKeycloakLabel", descKey: "settings.integrations.auth.providerKeycloakDesc", color: "#22c55e" },
  { value: "gitlab", labelKey: "settings.integrations.auth.providerGitlabLabel", descKey: "settings.integrations.auth.providerGitlabDesc", color: "#e24329" },
];

function GeneralAuthSection() {
  const { t } = useLocale();
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
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-1">{t("settings.integrations.auth.title")}</h3>
      <p className="text-xs text-[var(--text-muted)] mb-4">{t("settings.integrations.auth.hint")}</p>

      {/* Provider radio selector */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-2 mb-5">
        {AUTH_PROVIDERS.map((p) => (
          <label
            key={p.value}
            className={`flex items-start gap-3 p-3 rounded-[var(--radius-md)] border cursor-pointer transition ${
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
                <span className="text-sm font-medium text-[var(--text-primary)]">{t(p.labelKey)}</span>
              </div>
              <p className="text-xs text-[var(--text-muted)] mt-0.5">{t(p.descKey)}</p>
            </div>
          </label>
        ))}
      </div>

      {/* General auth settings (only shown when external provider is active) */}
      {activeProvider !== "local" && (
        <div className="border-t border-[var(--border-subtle)] pt-4 mt-4">
          <p className="text-xs font-semibold text-[var(--text-muted)] mb-3">{t("settings.integrations.auth.externalSettingsTitle")}</p>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <NativeSelect label={t("settings.integrations.auth.autoProvisionUsers")}
                value={form.auth_auto_provision ?? "true"}
                onChange={(e) => set("auth_auto_provision", e.target.value)}
              >
                <option value="true">{t("settings.integrations.enabled")}</option>
                <option value="false">{t("settings.integrations.disabled")}</option>
              </NativeSelect>
            </div>
            <div>
              <NativeSelect label={t("settings.integrations.auth.defaultRole")}
                value={form.auth_default_role ?? "viewer"}
                onChange={(e) => set("auth_default_role", e.target.value)}
              >
                <option value="viewer">{t("settings.integrations.auth.roleViewer")}</option>
                <option value="editor">{t("settings.integrations.auth.roleEditor")}</option>
                <option value="admin">{t("settings.integrations.auth.roleAdmin")}</option>
              </NativeSelect>
            </div>
            <div>
              <NativeSelect label={t("settings.integrations.auth.syncRoles")}
                value={form.auth_role_sync_enabled ?? "false"}
                onChange={(e) => set("auth_role_sync_enabled", e.target.value)}
              >
                <option value="false">{t("settings.integrations.disabled")}</option>
                <option value="true">{t("settings.integrations.enabled")}</option>
              </NativeSelect>
            </div>
          </div>
        </div>
      )}

      <div className="mt-4 flex justify-end">
        <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
          {t("common.save")}
        </Button>
      </div>
    </Card>
  );
}

function LDAPSection() {
  const { t } = useLocale();
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
    onError: () => setTestResult({ success: false, error: t("settings.integrations.ldap.requestFailed") }),
  });

  const set = (key: string, value: string) => setForm((f) => ({ ...f, [key]: value }));

  return (
    <Card>
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4">{t("settings.integrations.ldap.title")}</h3>
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label={t("settings.integrations.ldap.host")}
              value={form.auth_ldap_host ?? ""}
              onChange={(e) => set("auth_ldap_host", e.target.value)}
              placeholder="ldaps://ldap.example.com"
            />
            <Input
              label={t("settings.integrations.ldap.port")}
              value={form.auth_ldap_port ?? "636"}
              onChange={(e) => set("auth_ldap_port", e.target.value)}
              placeholder="636"
            />
            <div className="grid grid-cols-2 gap-2">
              <div>
                <NativeSelect label="TLS"
                  value={form.auth_ldap_use_tls ?? "true"}
                  onChange={(e) => set("auth_ldap_use_tls", e.target.value)}
                >
                  <option value="true">{t("common.yes")}</option>
                  <option value="false">{t("common.no")}</option>
                </NativeSelect>
              </div>
              <div>
                <NativeSelect label={t("settings.integrations.ldap.skipVerify")}
                  value={form.auth_ldap_skip_verify ?? "false"}
                  onChange={(e) => set("auth_ldap_skip_verify", e.target.value)}
                >
                  <option value="false">{t("common.no")}</option>
                  <option value="true">{t("common.yes")}</option>
                </NativeSelect>
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
            label={t("settings.integrations.ldap.bindPassword")}
            type="password"
            value={form.auth_ldap_bind_password ?? ""}
            onChange={(e) => set("auth_ldap_bind_password", e.target.value)}
            placeholder="••••••••"
          />

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label={t("settings.integrations.ldap.userFilter")}
              value={form.auth_ldap_user_filter ?? "(mail=%s)"}
              onChange={(e) => set("auth_ldap_user_filter", e.target.value)}
              placeholder="(mail=%s)"
            />
            <div>
              <NativeSelect label={t("settings.integrations.ldap.fallbackToLocal")}
                value={form.auth_ldap_fallback_to_local ?? "true"}
                onChange={(e) => set("auth_ldap_fallback_to_local", e.target.value)}
              >
                <option value="true">{t("settings.integrations.enabled")}</option>
                <option value="false">{t("settings.integrations.disabled")}</option>
              </NativeSelect>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label={t("settings.integrations.ldap.usernameAttr")}
              value={form.auth_ldap_username_attr ?? "uid"}
              onChange={(e) => set("auth_ldap_username_attr", e.target.value)}
              placeholder="uid"
            />
            <Input
              label={t("settings.integrations.ldap.displayNameAttr")}
              value={form.auth_ldap_display_name_attr ?? "cn"}
              onChange={(e) => set("auth_ldap_display_name_attr", e.target.value)}
              placeholder="cn"
            />
            <Input
              label={t("settings.integrations.ldap.emailAttr")}
              value={form.auth_ldap_email_attr ?? "mail"}
              onChange={(e) => set("auth_ldap_email_attr", e.target.value)}
              placeholder="mail"
            />
          </div>

          <div className="flex items-center gap-3">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              {t("settings.integrations.ldap.saveButton")}
            </Button>
            <Button
              variant="secondary"
              onClick={() => { setTestResult(null); testMutation.mutate(); }}
              loading={testMutation.isPending}
            >
              {t("settings.integrations.testConnection")}
            </Button>
            {testResult && (
              <span className={`text-sm ${testResult.success ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
                {testResult.success ? t("settings.integrations.connectionSuccessful") : testResult.error}
              </span>
            )}
          </div>
        </div>
    </Card>
  );
}

function KeycloakSection() {
  const { t } = useLocale();
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
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4">{t("settings.integrations.keycloak.title")}</h3>
        <div className="space-y-4">
          {/* Environment presets */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1.5">{t("settings.integrations.keycloak.environmentPresets")}</label>
            <div className="flex gap-2">
              {Object.keys(presets).map((env) => (
                <button
                  key={env}
                  type="button"
                  onClick={() => applyPreset(env)}
                  className="px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-overlay)] border border-[var(--border-default)] transition capitalize"
                >
                  {env}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label={t("settings.integrations.baseUrl")}
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
              label={t("settings.integrations.keycloak.clientId")}
              value={form.auth_keycloak_client_id ?? ""}
              onChange={(e) => set("auth_keycloak_client_id", e.target.value)}
              placeholder="my-app-client"
            />
            <Input
              label={t("settings.integrations.clientSecret")}
              type="password"
              value={form.auth_keycloak_client_secret ?? ""}
              onChange={(e) => set("auth_keycloak_client_secret", e.target.value)}
              placeholder="••••••••"
            />
          </div>

          {/* Callback URL (read-only) */}
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">{t("settings.integrations.keycloak.redirectUriHint")}</label>
            <div className="flex items-center gap-2">
              <code className="flex-1 bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-xs text-[var(--text-secondary)] font-mono break-all">
                {callbackURL}
              </code>
            </div>
          </div>

          <div className="flex justify-start">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              {t("settings.integrations.keycloak.saveButton")}
            </Button>
          </div>
        </div>
    </Card>
  );
}

function GitLabIntegrationSection({ ssoActive }: { ssoActive: boolean }) {
  const { t } = useLocale();
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
          ? t("settings.integrations.gitlab.connectedAs", { username: res.username ?? t("settings.integrations.gitlab.unknownUser") })
          : res.error || t("settings.integrations.connectionFailed"),
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
    if (!confirm(t("settings.integrations.confirmClearSecret", { label }))) return;
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
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t("settings.integrations.gitlab.title")}</h3>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">
            {t("settings.integrations.gitlab.hint")}
          </p>
        </div>
        {isView && (
          <Button size="sm" variant="secondary" onClick={() => setMode("edit")}>
            {t("common.edit")}
          </Button>
        )}
      </div>

      {/* Shared credentials */}
      <div className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Input
            label={t("settings.integrations.gitlab.baseUrl")}
            value={form.auth_gitlab_base_url ?? "https://gitlab.com"}
            onChange={(e) => set("auth_gitlab_base_url", e.target.value)}
            placeholder="https://gitlab.com"
            disabled={isView}
          />
          <Input
            label={t("settings.integrations.gitlab.applicationId")}
            value={form.auth_gitlab_client_id ?? ""}
            onChange={(e) => set("auth_gitlab_client_id", e.target.value)}
            placeholder="your-app-id"
            disabled={isView}
          />
        </div>
        <SecretInputWithClear
          label={t("settings.integrations.clientSecret")}
          value={form.auth_gitlab_client_secret ?? ""}
          onChange={(v) => set("auth_gitlab_client_secret", v)}
          disabled={isView}
          canClear={isEdit && (initialRef.current.auth_gitlab_client_secret === "••••••••")}
          onClear={() => handleClearSecret("auth_gitlab_client_secret", t("settings.integrations.clientSecret"))}
        />
      </div>

      {/* SSO subsection */}
      <details className="mt-5 border-t border-[var(--border-default)] pt-4" open={ssoActive || ssoEnabled}>
        <summary className="cursor-pointer list-none flex items-center justify-between gap-3">
          <div className="min-w-0">
            <span className="text-sm font-semibold text-[var(--text-primary)]">{t("settings.integrations.gitlab.ssoSectionTitle")}</span>
            <span className="ml-2 text-xs text-[var(--text-muted)]">
              {ssoEnabled
                ? (ssoActive ? t("settings.integrations.gitlab.ssoActiveHint") : t("settings.integrations.gitlab.ssoEnabledHint"))
                : t("settings.integrations.disabled")}
            </span>
          </div>
          <Toggle
            checked={ssoEnabled}
            onChange={(v) => set("auth_gitlab_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel={t("settings.integrations.gitlab.ariaEnableSso")}
          />
        </summary>
        <div className="mt-4 space-y-3">
          <label className="block text-xs text-[var(--text-muted)] mb-1">{t("settings.integrations.gitlab.redirectUriHint")}</label>
          <code className="block bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-xs text-[var(--text-secondary)] font-mono break-all">
            {callbackURL}
          </code>
          <p className="text-xs text-[var(--text-muted)]">
            {t("settings.integrations.gitlab.scopesHint")}
          </p>
        </div>
      </details>

      {/* Code Management subsection */}
      <details className="mt-4 border-t border-[var(--border-default)] pt-4" open={codeEnabled}>
        <summary className="cursor-pointer list-none flex items-center justify-between gap-3">
          <div className="min-w-0">
            <span className="text-sm font-semibold text-[var(--text-primary)]">{t("settings.integrations.gitlab.codeManagementTitle")}</span>
            <span className="ml-2 text-xs text-[var(--text-muted)]">
              {t("settings.integrations.gitlab.codeManagementHint")}
            </span>
          </div>
          <Toggle
            checked={codeEnabled}
            onChange={(v) => set("gitlab_integration_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel={t("settings.integrations.gitlab.ariaEnableCodeManagement")}
          />
        </summary>
        {codeEnabled && (
          <div className="mt-4 space-y-4">
            <SecretInputWithClear
              label={t("settings.integrations.gitlab.serviceAccessToken")}
              value={form.gitlab_code_service_token ?? ""}
              onChange={(v) => set("gitlab_code_service_token", v)}
              disabled={isView}
              canClear={isEdit && (initialRef.current.gitlab_code_service_token === "••••••••")}
              onClear={() => handleClearSecret("gitlab_code_service_token", t("settings.integrations.gitlab.serviceAccessToken"))}
            />
            <p className="text-xs text-[var(--text-muted)] -mt-2">
              {t("settings.integrations.gitlab.serviceTokenHint")}
            </p>
            <Input
              label={t("settings.integrations.gitlab.defaultBranch")}
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
                {t("settings.integrations.testConnection")}
              </Button>
              {testResult && (
                <span className={`text-xs ${testResult.success ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
                  {testResult.message}
                </span>
              )}
            </div>
          </div>
        )}
      </details>

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-xs text-[var(--text-muted)]">
          {isView ? t("settings.integrations.readOnlyHint") : hasDirty ? t("settings.integrations.unsavedChanges") : t("settings.integrations.noChanges")}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button variant="secondary" onClick={handleCancel}>{t("common.cancel")}</Button>
              <Button onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                {t("settings.integrations.saveChanges")}
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
  const { t } = useLocale();
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
        setTestResult({ success: false, message: t("settings.integrations.stageFailedDetail", { stage: t("settings.integrations.connectionFailed"), error: res.error || t("settings.integrations.unknownError") }) });
        return;
      }
      const parts: string[] = [];
      if (res.workspace) parts.push(res.workspace);
      if (res.user) parts.push(t("settings.integrations.asUser", { user: res.user }));
      setTestResult({ success: true, message: parts.join(" · ") || t("settings.integrations.connected") });
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
    if (!confirm(t("settings.integrations.confirmClearSecret", { label }))) return;
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
            ariaLabel={t("settings.integrations.outline.ariaEnableIntegration")}
          />
          <p className="text-xs text-[var(--text-muted)]">
            {t("settings.integrations.outline.wikiTabHint")}
          </p>
        </div>
        {isView && (
          <Button type="button" size="sm" variant="secondary" onClick={() => setMode("edit")}>
            {t("common.edit")}
          </Button>
        )}
      </div>

      {enabled && (
        <div className="space-y-4">
          <Input
            label={t("settings.integrations.baseUrl")}
            value={form.outline_base_url ?? ""}
            onChange={(e) => set("outline_base_url", e.target.value)}
            placeholder="https://wiki.example.org"
            disabled={isView}
          />
          <SecretInputWithClear
            label={t("settings.integrations.apiToken")}
            value={form.outline_api_token ?? ""}
            onChange={(v) => set("outline_api_token", v)}
            disabled={isView}
            canClear={isEdit && initialRef.current.outline_api_token === "••••••••"}
            onClear={() => handleClearSecret("outline_api_token", t("settings.integrations.apiToken"))}
          />
          <p className="text-xs text-[var(--text-muted)] -mt-2">
            {t("settings.integrations.outline.tokenVisibilityHint")}
          </p>

          {collectionsError || fallbackToText ? (
            <>
              <Input
                label={t("settings.integrations.outline.commonCollectionIds")}
                value={form.outline_common_collection_id ?? ""}
                onChange={(e) => set("outline_common_collection_id", e.target.value)}
                placeholder="uuid-1, uuid-2, uuid-3"
                disabled={isView}
              />
              <p className="text-xs text-[var(--text-muted)] -mt-2">
                {collectionsError
                  ? t("settings.integrations.outline.collectionsHintError")
                  : t("settings.integrations.outline.collectionsHintManual")}{" "}
                {!collectionsError && (
                  <button
                    type="button"
                    className="text-[var(--accent)] hover:underline"
                    onClick={() => setFallbackToText(false)}
                  >
                    {t("settings.integrations.outline.switchToPicker")}
                  </button>
                )}
              </p>
            </>
          ) : (
            <>
              <CollectionMultiSelect
                label={t("settings.integrations.outline.collectionsLabel")}
                collections={collectionsEnv?.collections ?? []}
                value={parseCollectionCSV(form.outline_common_collection_id ?? "")}
                onChange={(ids) => set("outline_common_collection_id", ids.join(", "))}
                loading={collectionsLoading}
                disabled={isView}
                emptyHint={
                  outlineReady
                    ? t("settings.integrations.outline.emptyHintReady")
                    : t("settings.integrations.outline.emptyHintNotReady")
                }
              />
              <p className="text-xs text-[var(--text-muted)] -mt-2">
                {t("settings.integrations.outline.feedsSidebarLong")}{" "}
                {isEdit && (
                  <button
                    type="button"
                    className="text-[var(--accent)] hover:underline"
                    onClick={() => setFallbackToText(true)}
                  >
                    {t("settings.integrations.outline.pasteUuidsInstead")}
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
              {t("settings.integrations.testConnection")}
            </Button>
            {testResult && (
              <span className={`text-xs ${testResult.success ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
                {testResult.message}
              </span>
            )}
          </div>
        </div>
      )}

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-xs text-[var(--text-muted)]">
          {isView ? t("settings.integrations.readOnlyHint") : hasDirty ? t("settings.integrations.unsavedChanges") : t("settings.integrations.noChanges")}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button type="button" variant="secondary" onClick={handleCancel}>{t("common.cancel")}</Button>
              <Button type="button" onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                {t("settings.integrations.saveChanges")}
              </Button>
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

function GLPIIntegrationSection() {
  const { t } = useLocale();
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
    if (!confirm(t("settings.integrations.glpi.confirmClearSecret", { label }))) return;
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
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t("settings.integrations.glpi.title")}</h3>
          <Toggle
            checked={enabled}
            onChange={(v) => set("glpi_enabled", v ? "true" : "false")}
            disabled={isView}
            ariaLabel={t("settings.integrations.glpi.ariaEnableIntegration")}
          />
          <p className="text-xs text-[var(--text-muted)]">
            {t("settings.integrations.glpi.hint")}
          </p>
        </div>
        {isView && (
          <Button type="button" size="sm" variant="secondary" onClick={() => setMode("edit")}>
            {t("common.edit")}
          </Button>
        )}
      </div>

      {enabled && (
        <div className="space-y-4">
          <Input
            label={t("settings.integrations.baseUrl")}
            value={form.glpi_base_url ?? ""}
            onChange={(e) => set("glpi_base_url", e.target.value)}
            placeholder="https://glpi.example.org"
            disabled={isView}
          />
          <SecretInputWithClear
            label={t("settings.integrations.glpi.appToken")}
            value={form.glpi_app_token ?? ""}
            onChange={(v) => set("glpi_app_token", v)}
            disabled={isView}
            canClear={isEdit && initialRef.current.glpi_app_token === "••••••••"}
            onClear={() => handleClearSecret("glpi_app_token", "App-Token")}
          />
          <p className="text-xs text-[var(--text-muted)] -mt-2">
            {t("settings.integrations.glpi.appTokenHint")}
          </p>
          <Input
            label={t("settings.integrations.glpi.defaultEntityIdFallback")}
            type="number"
            value={form.glpi_default_entity_id ?? "0"}
            onChange={(e) => set("glpi_default_entity_id", e.target.value)}
            disabled={isView}
          />

          <DropdownCatalogueSection />

          <details className="border-t border-[var(--border-default)] pt-4" open>
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              {t("settings.integrations.glpi.tokenProfilesHeading", { count: String(profiles?.length ?? 0) })}
            </summary>
            <div className="mt-3">
              <GlpiProfileList profiles={profiles ?? []} disabled={isView} />
            </div>
          </details>
        </div>
      )}

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-xs text-[var(--text-muted)]">
          {isView ? t("settings.integrations.glpi.readOnlyHint") : hasDirty ? t("settings.integrations.unsavedChanges") : t("settings.integrations.noChanges")}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button type="button" variant="secondary" onClick={handleCancel}>{t("common.cancel")}</Button>
              <Button type="button" onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                {t("settings.integrations.saveChanges")}
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
  const { t } = useLocale();
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
            ? t("settings.integrations.glpi.connectedProfiles", { profiles: res.profiles?.slice(0, 3).join(", ") || t("settings.integrations.glpi.noProfilesFallback") })
            : (res.error || t("settings.integrations.testFailed")),
        },
      }));
    },
  });

  return (
    <div className="space-y-3">
      {profiles.length === 0 && !adding && (
        <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.glpi.emptyProfiles")}</p>
      )}

      {profiles.map((p) => (
        <div key={p.id} className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] px-3 py-2">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <div className="min-w-0">
              <p className="text-sm font-medium text-[var(--text-primary)]">{p.name}</p>
              {p.description && <p className="text-xs text-[var(--text-muted)]">{p.description}</p>}
              <p className="text-2xs text-[var(--text-faint)] mt-0.5">
                {t("settings.integrations.glpi.tokenStatusLine", {
                  status: p.has_token ? t("settings.integrations.glpi.tokenStored") : t("settings.integrations.glpi.noTokenStored"),
                  id: String(p.default_entity_id),
                })}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                disabled={disabled || testMutation.isPending}
                onClick={() => testMutation.mutate(p.id)}
                className="text-xs text-[var(--accent)] hover:underline disabled:opacity-40"
              >
                {t("settings.integrations.glpi.testButton")}
              </button>
              <button
                type="button"
                disabled={disabled}
                onClick={() => {
                  if (!confirm(t("settings.integrations.glpi.confirmDeleteProfile", { name: p.name }))) return;
                  deleteMutation.mutate(p.id);
                }}
                className="text-xs text-[var(--danger)] hover:text-[var(--danger)] disabled:opacity-40"
              >
                {t("common.delete")}
              </button>
            </div>
          </div>
          {testResults[p.id] && (
            <p className={`text-xs mt-1 ${testResults[p.id].ok ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
              {testResults[p.id].message}
            </p>
          )}
        </div>
      ))}

      {adding ? (
        <div className="rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] p-3 space-y-3">
          <Input label={t("common.name")} value={newName} onChange={(e) => setNewName(e.target.value)} placeholder={t("settings.integrations.glpi.profileNamePlaceholder")} />
          <Input label={t("settings.integrations.glpi.profileDescLabel")} value={newDesc} onChange={(e) => setNewDesc(e.target.value)} />
          <Input label={t("settings.integrations.glpi.userTokenLabel")} type="password" value={newToken} onChange={(e) => setNewToken(e.target.value)} placeholder={t("settings.integrations.glpi.userTokenPlaceholder")} />
          <Input label={t("settings.integrations.glpi.entityIdLabel")} type="number" value={newEntity} onChange={(e) => setNewEntity(e.target.value)} />
          {error && <p className="text-xs text-[var(--danger)]">{error}</p>}
          <div className="flex gap-2">
            <Button type="button" size="sm" onClick={() => createMutation.mutate()} loading={createMutation.isPending} disabled={!newName.trim() || !newToken.trim()}>
              {t("common.add")}
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => { setAdding(false); setError(null); }}>
              {t("common.cancel")}
            </Button>
          </div>
        </div>
      ) : (
        <Button type="button" size="sm" variant="secondary" onClick={() => setAdding(true)} disabled={disabled}>
          {t("settings.integrations.glpi.addProfileButton")}
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
  const { t } = useLocale();
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
            className="text-xs text-[var(--danger)] hover:text-[var(--danger)] transition-colors pb-2.5 whitespace-nowrap"
          >
            {t("settings.integrations.clearStored")}
          </button>
        )}
      </div>
    </div>
  );
}


function GrafanaIntegrationSection() {
  const { t } = useLocale();
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
        const stage = res.stage === "auth" ? t("settings.integrations.authFailed") : t("settings.integrations.connectionFailed");
        setTestResult({ success: false, message: t("settings.integrations.stageFailedDetail", { stage, error: res.error || t("settings.integrations.unknownError") }) });
        return;
      }
      const parts: string[] = [];
      if (res.version) parts.push(`Grafana ${res.version}`);
      if (res.user) parts.push(`${t("settings.integrations.asUser", { user: res.user })}${res.name ? ` (${res.name})` : ""}`);
      if (res.org_id !== undefined) parts.push(t("settings.integrations.grafana.orgLabel", { id: String(res.org_id) }));
      setTestResult({ success: true, message: parts.join(" · ") || t("settings.integrations.connected") });
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
    if (!confirm(t("settings.integrations.confirmClearSecret", { label }))) return;
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
            ariaLabel={t("settings.integrations.grafana.ariaEnableIntegration")}
          />
          <p className="text-xs text-[var(--text-muted)]">
            {t("settings.integrations.grafana.hint")}
          </p>
        </div>
        {isView && (
          <Button type="button" size="sm" variant="secondary" onClick={() => setMode("edit")}>
            {t("common.edit")}
          </Button>
        )}
      </div>

      {enabled && (
        <div className="space-y-4">
          {/* Connection */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label={t("settings.integrations.baseUrl")}
              value={form.grafana_base_url ?? ""}
              onChange={(e) => set("grafana_base_url", e.target.value)}
              placeholder="https://grafana.example.org"
              disabled={isView}
            />
            <Input
              label={t("settings.integrations.grafana.datasourceUid")}
              value={form.grafana_datasource_uid ?? ""}
              onChange={(e) => set("grafana_datasource_uid", e.target.value)}
              placeholder="prometheus"
              disabled={isView}
            />
          </div>
          <SecretInputWithClear
            label={t("settings.integrations.apiToken")}
            value={form.grafana_api_token ?? ""}
            onChange={(v) => set("grafana_api_token", v)}
            disabled={isView}
            canClear={isEdit && initialRef.current.grafana_api_token === "••••••••"}
            onClear={() => handleClearSecret("grafana_api_token", t("settings.integrations.apiToken"))}
          />

          {/* Default dashboards */}
          <details className="border-t border-[var(--border-default)] pt-4" open>
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              {t("settings.integrations.grafana.defaultDashboardsHeading")}
            </summary>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
              <Input
                label={t("settings.integrations.grafana.defaultHostDashboardUid")}
                value={form.grafana_host_default_dashboard_uid ?? ""}
                onChange={(e) => set("grafana_host_default_dashboard_uid", e.target.value)}
                placeholder="node-exporter-full"
                disabled={isView}
              />
              <Input
                label={t("settings.integrations.grafana.defaultServiceDashboardUid")}
                value={form.grafana_service_default_dashboard_uid ?? ""}
                onChange={(e) => set("grafana_service_default_dashboard_uid", e.target.value)}
                placeholder="service-overview"
                disabled={isView}
              />
            </div>
            <p className="text-xs text-[var(--text-muted)] mt-2">
              {t("settings.integrations.grafana.dashboardVarHint")}
            </p>
          </details>

          {/* Remote write creds (for Agent install) */}
          <details className="border-t border-[var(--border-default)] pt-4">
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              Prometheus remote_write (Grafana Agent)
            </summary>
            <div className="space-y-3 mt-3">
              <Input
                label={t("settings.integrations.grafana.remoteWriteUrl")}
                value={form.grafana_prom_remote_write_url ?? ""}
                onChange={(e) => set("grafana_prom_remote_write_url", e.target.value)}
                placeholder="https://prometheus.example.org/api/v1/write"
                disabled={isView}
              />
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Input
                  label={t("settings.integrations.grafana.remoteWriteUsername")}
                  value={form.grafana_prom_remote_write_username ?? ""}
                  onChange={(e) => set("grafana_prom_remote_write_username", e.target.value)}
                  placeholder="scrape-user"
                  disabled={isView}
                />
                <SecretInputWithClear
                  label={t("settings.integrations.grafana.remoteWritePassword")}
                  value={form.grafana_prom_remote_write_password ?? ""}
                  onChange={(v) => set("grafana_prom_remote_write_password", v)}
                  disabled={isView}
                  canClear={isEdit && initialRef.current.grafana_prom_remote_write_password === "••••••••"}
                  onClear={() => handleClearSecret("grafana_prom_remote_write_password", t("settings.integrations.grafana.remoteWritePasswordConfirmLabel"))}
                />
              </div>
              <p className="text-xs text-[var(--text-muted)]">
                {t("settings.integrations.grafana.remoteWriteHint")}
              </p>
            </div>
          </details>

          {/* Webhook secret (for alert ingestion) */}
          <details className="border-t border-[var(--border-default)] pt-4">
            <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)]">
              {t("settings.integrations.grafana.alertWebhookHeading")}
            </summary>
            <div className="space-y-3 mt-3">
              <SecretInputWithClear
                label={t("settings.integrations.grafana.webhookHmacSecret")}
                value={form.grafana_webhook_secret ?? ""}
                onChange={(v) => set("grafana_webhook_secret", v)}
                disabled={isView}
                canClear={isEdit && initialRef.current.grafana_webhook_secret === "••••••••"}
                onClear={() => handleClearSecret("grafana_webhook_secret", t("settings.integrations.grafana.webhookSecretConfirmLabel"))}
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
                      className="text-xs text-[var(--accent)] hover:underline"
                    >
                      {t("settings.integrations.grafana.generateSecretButton")}
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
                          window.prompt(t("settings.integrations.grafana.webhookPromptCopy"), currentSecret);
                        }
                      }}
                      className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:text-[var(--text-muted)]"
                      title={hasPlaintext ? t("settings.integrations.grafana.copyToClipboardTitle") : t("settings.integrations.grafana.generateSecretFirstTitle")}
                    >
                      {webhookSecretCopied ? t("settings.integrations.grafana.copiedCheck") : t("common.copy")}
                    </button>
                    <span className="text-2xs text-[var(--text-faint)]">
                      {t("settings.integrations.grafana.webhookCopyHint")}
                    </span>
                  </div>
                );
              })()}
              <p className="text-xs text-[var(--text-muted)]">
                {t("settings.integrations.grafana.webhookConfigHint")}
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
              {t("settings.integrations.testConnection")}
            </Button>
            {testResult && (
              <span className={`text-xs ${testResult.success ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
                {testResult.message}
              </span>
            )}
          </div>
        </div>
      )}

      <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border-default)] pt-4">
        <span className="text-xs text-[var(--text-muted)]">
          {isView ? t("settings.integrations.readOnlyHint") : hasDirty ? t("settings.integrations.unsavedChanges") : t("settings.integrations.noChanges")}
        </span>
        <div className="flex gap-2">
          {isEdit && (
            <>
              <Button type="button" variant="secondary" onClick={handleCancel}>{t("common.cancel")}</Button>
              <Button type="button" onClick={handleSave} loading={mutation.isPending} disabled={!hasDirty}>
                {t("settings.integrations.saveChanges")}
              </Button>
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

function LLMSection() {
  const { t } = useLocale();
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
        const stageLabel = res.stage === "chat" ? t("settings.integrations.chatFailed") : t("settings.integrations.connectionFailed");
        setTestResult({ success: false, message: t("settings.integrations.stageFailedDetail", { stage: stageLabel, error: res.error || t("settings.integrations.unknownError") }) });
        return;
      }
      // Compose a friendly summary: endpoint → model catalog → chat round-trip.
      const parts: string[] = [];
      parts.push(t("settings.integrations.llm.endpointOk", { count: String(res.models_count ?? 0) }));
      if (res.model) {
        parts.push(res.model_available
          ? t("settings.integrations.llm.modelAvailable", { model: res.model })
          : t("settings.integrations.llm.modelNotInList", { model: res.model }));
      }
      if (res.chat_ok) {
        parts.push(res.chat_reply
          ? t("settings.integrations.llm.chatOkWithReply", { reply: res.chat_reply })
          : t("settings.integrations.llm.chatOk"));
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
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t("settings.integrations.llm.title")}</h3>
        <Toggle
          checked={enabled}
          onChange={(v) => set("llm_enabled", v ? "true" : "false")}
          ariaLabel={t("settings.integrations.llm.ariaEnableLlm")}
        />
      </div>

      {enabled && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              label={t("settings.integrations.llm.apiBaseUrl")}
              value={form.llm_base_url ?? ""}
              onChange={(e) => set("llm_base_url", e.target.value)}
              placeholder="https://api.sobdemanda.mandu.piaui.pro/v1"
            />
            <Input
              label={t("settings.integrations.llm.apiKey")}
              type="password"
              value={form.llm_api_key ?? ""}
              onChange={(e) => set("llm_api_key", e.target.value)}
              placeholder="••••••••"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label={t("settings.integrations.llm.textModel")}
              value={form.llm_model_text ?? "Qwen/Qwen3-30B-A3B"}
              onChange={(e) => set("llm_model_text", e.target.value)}
              placeholder="Qwen/Qwen3-30B-A3B"
            />
            <Input
              label={t("settings.integrations.llm.visionModel")}
              value={form.llm_model_vision ?? "Qwen/Qwen3-VL-30B-A3B-Thinking"}
              onChange={(e) => set("llm_model_vision", e.target.value)}
              placeholder="Qwen/Qwen3-VL-30B-A3B-Thinking"
            />
            <Input
              label={t("settings.integrations.llm.maxTokens")}
              type="number"
              value={form.llm_max_tokens ?? "2000"}
              onChange={(e) => set("llm_max_tokens", e.target.value)}
              placeholder="2000"
            />
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              {t("settings.integrations.llm.saveButton")}
            </Button>
            <Button
              variant="secondary"
              onClick={() => { setTestResult(null); testMutation.mutate(); }}
              loading={testMutation.isPending}
            >
              {t("settings.integrations.testConnection")}
            </Button>
            {testResult && (
              <span className={`text-xs ${testResult.success ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
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
  const { t } = useLocale();
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
          <p className="text-xs text-[var(--text-muted)] mt-0.5">{t("settings.integrations.coolify.hint")}</p>
        </div>
        <Toggle
          checked={enabled}
          onChange={(v) => set("coolify_enabled", v ? "true" : "false")}
          ariaLabel={t("settings.integrations.coolify.ariaEnableCoolify")}
        />
      </div>

      {enabled && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Input
              label={t("settings.integrations.baseUrl")}
              value={form.coolify_base_url ?? ""}
              onChange={(e) => set("coolify_base_url", e.target.value)}
              placeholder="https://coolify.example.com"
            />
            <Input
              label={t("settings.integrations.apiToken")}
              type="password"
              value={form.coolify_api_token ?? ""}
              onChange={(e) => set("coolify_api_token", e.target.value)}
              placeholder="••••••••"
            />
            <Input
              label={t("settings.integrations.coolify.sshUser")}
              value={form.coolify_default_user ?? ""}
              onChange={(e) => set("coolify_default_user", e.target.value)}
              placeholder="root"
            />
          </div>
          <p className="text-2xs text-[var(--text-faint)]">
            {t("settings.integrations.coolify.sshUserHint")}
          </p>

          <div className="flex items-center gap-2">
            <Button onClick={() => mutation.mutate(form)} loading={mutation.isPending}>
              {t("settings.integrations.coolify.saveButton")}
            </Button>
            <Button variant="secondary" onClick={() => { setTestResult(null); coolifyAPI.testConnection().then(setTestResult).catch(err => setTestResult({ success: false, error: err instanceof Error ? err.message : t("settings.integrations.testFailed") })); }} loading={false}>
              {t("settings.integrations.testConnection")}
            </Button>
          </div>
          {testResult && (
            <div className={`rounded-[var(--radius-sm)] px-3 py-2 text-xs ${testResult.success ? "bg-[var(--success)]/10 border border-[var(--success)]/25 text-[var(--success)]" : "bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)]"}`}>
              {testResult.success ? t("settings.integrations.connectionSuccessful") : t("settings.integrations.stageFailedDetail", { stage: t("settings.integrations.connectionFailed"), error: testResult.error ?? "" })}
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
