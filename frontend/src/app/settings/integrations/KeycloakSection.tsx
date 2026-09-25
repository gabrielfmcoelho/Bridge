"use client";

import { useLocale } from "@/contexts/LocaleContext";
import Input from "@/components/ui/Input";
import PillButton from "@/components/ui/PillButton";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import CallbackUrl from "./CallbackUrl";
import { useIntegrationForm } from "./useIntegrationForm";

// Environment presets from the PI Login docs.
const PRESETS: Record<string, { url: string; realm: string }> = {
  dev: { url: "https://dev-login.pi.gov.br", realm: "pi" },
  homolog: { url: "https://homolog-login.pi.gov.br", realm: "pi" },
  prod: { url: "https://login.pi.gov.br", realm: "pi" },
};

export default function KeycloakSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("keycloak");

  return (
    <IntegrationCard form={f} title={t("settings.integrations.keycloak.title")}>
      <div>
        <p className="text-xs font-medium text-[var(--text-muted)] mb-1.5">{t("settings.integrations.keycloak.environmentPresets")}</p>
        <div className="flex gap-2">
          {Object.entries(PRESETS).map(([env, p]) => (
            <PillButton
              key={env}
              active={f.form.auth_keycloak_base_url === p.url}
              onClick={() => {
                f.set("auth_keycloak_base_url", p.url);
                f.set("auth_keycloak_realm", p.realm);
              }}
            >
              {env}
            </PillButton>
          ))}
        </div>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label={t("settings.integrations.baseUrl")}
          value={f.form.auth_keycloak_base_url ?? ""}
          onChange={(e) => f.set("auth_keycloak_base_url", e.target.value)}
          placeholder="https://login.pi.gov.br"
        />
        <Input
          label="Realm"
          value={f.form.auth_keycloak_realm ?? "pi"}
          onChange={(e) => f.set("auth_keycloak_realm", e.target.value)}
          placeholder="pi"
        />
        <Input
          label={t("settings.integrations.keycloak.clientId")}
          value={f.form.auth_keycloak_client_id ?? ""}
          onChange={(e) => f.set("auth_keycloak_client_id", e.target.value)}
          placeholder="my-app-client"
        />
        <SecretInputWithClear form={f} name="auth_keycloak_client_secret" label={t("settings.integrations.clientSecret")} />
      </div>
      <CallbackUrl label={t("settings.integrations.keycloak.redirectUriHint")} path="/api/auth/oauth/keycloak/callback" />
    </IntegrationCard>
  );
}
