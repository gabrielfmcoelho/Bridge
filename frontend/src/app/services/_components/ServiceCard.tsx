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
  const borderColor = svc.is_external_dependency ? "#ef4444" : svc.developed_by === "internal" ? "#06b6d4" : "#f59e0b";
  const linkedHostsCount = svc.host_ids?.length || 0;
  const linkedDnsCount = svc.dns_ids?.length || 0;
  const depsCount = svc.depends_on_ids?.length || 0;

  return (
    <Link href={`/services/${svc.id}`}>
      <Card className="h-full border-l-[3px] flex flex-col overflow-hidden" style={{ borderLeftColor: borderColor }} clickIndicator="link">
        <CardHeader
          title={svc.nickname}
          subtitle={svc.service_type ? `${svc.service_type}${svc.service_subtype ? ` / ${svc.service_subtype}` : ""}` : undefined}
          description={svc.description || t("common.noDescription") || "-"}
          badge={
            svc.is_external_dependency ? (
              <Badge color="red" compact>{t("service.isExternalDependency") || "External Dep"}</Badge>
            ) : (
              <Badge color={svc.developed_by === "internal" ? "cyan" : "amber"} compact>
                {svc.developed_by === "internal" ? t("service.internal") || "Internal" : t("service.external") || "External"}
              </Badge>
            )
          }
        />

        <CardMetadataGrid
          items={[
            { label: t("service.technologyStack") || "Tech Stack", value: svc.technology_stack || "-", mono: true },
            { label: t("service.environment") || "Environment", value: svc.environment || "-" },
            { label: t("service.deployApproach") || "Deploy", value: svc.deploy_approach || "-" },
            { label: t("service.version") || "Version", value: svc.version || "-", mono: true },
          ]}
        />

        <CardTagsSection tags={svc.tags} />

        {/* Bottom indicators */}
        <div className="flex items-center gap-3 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4">
          <CardIndicator icon={ICON_PATHS.alert} count={svc.is_external_dependency ? 1 : 0} color="red" title={svc.is_external_dependency ? t("service.isExternalDependency") : "Not external dependency"} />
          <CardIndicator icon={ICON_PATHS.gear} count={svc.orchestrator_managed ? 1 : 0} color="purple" title={svc.orchestrator_managed ? "Orchestrator managed" : "Not orchestrated"} />
          <CardIndicator icon={ICON_PATHS.server} count={linkedHostsCount} color="cyan" title={`${linkedHostsCount} host(s)`} />
          <CardIndicator icon={ICON_PATHS.globe} count={linkedDnsCount} color="emerald" title={`${linkedDnsCount} DNS`} />
          <CardIndicatorSeparator />
          <CardIndicator icon={ICON_PATHS.link} count={depsCount} color="amber" title={`${depsCount} dependencies`} />
          <CardIndicator icon={ICON_PATHS.terminal} count={svc.port ? 1 : 0} color="sky" title={svc.port ? `Port ${svc.port}` : "No port"} />
        </div>
      </Card>
    </Link>
  );
}
