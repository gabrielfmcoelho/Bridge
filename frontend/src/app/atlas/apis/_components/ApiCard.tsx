"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { CardHeader, CardMetadataGrid, CardIndicator, CardIndicatorGrid } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { ApiCatalog } from "@/lib/types";
import { isSpecStale } from "./apiInsights";
import { baseHost, linkedNames, useApiLinkNames } from "./apiDisplay";

export default function ApiCard({ api }: { api: ApiCatalog }) {
  const { t } = useLocale();
  const names = useApiLinkNames();
  const stale = isSpecStale(api);
  const services = api.service_ids ?? [];
  const projects = api.project_ids ?? [];
  const avulso = services.length + projects.length === 0;

  return (
    <Link href={`/atlas/apis/${api.id}`} className="block h-full">
      <Card accent={stale ? "warning" : avulso ? "muted" : "rose"} className="h-full flex flex-col overflow-hidden">
        <CardHeader
          titleFont="display"
          title={api.name}
          subtitle={baseHost(api) || undefined}
          description={api.description || api.title}
        />

        <CardMetadataGrid
          items={[
            { label: t("atlas.apis.specTitle"), value: [api.title, api.version_label].filter(Boolean).join(" · ") },
            { label: t("atlas.apis.specVersion"), value: api.spec_version, mono: true },
            { label: t("topology.projects"), value: linkedNames(projects, names.project) },
            { label: t("topology.services"), value: linkedNames(services, names.service) },
          ]}
        />

        <CardIndicatorGrid>
          <CardIndicator icon={ICON_PATHS.code} count={api.operation_count} color="info" title={t("atlas.apis.endpointsCount", { count: String(api.operation_count) })} />
          <CardIndicator icon={ICON_PATHS.serverStack} count={services.length} color="warning" title={t("service.title")} />
          <CardIndicator icon={ICON_PATHS.folder} count={projects.length} color="accent" title={t("topology.projects")} />
          <CardIndicator icon={ICON_PATHS.globe} count={api.base_url ? 1 : 0} color="success" hideCount title={api.base_url ? t("atlas.apis.baseUrl") : t("atlas.apis.kpi.noBaseUrl")} />
          <CardIndicator icon={ICON_PATHS.clock} count={stale ? 1 : 0} color="warning" hideCount title={stale ? t("atlas.apis.kpi.specStale") : t("atlas.apis.specFresh")} />
        </CardIndicatorGrid>
      </Card>
    </Link>
  );
}
