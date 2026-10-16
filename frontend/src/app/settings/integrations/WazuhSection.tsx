"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { wazuhAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

/** Wazuh 4.8+ indexer: where the "Sync Wazuh" button on /vulnerabilities
 *  reads vulnerability states from. */
export default function WazuhSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("wazuh");

  return (
    <IntegrationCard
      form={f}
      title="Wazuh"
      hint={t("settings.integrations.wazuh.hint")}
      enabledKey="wazuh_enabled"
      toggleLabel={t("settings.integrations.wazuh.ariaEnable")}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label={t("settings.integrations.wazuh.indexerUrl")}
          value={f.form.wazuh_indexer_url ?? ""}
          onChange={(e) => f.set("wazuh_indexer_url", e.target.value)}
          placeholder="https://wazuh-indexer:9200"
        />
        <Input
          label={t("settings.integrations.wazuh.user")}
          value={f.form.wazuh_indexer_user ?? ""}
          onChange={(e) => f.set("wazuh_indexer_user", e.target.value)}
          hint={t("settings.integrations.wazuh.userHint")}
        />
        <SecretInputWithClear form={f} name="wazuh_indexer_password" label={t("settings.integrations.wazuh.password")} />
        <NativeSelect
          label={t("settings.integrations.ldap.skipVerify")}
          value={f.form.wazuh_tls_skip_verify === "true" ? "true" : "false"}
          onChange={(e) => f.set("wazuh_tls_skip_verify", e.target.value)}
        >
          <option value="false">{t("common.no")}</option>
          <option value="true">{t("common.yes")}</option>
        </NativeSelect>
      </div>
      {/* The endpoint tests the saved settings, so unsaved edits are not part of it. */}
      <TestConnectionButton
        disabled={f.dirty}
        run={async () => {
          const res = await wazuhAPI.test();
          return {
            success: res.success,
            message: res.success ? t("settings.integrations.wazuh.testOk", { count: String(res.count ?? 0) }) : failedLine(t, res.error),
          };
        }}
      />
    </IntegrationCard>
  );
}
