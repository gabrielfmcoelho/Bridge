"use client";

import { useLocale } from "@/contexts/LocaleContext";
import NativeSelect from "@/components/ui/NativeSelect";
import SectionCard from "@/components/ui/SectionCard";
import IntegrationCard from "./IntegrationCard";
import { useIntegrationForm } from "./useIntegrationForm";

const AUTH_PROVIDERS = [
  { value: "local", labelKey: "settings.integrations.auth.providerLocalLabel", descKey: "settings.integrations.auth.providerLocalDesc", color: "var(--cyan)" },
  { value: "ldap", labelKey: "settings.integrations.auth.providerLdapLabel", descKey: "settings.integrations.auth.providerLdapDesc", color: "var(--info)" },
  { value: "keycloak", labelKey: "settings.integrations.auth.providerKeycloakLabel", descKey: "settings.integrations.auth.providerKeycloakDesc", color: "var(--success)" },
  { value: "gitlab", labelKey: "settings.integrations.auth.providerGitlabLabel", descKey: "settings.integrations.auth.providerGitlabDesc", color: "var(--warning)" },
];

export default function GeneralAuthSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("general");
  const activeProvider = f.form.auth_active_provider ?? "local";
  const select = (key: string, fallback: string, label: string, options: [string, string][]) => (
    <NativeSelect label={label} value={f.form[key] ?? fallback} onChange={(e) => f.set(key, e.target.value)}>
      {options.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
    </NativeSelect>
  );
  const onOff: [string, string][] = [["true", t("settings.integrations.enabled")], ["false", t("settings.integrations.disabled")]];

  return (
    <IntegrationCard form={f} title={t("settings.integrations.auth.title")} hint={t("settings.integrations.auth.hint")}>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
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
              onChange={() => f.set("auth_active_provider", p.value)}
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

      {/* Provisioning settings only matter for an external provider. */}
      {activeProvider !== "local" && (
        <SectionCard as="h3" variant="plain" title={t("settings.integrations.auth.externalSettingsTitle")} className="pt-2">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {select("auth_auto_provision", "true", t("settings.integrations.auth.autoProvisionUsers"), onOff)}
            {select("auth_default_role", "viewer", t("settings.integrations.auth.defaultRole"), [
              ["viewer", t("settings.integrations.auth.roleViewer")],
              ["editor", t("settings.integrations.auth.roleEditor")],
              ["admin", t("settings.integrations.auth.roleAdmin")],
            ])}
            {select("auth_role_sync_enabled", "false", t("settings.integrations.auth.syncRoles"), [...onOff].reverse())}
          </div>
        </SectionCard>
      )}
    </IntegrationCard>
  );
}
