"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

export default function GestorSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("gestor");

  return (
    <IntegrationCard
      form={f}
      title="Gestor"
      hint={t("settings.integrations.gestor.hint")}
      enabledKey="gestor_integration_enabled"
      toggleLabel={t("settings.integrations.gestor.ariaEnable")}
    >
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Input
          label={t("settings.integrations.baseUrl")}
          value={f.form.gestor_base_url ?? ""}
          onChange={(e) => f.set("gestor_base_url", e.target.value)}
          placeholder="https://gateway.sead.pi.gov.br/gestor"
        />
        <Input
          label={t("settings.integrations.keycloak.clientId")}
          value={f.form.gestor_client_id ?? ""}
          onChange={(e) => f.set("gestor_client_id", e.target.value)}
        />
        <SecretInputWithClear form={f} name="gestor_client_secret" label={t("settings.integrations.clientSecret")} />
      </div>
      <Textarea
        label={t("settings.integrations.gestor.gitlabGroups")}
        value={f.form.gestor_gitlab_grupos ?? ""}
        onChange={(e) => f.set("gestor_gitlab_grupos", e.target.value)}
        placeholder="seadpi"
        rows={2}
        hint={t("settings.integrations.gestor.gitlabGroupsHint")}
      />
      <div className="flex flex-wrap items-center gap-4">
        {/* The endpoint tests the saved settings, so unsaved edits are not part of it. */}
        <TestConnectionButton
          disabled={f.dirty}
          run={async () => {
            const res = await integrationsAPI.testGestor();
            return { success: res.success, message: res.success ? t("settings.integrations.connectionSuccessful") : failedLine(t, res.error) };
          }}
        />
        <Link href="/settings/gestor/autores" className="text-sm text-[var(--accent)] hover:underline">
          {t("settings.integrations.gestor.authorsLink")}
        </Link>
      </div>
    </IntegrationCard>
  );
}
