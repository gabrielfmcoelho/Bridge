"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

export default function LDAPSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("ldap");
  const text = (key: string, label: string, placeholder: string, fallback = "") => (
    <Input label={label} value={f.form[key] ?? fallback} onChange={(e) => f.set(key, e.target.value)} placeholder={placeholder} />
  );

  return (
    <IntegrationCard form={f} title={t("settings.integrations.ldap.title")}>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {text("auth_ldap_host", t("settings.integrations.ldap.host"), "ldaps://ldap.example.com")}
        {text("auth_ldap_port", t("settings.integrations.ldap.port"), "636", "636")}
        <div className="grid grid-cols-2 gap-2">
          <NativeSelect label="TLS" value={f.form.auth_ldap_use_tls ?? "true"} onChange={(e) => f.set("auth_ldap_use_tls", e.target.value)}>
            <option value="true">{t("common.yes")}</option>
            <option value="false">{t("common.no")}</option>
          </NativeSelect>
          <NativeSelect
            label={t("settings.integrations.ldap.skipVerify")}
            value={f.form.auth_ldap_skip_verify ?? "false"}
            onChange={(e) => f.set("auth_ldap_skip_verify", e.target.value)}
          >
            <option value="false">{t("common.no")}</option>
            <option value="true">{t("common.yes")}</option>
          </NativeSelect>
        </div>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {text("auth_ldap_base_dn", "Base DN", "dc=example,dc=com")}
        {text("auth_ldap_bind_dn", "Bind DN", "cn=admin,dc=example,dc=com")}
      </div>
      <SecretInputWithClear form={f} name="auth_ldap_bind_password" label={t("settings.integrations.ldap.bindPassword")} />
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {text("auth_ldap_user_filter", t("settings.integrations.ldap.userFilter"), "(mail=%s)", "(mail=%s)")}
        <NativeSelect
          label={t("settings.integrations.ldap.fallbackToLocal")}
          value={f.form.auth_ldap_fallback_to_local ?? "true"}
          onChange={(e) => f.set("auth_ldap_fallback_to_local", e.target.value)}
        >
          <option value="true">{t("settings.integrations.enabled")}</option>
          <option value="false">{t("settings.integrations.disabled")}</option>
        </NativeSelect>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {text("auth_ldap_username_attr", t("settings.integrations.ldap.usernameAttr"), "uid", "uid")}
        {text("auth_ldap_display_name_attr", t("settings.integrations.ldap.displayNameAttr"), "cn", "cn")}
        {text("auth_ldap_email_attr", t("settings.integrations.ldap.emailAttr"), "mail", "mail")}
      </div>
      {/* The endpoint tests the saved settings, so unsaved edits are not part of it. */}
      <TestConnectionButton
        disabled={f.dirty}
        run={async () => {
          const res = await integrationsAPI.testLDAP();
          return { success: res.success, message: res.success ? t("settings.integrations.connectionSuccessful") : res.error || t("settings.integrations.connectionFailed") };
        }}
      />
    </IntegrationCard>
  );
}
