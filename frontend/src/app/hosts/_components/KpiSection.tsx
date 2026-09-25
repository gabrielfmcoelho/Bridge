"use client";

import KpiGrid from "@/components/inventory/KpiGrid";
import KpiPicker from "@/components/inventory/KpiPicker";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Host, HostFilters, HostSortConfig } from "@/lib/types";
import { hostInsights, DEFAULT_HOST_INSIGHTS, type HostInsight } from "./hostInsights";

const DEFAULT_SORT: HostSortConfig = { field: "nickname", direction: "asc" };

/**
 * The admin's own KPI strip: which insights show is remembered per browser,
 * and a tile that maps to a filter or sort applies it on click (and clears it
 * on a second click). Counts describe the current listing.
 */
export default function KpiSection({
  hosts,
  filters,
  onFiltersChange,
  sort,
  onSortChange,
  layout = "grid",
}: {
  hosts: Host[];
  filters: HostFilters;
  onFiltersChange: (f: HostFilters) => void;
  sort: HostSortConfig;
  onSortChange: (s: HostSortConfig) => void;
  /** "list" in the Visão geral side column, "grid" on the Dashboard. */
  layout?: "grid" | "list";
}) {
  const { t } = useLocale();
  const [selected, setSelected] = useLocalStorage<string[]>("hosts_kpis", DEFAULT_HOST_INSIGHTS);
  const all = hostInsights(hosts, t);
  const byKey = new Map(all.map((i) => [i.key, i]));

  const isActive = (i: HostInsight) =>
    i.filter
      ? Object.entries(i.filter).every(([k, v]) => filters[k as keyof HostFilters] === v)
      : !!i.sort && sort.field === i.sort.field && sort.direction === i.sort.direction;

  const apply = (i: HostInsight) => {
    if (i.filter) {
      const cleared = Object.fromEntries(Object.keys(i.filter).map((k) => [k, ""]));
      onFiltersChange({ ...filters, ...(isActive(i) ? cleared : i.filter) });
    } else if (i.sort) {
      onSortChange(isActive(i) ? DEFAULT_SORT : i.sort);
    }
  };

  // A selected tag may have left the listing; its tile just drops out.
  const kpis = selected.flatMap((key) => {
    const i = byKey.get(key);
    if (!i) return [];
    const clickable = !!(i.filter || i.sort);
    return [{
      label: i.label,
      value: i.value,
      color: i.color,
      icon: ICON_PATHS[i.icon],
      hint: i.hint || undefined,
      onClick: clickable ? () => apply(i) : undefined,
      active: clickable && isActive(i),
    }];
  });

  return (
    <KpiGrid
      layout={layout}
      description={layout === "list" ? t("inventory.kpis.description") : undefined}
      kpis={kpis}
      heading={t("common.indicators")}
      columns={Math.min(Math.max(kpis.length, 2), 5) as 2 | 3 | 4 | 5}
      actions={
        <KpiPicker
          options={all.map((i) => ({
            key: i.key,
            label: i.label,
            group: i.key.startsWith("tag:") ? t("common.tags") : t("common.indicators"),
          }))}
          selected={selected}
          onChange={setSelected}
          onReset={() => setSelected(DEFAULT_HOST_INSIGHTS)}
        />
      }
    />
  );
}
