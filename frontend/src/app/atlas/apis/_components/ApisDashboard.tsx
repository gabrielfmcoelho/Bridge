"use client";

import { useLocale } from "@/contexts/LocaleContext";
import InsightKpis from "@/components/inventory/InsightKpis";
import Breakdowns, { type BreakdownBlock } from "@/components/inventory/Breakdowns";
import type { ApiCatalog } from "@/lib/types";
import { apiInsights, apiBreakdowns, DEFAULT_API_INSIGHTS, type ApiFilters } from "./apiInsights";
import { useApiLinkNames } from "./apiDisplay";

/** The APIs "Dashboard" tab: KPI tiles, then source, linked project, linked
 *  service and spec version as bar lists. A row that maps to a filter applies it. */
export default function ApisDashboard({ apis, filters, onApplyFilter }: {
  apis: ApiCatalog[];
  filters: ApiFilters;
  onApplyFilter: (f: Partial<ApiFilters>) => void;
}) {
  const { t } = useLocale();
  const names = useApiLinkNames();
  const b = apiBreakdowns(apis, (id) => names.project.get(id), (id) => names.service.get(id));
  const blocks: BreakdownBlock<ApiFilters>[] = [
    { title: t("atlas.apis.dash.bySource"), rows: b.source },
    { title: t("atlas.apis.dash.byProject"), rows: b.projects },
    { title: t("atlas.apis.dash.byService"), rows: b.services },
    { title: t("atlas.apis.dash.bySpec"), rows: b.spec },
  ];
  return (
    <div className="space-y-6">
      <InsightKpis insights={apiInsights(apis, t)} defaults={DEFAULT_API_INSIGHTS} storageKey="apis_kpis"
        filters={filters} onFiltersChange={(f) => onApplyFilter(f)} />
      <Breakdowns blocks={blocks} total={apis.length} onApply={onApplyFilter} />
    </div>
  );
}
