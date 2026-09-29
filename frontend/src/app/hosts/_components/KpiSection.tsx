"use client";

import InsightKpis from "@/components/inventory/InsightKpis";
import { useLocale } from "@/contexts/LocaleContext";
import type { Host, HostFilters, HostSortConfig } from "@/lib/types";
import type { SortConfig } from "@/lib/insights";
import { hostInsights, DEFAULT_HOST_INSIGHTS } from "./hostInsights";

const DEFAULT_SORT: HostSortConfig = { field: "nickname", direction: "asc" };

/** The hosts KPI strip: the host insight catalog through InsightKpis. */
export default function KpiSection({ hosts, filters, onFiltersChange, sort, onSortChange, layout = "grid" }: {
  hosts: Host[];
  filters: HostFilters;
  onFiltersChange: (f: HostFilters) => void;
  sort: HostSortConfig;
  onSortChange: (s: HostSortConfig) => void;
  /** "list" (compact grid) on Visão geral, "grid" (stat cards) on the Dashboard. */
  layout?: "grid" | "list";
}) {
  const { t } = useLocale();
  return (
    <InsightKpis
      insights={hostInsights(hosts, t)}
      defaults={DEFAULT_HOST_INSIGHTS}
      storageKey="hosts_kpis"
      filters={filters}
      onFiltersChange={onFiltersChange}
      sort={sort}
      onSortChange={(s: SortConfig) => onSortChange(s as HostSortConfig)}
      defaultSort={DEFAULT_SORT}
      layout={layout}
    />
  );
}
