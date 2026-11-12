"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm } from "./useIntegrationForm";

/** MinIO (S3): the connection only. What each feature keeps there is the
 *  feature's (host backups: Settings › Hosts › Backups de hosts). */
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
      <Input label={t("settings.integrations.minio.bucket")} value={f.form.minio_bucket ?? ""}
        onChange={(e) => f.set("minio_bucket", e.target.value)} placeholder="ops" />
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
