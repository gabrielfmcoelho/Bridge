"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { coolifyAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

export default function CoolifySection() {
  const { t } = useLocale();
  const f = useIntegrationForm("coolify");

  return (
    <IntegrationCard
      form={f}
      title="Coolify"
      hint={t("settings.integrations.coolify.hint")}
      enabledKey="coolify_enabled"
      toggleLabel={t("settings.integrations.coolify.ariaEnableCoolify")}
    >
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Input
          label={t("settings.integrations.baseUrl")}
          value={f.form.coolify_base_url ?? ""}
          onChange={(e) => f.set("coolify_base_url", e.target.value)}
          placeholder="https://coolify.example.com"
        />
        <SecretInputWithClear form={f} name="coolify_api_token" label={t("settings.integrations.apiToken")} />
        <Input
          label={t("settings.integrations.coolify.sshUser")}
          value={f.form.coolify_default_user ?? ""}
          onChange={(e) => f.set("coolify_default_user", e.target.value)}
          placeholder="root"
          hint={t("settings.integrations.coolify.sshUserHint")}
        />
      </div>
      {/* The endpoint tests the saved settings, so unsaved edits are not part of it. */}
      <TestConnectionButton
        disabled={f.dirty}
        run={async () => {
          const res = await coolifyAPI.testConnection();
          return { success: res.success, message: res.success ? t("settings.integrations.connectionSuccessful") : failedLine(t, res.error) };
        }}
      />
    </IntegrationCard>
  );
}
