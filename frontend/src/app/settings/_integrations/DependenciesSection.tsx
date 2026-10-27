"use client";

import { useLocale } from "@/contexts/LocaleContext";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import { useIntegrationForm } from "./useIntegrationForm";

/** CVE checks of the dependency inventory: the "Evaluate CVEs" button on
 *  /vulnerabilities › Dependencies asks OSV, NVD and endoflife.date. */
export default function DependenciesSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("dependencies");
  return (
    <IntegrationCard
      form={f}
      title={t("settings.integrations.dependencies.title")}
      hint={t("settings.integrations.dependencies.hint")}
      enabledKey="deps_cve_enabled"
      toggleLabel={t("settings.integrations.dependencies.ariaEnable")}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <SecretInputWithClear form={f} name="nvd_api_key" label={t("settings.integrations.dependencies.nvdKey")} />
      </div>
      <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.dependencies.nvdKeyHint")}</p>
    </IntegrationCard>
  );
}
