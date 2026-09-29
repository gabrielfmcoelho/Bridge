"use client";

import { useLocale } from "@/contexts/LocaleContext";
import Breakdowns, { type BreakdownBlock } from "@/components/inventory/Breakdowns";
import type { Host, HostFilters, HostSortConfig } from "@/lib/types";
import KpiSection from "./KpiSection";
import { hostBreakdowns } from "./hostInsights";

/**
 * The Hosts "Dashboard" tab: the same customizable KPI tiles as the side
 * column, then one bar list per breakdown of the current listing. A row that
 * maps to a list filter applies it (and the page returns to Visão geral).
 */
export default function HostsDashboard({
  hosts,
  filters,
  sort,
  onSortChange,
  onApplyFilter,
}: {
  hosts: Host[];
  filters: HostFilters;
  sort: HostSortConfig;
  onSortChange: (s: HostSortConfig) => void;
  onApplyFilter: (f: Partial<HostFilters>) => void;
}) {
  const { t } = useLocale();
  const b = hostBreakdowns(hosts);
  const blocks: BreakdownBlock<HostFilters>[] = [
    { title: t("inventory.dash.bySituacao"), rows: b.situacao },
    { title: t("host.dash.byUsage"), rows: b.usage },
    { title: t("host.dash.alertsByLevel"), rows: b.alerts },
    { title: t("host.dash.byHospedagem"), rows: b.hospedagem },
    { title: t("inventory.dash.byEntidade"), rows: b.entidade },
    { title: t("inventory.dash.byTags"), rows: b.tags },
  ];

  return (
    <div className="space-y-6">
      <KpiSection hosts={hosts} filters={filters} onFiltersChange={(f) => onApplyFilter(f)} sort={sort} onSortChange={onSortChange} />
      <Breakdowns blocks={blocks} total={hosts.length} onApply={onApplyFilter} />
    </div>
  );
}
