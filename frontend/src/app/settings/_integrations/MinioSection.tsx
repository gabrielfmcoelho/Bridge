"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import NativeSelect from "@/components/ui/NativeSelect";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

// Defaults shown as placeholders; the server applies the same ones when the
// fields are empty (internal/integrations/minio).
const DEFAULT_EXCLUDE = "/tmp /var/tmp /var/cache /var/lib/docker /var/lib/containerd /swapfile /swap.img /var/lib/bridge-osupdate /lost+found";

/** MinIO (S3): where OS update runs back hosts up before touching them. */
export default function MinioSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("minio");

  return (
    <IntegrationCard form={f} title="MinIO (S3)" hint={t("settings.integrations.minio.hint")}
      enabledKey="minio_enabled" toggleLabel={t("settings.integrations.minio.ariaEnable")}>
      <div className="grid grid-cols-1 sm:grid-cols-[2fr_1fr] gap-3">
        <Input label={t("settings.integrations.minio.endpoint")} value={f.form.minio_endpoint ?? ""}
          onChange={(e) => f.set("minio_endpoint", e.target.value)} placeholder="10.0.122.76:9000" />
        <NativeSelect label={t("settings.integrations.minio.protocol")} value={f.form.minio_use_ssl === "true" ? "true" : "false"}
          onChange={(e) => f.set("minio_use_ssl", e.target.value)}>
          <option value="false">HTTP</option>
          <option value="true">HTTPS</option>
        </NativeSelect>
      </div>
      <Input label={t("settings.integrations.minio.publicUrl")} value={f.form.minio_public_url ?? ""}
        onChange={(e) => f.set("minio_public_url", e.target.value)} placeholder="https://s3.example.gov.br"
        hint={t("settings.integrations.minio.publicUrlHint")} />
      <Input label={t("settings.integrations.minio.accessKey")} value={f.form.minio_access_key ?? ""}
        onChange={(e) => f.set("minio_access_key", e.target.value)} autoComplete="off" />
      <SecretInputWithClear form={f} name="minio_secret_key" label={t("settings.integrations.minio.secretKey")} />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Input label={t("settings.integrations.minio.bucket")} value={f.form.minio_bucket ?? ""}
          onChange={(e) => f.set("minio_bucket", e.target.value)} placeholder="ops" />
        <Input label={t("settings.integrations.minio.prefix")} value={f.form.osbackup_prefix ?? ""}
          onChange={(e) => f.set("osbackup_prefix", e.target.value)} placeholder="bridge/os-backups" />
      </div>
      <Textarea label={t("settings.integrations.minio.include")} rows={2} value={f.form.osbackup_include ?? ""}
        onChange={(e) => f.set("osbackup_include", e.target.value)} placeholder="/" hint={t("settings.integrations.minio.includeHint")} />
      <Textarea label={t("settings.integrations.minio.exclude")} rows={3} value={f.form.osbackup_exclude ?? ""}
        onChange={(e) => f.set("osbackup_exclude", e.target.value)} placeholder={DEFAULT_EXCLUDE} hint={t("settings.integrations.minio.excludeHint")} />
      {/* Tests the saved settings: save first. */}
      <TestConnectionButton
        run={async () => {
          const res = await integrationsAPI.testMinio();
          if (!res.success) return { success: false, message: failedLine(t, res.error) };
          return { success: true, message: t("settings.integrations.minio.bucketOk", { bucket: res.bucket ?? "" }) };
        }}
      />
    </IntegrationCard>
  );
}
