"use client";

import { useLocale } from "@/contexts/LocaleContext";
import InsightKpis from "@/components/inventory/InsightKpis";
import Breakdowns, { type BreakdownBlock } from "@/components/inventory/Breakdowns";
import { useHostNames } from "@/hooks/useHostNames";
import type { Service } from "@/lib/types";
import type { ServiceFilters } from "../FilterDrawer";
import { serviceInsights, serviceBreakdowns, DEFAULT_SERVICE_INSIGHTS } from "./serviceInsights";

/** The Serviços "Dashboard" tab: the KPI tiles, then one bar list per
 *  breakdown — category, origin, runtime status, database engines, busiest
 *  hosts, tags. A row that maps to a filter applies it. */
export default function ServicesDashboard({ services, filters, onApplyFilter }: {
  services: Service[];
  filters: ServiceFilters;
  onApplyFilter: (f: Partial<ServiceFilters>) => void;
}) {
  const { t } = useLocale();
  const hostNames = useHostNames();
  const b = serviceBreakdowns(services, (id) => hostNames.get(id));
  const blocks: BreakdownBlock<ServiceFilters>[] = [
    { title: t("service.dash.byKind"), rows: b.kind },
    { title: t("service.dash.byOrigin"), rows: b.origin },
    { title: t("service.dash.byStatus"), rows: b.status },
    { title: t("service.dash.engines"), rows: b.engines },
    { title: t("service.dash.byHost"), rows: b.hosts },
    { title: t("inventory.dash.byTags"), rows: b.tags },
  ];
  return (
    <div className="space-y-6">
      <InsightKpis insights={serviceInsights(services, t)} defaults={DEFAULT_SERVICE_INSIGHTS} storageKey="services_kpis"
        filters={filters} onFiltersChange={(f) => onApplyFilter(f)} />
      <Breakdowns blocks={blocks} total={services.length} onApply={onApplyFilter} />
    </div>
  );
}
