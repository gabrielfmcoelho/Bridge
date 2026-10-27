"use client";

import type { ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import CopyButton from "@/components/ui/CopyButton";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm, MASKED } from "./useIntegrationForm";

/** A collapsible group of fields inside the Grafana card: plain, so no card-in-card. */
function Group({ title, open = false, children }: { title: string; open?: boolean; children: ReactNode }) {
  return (
    <SectionCard as="h3" variant="plain" collapsible defaultOpen={open} title={title} className="pt-2">
      <div className="space-y-3">{children}</div>
    </SectionCard>
  );
}

export default function GrafanaIntegrationSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("grafana");
  const text = (key: string, label: string, placeholder: string) => (
    <Input label={label} value={f.form[key] ?? ""} onChange={(e) => f.set(key, e.target.value)} placeholder={placeholder} />
  );

  // A freshly generated webhook secret is plaintext until saved; after that
  // the server only returns the mask, so there is nothing left to copy.
  const webhookSecret = f.form.grafana_webhook_secret ?? "";
  const hasPlaintext = webhookSecret !== "" && webhookSecret !== MASKED;
  const generateSecret = () => {
    // 32 random bytes → 64 hex chars: ample for HMAC-SHA256, copy-paste safe.
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    f.set("grafana_webhook_secret", Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join(""));
  };

  return (
    <IntegrationCard
      form={f}
      title="Grafana"
      hint={t("settings.integrations.grafana.hint")}
      enabledKey="grafana_enabled"
      toggleLabel={t("settings.integrations.grafana.ariaEnableIntegration")}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {text("grafana_base_url", t("settings.integrations.baseUrl"), "https://grafana.example.org")}
        {text("grafana_datasource_uid", t("settings.integrations.grafana.datasourceUid"), "prometheus")}
      </div>
      <SecretInputWithClear form={f} name="grafana_api_token" label={t("settings.integrations.apiToken")} />

      <Group title={t("settings.integrations.grafana.defaultDashboardsHeading")} open>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {text("grafana_host_default_dashboard_uid", t("settings.integrations.grafana.defaultHostDashboardUid"), "node-exporter-full")}
          {text("grafana_service_default_dashboard_uid", t("settings.integrations.grafana.defaultServiceDashboardUid"), "service-overview")}
        </div>
        <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.grafana.dashboardVarHint")}</p>
      </Group>

      <Group title="Prometheus remote_write (Grafana Agent)">
        {text("grafana_prom_remote_write_url", t("settings.integrations.grafana.remoteWriteUrl"), "https://prometheus.example.org/api/v1/write")}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {text("grafana_prom_remote_write_username", t("settings.integrations.grafana.remoteWriteUsername"), "scrape-user")}
          <SecretInputWithClear
            form={f}
            name="grafana_prom_remote_write_password"
            label={t("settings.integrations.grafana.remoteWritePassword")}
          />
        </div>
        <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.grafana.remoteWriteHint")}</p>
      </Group>

      <Group title={t("settings.integrations.grafana.alertWebhookHeading")}>
        <SecretInputWithClear form={f} name="grafana_webhook_secret" label={t("settings.integrations.grafana.webhookHmacSecret")} />
        <div className="flex items-center flex-wrap gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={generateSecret}>
            {t("settings.integrations.grafana.generateSecretButton")}
          </Button>
          <CopyButton
            value={webhookSecret}
            size="sm"
            variant="ghost"
            disabled={!hasPlaintext}
            title={hasPlaintext ? t("settings.integrations.grafana.copyToClipboardTitle") : t("settings.integrations.grafana.generateSecretFirstTitle")}
          />
          <span className="text-xs text-[var(--text-muted)]">{t("settings.integrations.grafana.webhookCopyHint")}</span>
        </div>
        <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.grafana.webhookConfigHint")}</p>
      </Group>

      {/* Tests what is typed; an untouched (masked) token falls back to the stored one. */}
      <TestConnectionButton
        run={async () => {
          const token = f.form.grafana_api_token ?? "";
          const res = await integrationsAPI.testGrafana({
            base_url: f.form.grafana_base_url ?? "",
            token: token === MASKED ? "" : token,
          });
          if (!res.success) {
            return { success: false, message: failedLine(t, res.error, res.stage === "auth" ? t("settings.integrations.authFailed") : undefined) };
          }
          const parts: string[] = [];
          if (res.version) parts.push(`Grafana ${res.version}`);
          if (res.user) parts.push(`${t("settings.integrations.asUser", { user: res.user })}${res.name ? ` (${res.name})` : ""}`);
          if (res.org_id !== undefined) parts.push(t("settings.integrations.grafana.orgLabel", { id: String(res.org_id) }));
          return { success: true, message: parts.join(" · ") || t("settings.integrations.connected") };
        }}
      />
    </IntegrationCard>
  );
}
