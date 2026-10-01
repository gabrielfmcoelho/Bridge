"use client";

import { useLocale } from "@/contexts/LocaleContext";
import Input from "@/components/ui/Input";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import { useIntegrationForm } from "./useIntegrationForm";

// The Keycloak realm that governs SEAD's APIs (keycloak_apis): Atlas manages
// their access keys as clients through the admin service account, reads
// usage through a separate usage client (its token is the only one sent to
// an API), and
// Bridge's own API accepts the realm's tokens. Servers reach Keycloak on an
// internal address with the public name as Host.
export default function KeycloakAPIsSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("keycloak_apis");

  return (
    <IntegrationCard form={f} title={t("settings.integrations.keycloakApis.title")} hint={t("settings.integrations.keycloakApis.hint")}>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label={t("settings.integrations.keycloakApis.publicUrl")}
          value={f.form.kc_apis_base_url ?? ""}
          onChange={(e) => f.set("kc_apis_base_url", e.target.value)}
          placeholder="http://keycloak.10.0.122.89.sslip.io"
        />
        <Input
          label="Realm"
          value={f.form.kc_apis_realm ?? ""}
          onChange={(e) => f.set("kc_apis_realm", e.target.value)}
          placeholder="apis"
        />
        <Input
          label={t("settings.integrations.keycloakApis.internalUrl")}
          hint={t("settings.integrations.keycloakApis.internalUrlHint")}
          value={f.form.kc_apis_internal_url ?? ""}
          onChange={(e) => f.set("kc_apis_internal_url", e.target.value)}
          placeholder="http://10.0.122.89"
        />
        <Input
          label={t("settings.integrations.keycloakApis.hostHeader")}
          hint={t("settings.integrations.keycloakApis.hostHeaderHint")}
          value={f.form.kc_apis_host_header ?? ""}
          onChange={(e) => f.set("kc_apis_host_header", e.target.value)}
          placeholder="keycloak.10.0.122.89.sslip.io"
        />
        <Input
          label={t("settings.integrations.keycloak.clientId")}
          value={f.form.kc_apis_client_id ?? ""}
          onChange={(e) => f.set("kc_apis_client_id", e.target.value)}
          hint={t("settings.integrations.keycloakApis.adminClientHint")}
          placeholder="kc-bridge-admin"
        />
        <SecretInputWithClear form={f} name="kc_apis_client_secret" label={t("settings.integrations.clientSecret")} />
        <Input
          label={t("settings.integrations.keycloakApis.usageClientId")}
          hint={t("settings.integrations.keycloakApis.usageClientHint")}
          value={f.form.kc_apis_usage_client_id ?? ""}
          onChange={(e) => f.set("kc_apis_usage_client_id", e.target.value)}
          placeholder="kc-bridge-usage"
        />
        <SecretInputWithClear form={f} name="kc_apis_usage_client_secret" label={t("settings.integrations.keycloakApis.usageClientSecret")} />
      </div>
    </IntegrationCard>
  );
}
