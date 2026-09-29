"use client";

import KpiGrid from "@/components/inventory/KpiGrid";
import KpiPicker from "@/components/inventory/KpiPicker";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Insight, SortConfig } from "@/lib/insights";

/**
 * An inventory's KPI strip, from its insight catalog: which insights show is
 * remembered per browser (`storageKey`), and a tile that maps to a filter or
 * sort applies it on click (and clears it on a second click). "list" is the
 * compact grid of the Visão geral column; "grid" the Dashboard's stat cards.
 */
export default function InsightKpis<F extends object>({
  insights, defaults, storageKey, filters, onFiltersChange, sort, onSortChange, defaultSort, layout = "grid",
}: {
  insights: Insight<F>[];
  defaults: string[];
  storageKey: string;
  filters: F;
  onFiltersChange: (f: F) => void;
  sort?: SortConfig;
  onSortChange?: (s: SortConfig) => void;
  defaultSort?: SortConfig;
  layout?: "grid" | "list";
}) {
  const { t } = useLocale();
  const [selected, setSelected] = useLocalStorage<string[]>(storageKey, defaults);
  const byKey = new Map(insights.map((i) => [i.key, i]));

  const isActive = (i: Insight<F>) =>
    i.filter
      ? Object.entries(i.filter).every(([k, v]) => (filters as Record<string, unknown>)[k] === v)
      : !!i.sort && sort?.field === i.sort.field && sort?.direction === i.sort.direction;

  const apply = (i: Insight<F>) => {
    if (i.filter) {
      const cleared = Object.fromEntries(Object.keys(i.filter).map((k) => [k, ""]));
      onFiltersChange({ ...filters, ...(isActive(i) ? cleared : i.filter) });
    } else if (i.sort && onSortChange) {
      onSortChange(isActive(i) && defaultSort ? defaultSort : i.sort);
    }
  };

  // A selected tag may have left the listing; its tile just drops out.
  const kpis = selected.flatMap((key) => {
    const i = byKey.get(key);
    if (!i) return [];
    const clickable = !!(i.filter || (i.sort && onSortChange));
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
      kpis={kpis}
      heading={t("common.indicators")}
      columns={Math.min(Math.max(kpis.length, 2), 5) as 2 | 3 | 4 | 5}
      actions={
        <KpiPicker
          options={insights.map((i) => ({
            key: i.key,
            label: i.label,
            group: i.key.startsWith("tag:") ? t("common.tags") : t("common.indicators"),
          }))}
          selected={selected}
          onChange={setSelected}
          onReset={() => setSelected(defaults)}
        />
      }
    />
  );
}
