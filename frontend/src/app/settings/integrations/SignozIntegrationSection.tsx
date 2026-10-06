"use client";

import { useLocale } from "@/contexts/LocaleContext";
import Input from "@/components/ui/Input";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import { useIntegrationForm } from "./useIntegrationForm";

export default function SignozIntegrationSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("signoz");
  const text = (key: string, label: string, placeholder: string) => (
    <Input label={label} value={f.form[key] ?? ""} onChange={(e) => f.set(key, e.target.value)} placeholder={placeholder} />
  );

  return (
    <IntegrationCard
      form={f}
      title="SigNoz"
      hint={t("settings.integrations.signoz.hint")}
      enabledKey="signoz_enabled"
      toggleLabel={t("settings.integrations.signoz.ariaEnableIntegration")}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {text("signoz_ch_url", t("settings.integrations.signoz.chUrl"), "http://10.0.0.10:8123")}
        {text("signoz_ui_url", t("settings.integrations.signoz.uiUrl"), "https://signoz.example.org")}
        {text("signoz_ch_user", t("settings.integrations.signoz.chUser"), "bridge_ro")}
        <SecretInputWithClear form={f} name="signoz_ch_password" label={t("settings.integrations.signoz.chPassword")} />
      </div>
      <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.signoz.chHint")}</p>
    </IntegrationCard>
  );
}
