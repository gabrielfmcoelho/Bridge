"use client";

import { useMemo } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import StatCard from "@/components/ui/StatCard";
import type { LineageIndexes } from "@/lib/lineage/indexes";
import { LAYER_COLORS, NODE_TYPE_LABELS } from "@/lib/lineage/style";

const ICONS = {
  error:   "M12 9v2m0 4h.01M5.07 19h13.86c1.54 0 2.5-1.67 1.73-3L13.73 4a2 2 0 00-3.46 0L3.34 16c-.77 1.33.19 3 1.73 3z",
  orphan:  "M12 6v6m0 0v6m0-6h6m-6 0H6",
  isolate: "M12 2l9 4.5v9L12 20l-9-4.5v-9L12 2zm0 0v18",
  source:  "M4 7v10c0 2 1.79 4 4 4h8c2.21 0 4-2 4-4V7m-16 0c0-2 1.79-4 4-4h8c2.21 0 4 2 4 4m-16 0h16",
  macro:   "M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4",
  domain:  "M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z",
  layer:   "M5 12l-1.59 1.59a2 2 0 000 2.82l5.18 5.18a2 2 0 002.82 0L13 20M5 12l-1.59-1.59a2 2 0 010-2.82l5.18-5.18a2 2 0 012.82 0L13 4M5 12h14M13 4v16",
};

interface Props {
  indexes: LineageIndexes;
  onNavigate: (view: "overview" | "graph" | "issues", extra?: Record<string, string | undefined>) => void;
}

export default function OverviewPanel({ indexes, onNavigate }: Props) {
  const { t } = useLocale();
  const cov = indexes.raw.coverage;

  const stats = useMemo(() => {
    const warnings = indexes.raw.warnings ?? [];
    const errors = warnings.filter(w => w.severity === "error").length;
    const gaps = cov?.gaps ?? {};
    return {
      nodes: cov?.node_total ?? indexes.raw.nodes.length,
      edges: cov?.edge_total ?? indexes.raw.edges.length,
      errors,
      orphanTasks: gaps.orphan_tasks?.length ?? 0,
      isolatedModels: gaps.isolated_models?.length ?? 0,
      unusedSources: gaps.unused_sources?.length ?? 0,
      unusedMacros: gaps.unused_macros?.length ?? 0,
      columnEdges: cov?.column_lineage?.edges_emitted ?? 0,
      modelsCovered: cov?.column_lineage?.models_with_column_lineage?.length ?? 0,
      modelsTotal:
        (cov?.column_lineage?.models_with_column_lineage?.length ?? 0) +
        (cov?.column_lineage?.models_without_column_lineage?.length ?? 0),
    };
  }, [cov, indexes.raw]);

  const domains = useMemo(() => {
    const map = new Map<string, { dags: number; models: number }>();
    for (const n of indexes.raw.nodes) {
      const ns = n.namespace;
      if (!ns) continue;
      const cur = map.get(ns) ?? { dags: 0, models: 0 };
      if (n.type === "dag") cur.dags++;
      else if (n.type === "dbt_model") cur.models++;
      map.set(ns, cur);
    }
    return Array.from(map.entries()).sort((a, b) => b[1].dags + b[1].models - a[1].dags - a[1].models);
  }, [indexes]);

  const layers = useMemo(() => {
    const out: Array<[string, number]> = [];
    for (const [layer, nodes] of indexes.modelsByLayer.entries()) {
      out.push([layer, nodes.length]);
    }
    return out.sort((a, b) => b[1] - a[1]);
  }, [indexes]);

  return (
    <div className="space-y-6">
      {/* Top-line counts */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <CountCard label={t("atlas.lineage.cards.nodes")} value={stats.nodes} />
        <CountCard label={t("atlas.lineage.cards.edges")} value={stats.edges} />
        <CountCard label={t("atlas.lineage.cards.columnEdges")} value={stats.columnEdges} />
        <CountCard
          label={t("atlas.lineage.cards.modelsCovered")}
          value={`${stats.modelsCovered}/${stats.modelsTotal || "?"}`}
        />
      </div>

      {/* Issue cards */}
      <div>
        <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-2 uppercase tracking-wider">
          {t("atlas.lineage.issuesHeader")}
        </h2>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
          <button onClick={() => onNavigate("issues", { kind: undefined })} className="text-left">
            <StatCard label={t("atlas.lineage.cards.errors")} value={stats.errors} icon={ICONS.error} color="red" />
          </button>
          <button onClick={() => onNavigate("issues", { kind: "orphan_tasks" })} className="text-left">
            <StatCard label={t("atlas.lineage.cards.orphanTasks")} value={stats.orphanTasks} icon={ICONS.orphan} color="amber" />
          </button>
          <button onClick={() => onNavigate("issues", { kind: "isolated_models" })} className="text-left">
            <StatCard label={t("atlas.lineage.cards.isolatedModels")} value={stats.isolatedModels} icon={ICONS.isolate} color="amber" />
          </button>
          <button onClick={() => onNavigate("issues", { kind: "unused_sources" })} className="text-left">
            <StatCard label={t("atlas.lineage.cards.unusedSources")} value={stats.unusedSources} icon={ICONS.source} color="sky" />
          </button>
          <button onClick={() => onNavigate("issues", { kind: "unused_macros" })} className="text-left">
            <StatCard label={t("atlas.lineage.cards.unusedMacros")} value={stats.unusedMacros} icon={ICONS.macro} color="purple" />
          </button>
        </div>
      </div>

      {/* Domains */}
      {domains.length > 0 && (
        <div>
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-2 uppercase tracking-wider">
            {t("atlas.lineage.domainsHeader")}
          </h2>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
            {domains.map(([ns, s]) => (
              <button
                key={ns}
                onClick={() => onNavigate("graph", { focus: `ns:${ns}` })}
                className="text-left bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] p-4 hover:-translate-y-0.5 hover:shadow-[var(--shadow-md)] transition-all"
              >
                <div className="text-xs uppercase tracking-wider text-[var(--text-muted)]">{t("atlas.lineage.namespace")}</div>
                <div className="text-lg font-bold mt-0.5" style={{ fontFamily: "var(--font-display)" }}>{ns}</div>
                <div className="text-xs text-[var(--text-secondary)] mt-1 flex gap-3">
                  <span>{s.dags} DAGs</span>
                  <span>{s.models} models</span>
                </div>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Layers */}
      {layers.length > 0 && (
        <div>
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-2 uppercase tracking-wider">
            {t("atlas.lineage.layersHeader")}
          </h2>
          <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
            {layers.map(([layer, count]) => (
              <div
                key={layer}
                className="bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] p-4"
              >
                <div className={`text-xs uppercase tracking-wider font-semibold ${LAYER_COLORS[layer] ?? "text-[var(--text-muted)]"}`}>
                  {layer}
                </div>
                <div className="text-xl font-bold mt-1" style={{ fontFamily: "var(--font-display)" }}>{count}</div>
                <div className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">
                  {NODE_TYPE_LABELS.dbt_model.toLowerCase()}s
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function CountCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] p-4">
      <p className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider font-medium">{label}</p>
      <p className="text-2xl font-bold mt-0.5" style={{ fontFamily: "var(--font-display)" }}>{value}</p>
    </div>
  );
}
