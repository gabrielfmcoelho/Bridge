"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { proxmoxAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

export default function ProxmoxSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("proxmox");

  return (
    <IntegrationCard
      form={f}
      title="Proxmox VE"
      hint={t("settings.integrations.proxmox.hint")}
      enabledKey="proxmox_enabled"
      toggleLabel={t("settings.integrations.proxmox.ariaEnable")}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label={t("settings.integrations.baseUrl")}
          value={f.form.proxmox_base_url ?? ""}
          onChange={(e) => f.set("proxmox_base_url", e.target.value)}
          placeholder="https://pve.example.gov.br:8006"
        />
        <NativeSelect
          label={t("settings.integrations.ldap.skipVerify")}
          value={f.form.proxmox_skip_verify ?? "false"}
          onChange={(e) => f.set("proxmox_skip_verify", e.target.value)}
        >
          <option value="false">{t("common.no")}</option>
          <option value="true">{t("common.yes")}</option>
        </NativeSelect>
        <Input
          label={t("settings.integrations.proxmox.tokenId")}
          value={f.form.proxmox_token_id ?? ""}
          onChange={(e) => f.set("proxmox_token_id", e.target.value)}
          placeholder="bridge@pve!sync"
        />
        <SecretInputWithClear form={f} name="proxmox_token_secret" label={t("settings.integrations.proxmox.tokenSecret")} />
      </div>
      <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.proxmox.tokenHint")}</p>
      {/* Unsaved values win on the server; a masked secret falls back to the stored one. */}
      <TestConnectionButton
        run={async () => {
          const res = await proxmoxAPI.test({
            base_url: f.form.proxmox_base_url,
            token_id: f.form.proxmox_token_id,
            token_secret: f.form.proxmox_token_secret,
            skip_verify: f.form.proxmox_skip_verify === "true",
          });
          return res.success
            ? { success: true, message: t("settings.integrations.proxmox.connectedVersion", { version: res.version ?? "" }) }
            : { success: false, message: failedLine(t, res.error) };
        }}
      />
    </IntegrationCard>
  );
}
