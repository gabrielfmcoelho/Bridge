"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { StatusText } from "@/components/ui/SituacaoText";
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
      <Card accent={accent} className="h-full flex flex-col overflow-hidden">
        {/* Fixed anatomy: every slot renders, "–" when empty. Origin and
            discovery moved from header chips into the grid; internal/external
            is the accent stripe plus the first indicator. */}
        <CardHeader
          titleFont="display"
          title={svc.nickname}
          subtitle={svc.service_type ? `${svc.service_type}${svc.service_subtype ? ` / ${svc.service_subtype}` : ""}` : undefined}
          subtitleFont="display"
          status={
            svc.container_status ? (
              <StatusText
                color={svc.container_status === "online" ? "var(--success)" : "var(--text-muted)"}
                on={svc.container_status === "online"}
                label={svc.container_status === "online" ? t("service.containerOnline") : t("service.containerOffline")}
              />
            ) : undefined
          }
          description={svc.description}
        />

        <CardMetadataGrid
          items={[
            { label: t("service.technologyStack"), value: svc.technology_stack || "", mono: true },
            { label: t("service.environment"), value: svc.environment || "" },
            { label: t("service.deployApproach"), value: svc.deploy_approach || "" },
            { label: t("service.version"), value: svc.version || "", mono: true },
            { label: t("service.source"), value: svc.source === "auto" ? t("service.sourceAuto") : svc.source === "fixed" ? t("service.sourceFixed") : t("service.sourceManual") },
            { label: t("service.discovery"), value: svc.discovery_kind === "container" ? t("service.kindContainer") : svc.discovery_kind ? t("service.kindHost") : "" },
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
