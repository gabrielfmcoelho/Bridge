"use client";

import { useQuery } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI, osUpdateAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import StatusAlert from "@/components/ui/StatusAlert";
import IntegrationCard from "./_integrations/IntegrationCard";
import TestConnectionButton, { failedLine } from "./_integrations/TestConnectionButton";
import { useIntegrationForm } from "./_integrations/useIntegrationForm";

/** Settings › Hosts › Backups de hosts: the feature's defaults. A host's
 *  profile overrides the paths, a run overrides them once; the connection is
 *  the MinIO integration's. */
export default function HostBackupsTab() {
  const { t } = useLocale();
  const f = useIntegrationForm("osbackup");
  // The server's defaults (what an empty field means), shown as placeholders.
  const { data: config } = useQuery({ queryKey: ["os-updates", "config"], queryFn: osUpdateAPI.config });
  const minioOn = f.data?.minio?.minio_enabled === "true";

  return (
    <div className="space-y-6 max-w-2xl">
      {f.data && !minioOn && <StatusAlert variant="warning">{t("settings.hostBackups.minioOff")}</StatusAlert>}
      <IntegrationCard form={f} title={t("settings.hostBackups.title")} hint={t("settings.hostBackups.hint")}>
        <Input label={t("settings.integrations.minio.prefix")} value={f.form.osbackup_prefix ?? ""}
          onChange={(e) => f.set("osbackup_prefix", e.target.value)} placeholder={config?.backup_prefix || "bridge/os-backups"}
          hint={t("settings.hostBackups.prefixHint")} />
        <Input label={t("settings.integrations.minio.retention")} type="number" min={0} value={f.form.osbackup_retention_days ?? ""}
          onChange={(e) => f.set("osbackup_retention_days", e.target.value)} placeholder="30" hint={t("settings.integrations.minio.retentionHint")} />
        <Textarea label={t("settings.integrations.minio.include")} rows={2} value={f.form.osbackup_include ?? ""}
          onChange={(e) => f.set("osbackup_include", e.target.value)} placeholder="/" hint={t("settings.hostBackups.includeHint")} />
        <Textarea label={t("settings.integrations.minio.exclude")} rows={3} value={f.form.osbackup_exclude ?? ""}
          onChange={(e) => f.set("osbackup_exclude", e.target.value)} placeholder={(config?.backup_exclude ?? []).join(" ")}
          hint={t("settings.integrations.minio.excludeHint")} />
        {/* Applies the saved retention: save first. */}
        <TestConnectionButton label={t("settings.hostBackups.applyRetention")} disabled={!minioOn || f.dirty}
          run={async () => {
            const res = await integrationsAPI.applyBackupRetention();
            if (!res.success) return { success: false, message: failedLine(t, res.error, t("settings.hostBackups.retentionStage")) };
            return { success: true, message: res.retention_days
              ? t("settings.integrations.minio.lifecycleOk", { days: String(res.retention_days) })
              : t("settings.integrations.minio.lifecycleOff") };
          }} />
      </IntegrationCard>
    </div>
  );
}
