"use client";

import { useMemo, type ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { AtlasIndexes, AtlasFilters, TableRole } from "@/lib/atlas/types";
import type { TableLayer } from "@/lib/lineage/indexes";
import PillFilter from "./PillFilter";
import { getLayerStyle } from "./LayerBadge";

interface Props {
  indexes: AtlasIndexes;
  filters: AtlasFilters;
  onChange: (next: Partial<AtlasFilters>) => void;
  /** Optional: hide the role filter (pipeline page doesn't use it). */
  showRoleFilter?: boolean;
  /** Right-aligned slot: typically the view-mode toggle. */
  rightSlot?: ReactNode;
  /** Optional: open the cmd-k omnibar. */
  onOpenSearch?: () => void;
}

const LAYER_OPTIONS: TableLayer[] = ["source", "bronze", "silver", "gold", "iapep", "iaspi"];

export default function AtlasToolbar({ indexes, filters, onChange, showRoleFilter = true, rightSlot, onOpenSearch }: Props) {
  const { t } = useLocale();

  // Counts per filter option keep the UI honest about what's left after filtering.
  const namespaceCounts = useMemo(() => {
    const m = new Map<string, number>();
    for (const ns of indexes.namespaces) m.set(ns, indexes.tablesByNamespace.get(ns)?.length ?? 0);
    return m;
  }, [indexes]);

  const layerCounts = useMemo(() => {
    const m = new Map<TableLayer, number>();
    for (const t of indexes.tables) m.set(t.layer, (m.get(t.layer) ?? 0) + 1);
    return m;
  }, [indexes]);

  const roleCounts = useMemo(() => ({
    source: indexes.tables.filter(t => t.role === "source").length,
    built: indexes.tables.filter(t => t.role === "built").length,
  }), [indexes]);

  return (
    <div className="flex flex-col gap-3 px-1">
      {/* Top row: search + view toggle */}
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={onOpenSearch}
          className="group flex-1 min-w-0 inline-flex items-center gap-2 h-8 px-3 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] text-left text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:border-[var(--border-strong)] transition-colors"
        >
          <svg className="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <span className="truncate flex-1">{t("atlas.catalog.toolbar.searchPlaceholder")}</span>
          <kbd className="text-[9px] px-1 py-0.5 rounded bg-[var(--bg-elevated)] text-[var(--text-faint)] font-mono">⌘K</kbd>
        </button>
        {rightSlot}
      </div>

      {/* Filter row */}
      <div className="flex flex-col gap-2">
        <PillFilter<string>
          label={t("atlas.catalog.toolbar.domain")}
          options={indexes.namespaces.map(ns => ({ value: ns, label: ns, count: namespaceCounts.get(ns) }))}
          value={filters.domains}
          onChange={(domains) => onChange({ domains })}
        />
        <PillFilter<TableLayer>
          label={t("atlas.catalog.toolbar.layer")}
          options={LAYER_OPTIONS
            .filter(l => (layerCounts.get(l) ?? 0) > 0)
            .map(l => ({ value: l, label: l, count: layerCounts.get(l) }))}
          value={filters.layers as TableLayer[]}
          onChange={(layers) => onChange({ layers })}
          renderLead={(v) => <span className={`w-1.5 h-1.5 rounded-full ${getLayerStyle(v).dot}`} />}
        />
        {showRoleFilter && (
          <PillFilter<TableRole>
            label={t("atlas.catalog.toolbar.role")}
            options={[
              { value: "source", label: t("atlas.catalog.role.source"), count: roleCounts.source },
              { value: "built", label: t("atlas.catalog.role.built"), count: roleCounts.built },
            ]}
            value={filters.role === "all" ? [] : [filters.role]}
            onChange={(roles) => onChange({ role: roles.length === 0 ? "all" : roles[0] })}
          />
        )}
      </div>
    </div>
  );
}
