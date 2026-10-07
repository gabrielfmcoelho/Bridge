"use client";

import { useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

/** Outgoing email, used to send share links (link + passphrase) to recipients. */
export default function SMTPSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("smtp");
  const [to, setTo] = useState("");

  return (
    <IntegrationCard
      form={f}
      title={t("settings.integrations.smtp.title")}
      hint={t("settings.integrations.smtp.hint")}
      enabledKey="smtp_enabled"
      toggleLabel={t("settings.integrations.smtp.ariaEnable")}
    >
      <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
        <Input
          label={t("settings.integrations.smtp.host")}
          value={f.form.smtp_host ?? ""}
          onChange={(e) => f.set("smtp_host", e.target.value)}
          placeholder="smtp.example.org"
        />
        <Input
          label={t("settings.integrations.smtp.port")}
          type="number"
          value={f.form.smtp_port ?? ""}
          onChange={(e) => f.set("smtp_port", e.target.value)}
          placeholder="587"
        />
        <NativeSelect
          label={t("settings.integrations.smtp.tls")}
          value={f.form.smtp_tls || "starttls"}
          onChange={(e) => f.set("smtp_tls", e.target.value)}
        >
          <option value="starttls">STARTTLS (587)</option>
          <option value="tls">TLS (465)</option>
          <option value="none">{t("settings.integrations.smtp.tlsNone")}</option>
        </NativeSelect>
      </div>
      <Input
        label={t("settings.integrations.smtp.username")}
        value={f.form.smtp_username ?? ""}
        onChange={(e) => f.set("smtp_username", e.target.value)}
        autoComplete="off"
      />
      <SecretInputWithClear form={f} name="smtp_password" label={t("settings.integrations.smtp.password")} />
      <Input
        label={t("settings.integrations.smtp.from")}
        value={f.form.smtp_from ?? ""}
        onChange={(e) => f.set("smtp_from", e.target.value)}
        placeholder="Bridge <bridge@example.org>"
      />
      <Input
        label={t("settings.integrations.smtp.linkBaseUrl")}
        value={f.form.smtp_link_base_url ?? ""}
        onChange={(e) => f.set("smtp_link_base_url", e.target.value)}
        placeholder="https://bridge.example.org"
      />
      <p className="text-xs text-[var(--text-muted)] -mt-2">{t("settings.integrations.smtp.linkBaseUrlHint")}</p>

      {/* Tests what is SAVED: the server reads its own settings. */}
      <Input
        label={t("settings.integrations.smtp.testTo")}
        type="email"
        value={to}
        onChange={(e) => setTo(e.target.value)}
        placeholder={t("settings.integrations.smtp.testToPlaceholder")}
      />
      <TestConnectionButton
        disabled={f.dirty}
        run={async () => {
          const res = await integrationsAPI.testSMTP(to.trim());
          return res.success
            ? { success: true, message: t("settings.integrations.smtp.testSent", { to: res.to || "" }) }
            : { success: false, message: failedLine(t, res.error) };
        }}
      />
    </IntegrationCard>
  );
}
