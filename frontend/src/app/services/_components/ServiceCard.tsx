"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Service } from "@/lib/types";

export default function ServiceCard({ svc }: { svc: Service }) {
  const { t } = useLocale();
  // red = external dependency, cyan = built in-house, amber = external vendor
  const accent = svc.is_external_dependency ? "danger" : svc.developed_by === "internal" ? "cyan" : "warning";
  const linkedHostsCount = svc.host_ids?.length || 0;
  const linkedDnsCount = svc.dns_ids?.length || 0;
  const depsCount = svc.depends_on_ids?.length || 0;

  return (
    <Link href={`/services/${svc.id}`} className="block h-full">
      <Card accent={accent} className="h-full flex flex-col overflow-hidden" clickIndicator="link">
        <CardHeader
          title={svc.nickname}
          subtitle={svc.service_type ? `${svc.service_type}${svc.service_subtype ? ` / ${svc.service_subtype}` : ""}` : undefined}
          description={svc.description || t("common.noDescription")}
          badge={
            <div className="flex items-center gap-1.5">
              {svc.source !== "manual" && (
                <Badge color={svc.source === "auto" ? "blue" : "emerald"} compact>
                  {svc.source === "auto" ? t("service.sourceAuto") : t("service.sourceFixed")}
                </Badge>
              )}
              {svc.discovery_kind && (
                <Badge color={svc.discovery_kind === "container" ? "cyan" : "accent"} compact>
                  {svc.discovery_kind === "container" ? t("service.kindContainer") : t("service.kindHost")}
                </Badge>
              )}
              {svc.container_status && (
                <span className={`inline-block w-2 h-2 rounded-full ${svc.container_status === "online" ? "bg-[var(--success)]" : "bg-[var(--text-faint)]"}`} title={svc.container_status === "online" ? t("service.containerOnline") : t("service.containerOffline")} />
              )}
              {svc.is_external_dependency ? (
                <Badge color="red" compact>{t("service.isExternalDependency")}</Badge>
              ) : (
                <Badge color={svc.developed_by === "internal" ? "cyan" : "amber"} compact>
                  {svc.developed_by === "internal" ? t("service.internal") : t("service.external")}
                </Badge>
              )}
            </div>
          }
        />

        <CardMetadataGrid
          items={[
            { label: t("service.technologyStack"), value: svc.technology_stack || "-", mono: true },
            { label: t("service.environment"), value: svc.environment || "-" },
            { label: t("service.deployApproach"), value: svc.deploy_approach || "-" },
            { label: t("service.version"), value: svc.version || "-", mono: true },
          ]}
        />

        <CardTagsSection tags={svc.tags} />

        {/* Bottom indicators */}
        <div className="flex items-center gap-3 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4">
          <CardIndicator icon={ICON_PATHS.alert} count={svc.is_external_dependency ? 1 : 0} color="red" title={svc.is_external_dependency ? t("service.isExternalDependency") : t("service.notExternalDependency")} hideCount />
          <CardIndicator icon={ICON_PATHS.gear} count={svc.orchestrator_managed ? 1 : 0} color="accent" title={svc.orchestrator_managed ? t("service.orchestratorManaged") : t("service.notOrchestrated")} hideCount />
          <CardIndicator icon={ICON_PATHS.server} count={linkedHostsCount} color="cyan" title={t("service.hostsCount", { count: String(linkedHostsCount) })} />
          <CardIndicator icon={ICON_PATHS.globe} count={linkedDnsCount} color="emerald" title={t("service.dnsCount", { count: String(linkedDnsCount) })} />
          <CardIndicatorSeparator />
          <CardIndicator icon={ICON_PATHS.link} count={depsCount} color="amber" title={`${depsCount} dependencies`} />
          <CardIndicator icon={ICON_PATHS.terminal} count={svc.port ? 1 : 0} color="sky" title={svc.port ? t("service.portWithValue", { port: String(svc.port) }) : t("service.noPort")} hideCount />
        </div>
      </Card>
    </Link>
  );
}
