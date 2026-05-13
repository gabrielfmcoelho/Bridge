"use client";

import { useMemo } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { LineageIndexes } from "@/lib/lineage/indexes";
import { LAYERS } from "@/lib/lineage/types";
import { NODE_TYPE_LABELS } from "@/lib/lineage/style";

export type TraceMode = "none" | "upstream" | "downstream";

export interface GraphFilters {
  namespace: string;
  layer: string;
  type: string;
  onlyGaps: boolean;
  showColumnEdges: boolean;
  trace: TraceMode;
}

interface Props {
  indexes: LineageIndexes;
  filters: GraphFilters;
  onChange: (next: GraphFilters) => void;
  onOpenSearch: () => void;
}

export default function LineageToolbar({ indexes, filters, onChange, onOpenSearch }: Props) {
  const { t } = useLocale();
  const namespaces = useMemo(
    () => Array.from(indexes.nodesByNamespace.keys()).sort(),
    [indexes],
  );
  const typesPresent = useMemo(() => Array.from(indexes.nodesByType.keys()).sort(), [indexes]);

  return (
    <div className="flex flex-wrap gap-2 items-center p-2 bg-[var(--bg-surface)] text-xs">
      <Select
        label={t("atlas.lineage.toolbar.namespace")}
        value={filters.namespace}
        onChange={v => onChange({ ...filters, namespace: v })}
        options={[{ value: "all", label: t("common.all") }, ...namespaces.map(n => ({ value: n, label: n }))]}
      />
      <Select
        label={t("atlas.lineage.toolbar.layer")}
        value={filters.layer}
        onChange={v => onChange({ ...filters, layer: v })}
        options={[{ value: "all", label: t("common.all") }, ...LAYERS.map(l => ({ value: l, label: l }))]}
      />
      <Select
        label={t("atlas.lineage.toolbar.type")}
        value={filters.type}
        onChange={v => onChange({ ...filters, type: v })}
        options={[{ value: "all", label: t("common.all") }, ...typesPresent.map(t2 => ({ value: t2, label: NODE_TYPE_LABELS[t2] ?? t2 }))]}
      />

      <div className="h-5 border-l border-[var(--border-subtle)] mx-1" />

      <ToggleChip
        active={filters.onlyGaps}
        onClick={() => onChange({ ...filters, onlyGaps: !filters.onlyGaps })}
        label={t("atlas.lineage.toolbar.showOnlyGaps")}
      />
      <ToggleChip
        active={filters.showColumnEdges}
        onClick={() => onChange({ ...filters, showColumnEdges: !filters.showColumnEdges })}
        label={t("atlas.lineage.toolbar.showColumnEdges")}
      />

      <div className="h-5 border-l border-[var(--border-subtle)] mx-1" />

      <Select
        label={t("atlas.lineage.toolbar.trace")}
        value={filters.trace}
        onChange={v => onChange({ ...filters, trace: v as TraceMode })}
        options={[
          { value: "none", label: t("atlas.lineage.toolbar.traceOff") },
          { value: "upstream", label: t("atlas.lineage.toolbar.traceUpstream") },
          { value: "downstream", label: t("atlas.lineage.toolbar.traceDownstream") },
        ]}
      />

      <div className="ml-auto flex items-center gap-2">
        <button
          onClick={onOpenSearch}
          className="flex items-center gap-1.5 px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-base)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:border-[var(--border-default)] transition-colors"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <span>{t("atlas.lineage.toolbar.search")}</span>
          <kbd className="text-[9px] px-1 py-0.5 rounded bg-[var(--bg-elevated)] text-[var(--text-faint)]">⌘K</kbd>
        </button>
      </div>
    </div>
  );
}

function Select({ label, value, onChange, options }: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: Array<{ value: string; label: string }>;
}) {
  return (
    <label className="flex items-center gap-1.5 text-[var(--text-muted)]">
      <span className="text-[10px] uppercase tracking-wider">{label}</span>
      <select
        value={value}
        onChange={e => onChange(e.target.value)}
        className="bg-[var(--bg-base)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2 py-1 text-xs text-[var(--text-primary)] focus:outline-none focus:border-[var(--border-default)]"
      >
        {options.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
    </label>
  );
}

function ToggleChip({ active, onClick, label }: { active: boolean; onClick: () => void; label: string }) {
  return (
    <button
      onClick={onClick}
      className={`px-2.5 py-1 rounded-full text-[11px] border transition-colors ${
        active
          ? "bg-cyan-500/15 border-cyan-500/40 text-cyan-300"
          : "bg-[var(--bg-base)] border-[var(--border-subtle)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
      }`}
    >
      {label}
    </button>
  );
}
