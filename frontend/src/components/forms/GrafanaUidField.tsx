"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { grafanaAPI, integrationsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Input from "@/components/ui/Input";
import Button from "@/components/ui/Button";

/**
 * The Grafana dashboard UID of a host or service, plus "provision" once the
 * asset exists (the endpoint needs its slug/id). Replaces the two copies that
 * lived in HostForm and ServiceForm.
 */
export default function GrafanaUidField({ kind, target, value, onChange }: {
  kind: "host" | "service";
  /** The saved asset: slug for hosts, id for services; absent on create. */
  target?: string | number;
  value: string;
  onChange: (uid: string) => void;
}) {
  const { t } = useLocale();
  const { data: integrations } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get, retry: false, staleTime: 60_000 });
  const enabled = integrations?.grafana?.grafana_enabled === "true";
  const datasourceSet = !!integrations?.grafana?.grafana_datasource_uid;
  const provision = useMutation({
    mutationFn: () => (kind === "host" ? grafanaAPI.provisionHostDashboard(String(target)) : grafanaAPI.provisionServiceDashboard(Number(target))),
    onSuccess: (res) => onChange(res.uid),
  });

  return (
    <div className="space-y-2">
      <Input
        label={t(kind === "host" ? "host.grafanaDashboardUidLabel" : "service.grafanaDashboardUidLabel")}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={t(kind === "host" ? "host.grafanaDashboardUidPlaceholder" : "service.grafanaDashboardUidPlaceholder")}
        hint={kind === "host" ? t("host.grafanaDashboardHint") : undefined}
      />
      {enabled && target != null && (
        <div className="space-y-1">
          <Button type="button" size="sm" variant="ghost" onClick={() => provision.mutate()} loading={provision.isPending} disabled={!datasourceSet}>
            {t("host.grafanaProvisionButton")}
          </Button>
          {!datasourceSet && <p className="text-xs text-[var(--warning)]">{t("host.grafanaDatasourceRequired")}</p>}
          {provision.isSuccess && <p className="text-xs text-[var(--success)]">{provision.data?.message}</p>}
          {provision.isError && <p className="text-xs text-[var(--danger)]">{provision.error instanceof Error ? provision.error.message : t("host.grafanaProvisionFailed")}</p>}
        </div>
      )}
    </div>
  );
}
