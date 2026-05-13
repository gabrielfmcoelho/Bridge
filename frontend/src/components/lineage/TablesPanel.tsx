"use client";

import { useMemo, useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { LineageIndexes } from "@/lib/lineage/indexes";
import { buildTableMatrix, TABLE_LAYERS, type TableLayer } from "@/lib/lineage/indexes";
import type { LineageNode } from "@/lib/lineage/types";
import { LAYER_COLORS } from "@/lib/lineage/style";
import DetailDrawer from "./DetailDrawer";

interface Props {
  indexes: LineageIndexes;
  onOpenInGraph: (id: string) => void;
}

export default function TablesPanel({ indexes, onOpenInGraph }: Props) {
  const { t } = useLocale();
  const { matrix, namespaces } = useMemo(() => buildTableMatrix(indexes), [indexes]);
  const [namespaceFilter, setNamespaceFilter] = useState<string>("all");
  const [layerFilter, setLayerFilter] = useState<string>("all");
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const visibleNamespaces = namespaceFilter === "all"
    ? namespaces
    : namespaces.filter(n => n === namespaceFilter);

  const visibleLayers = layerFilter === "all"
    ? TABLE_LAYERS
    : (TABLE_LAYERS.filter(l => l === layerFilter) as readonly TableLayer[]);

  const totalCount = useMemo(() => {
    let total = 0;
    for (const ns of visibleNamespaces) {
      for (const layer of visibleLayers) {
        total += (matrix.get(ns)?.get(layer) ?? []).length;
      }
    }
    return total;
  }, [visibleNamespaces, visibleLayers, matrix]);

  const q = query.trim().toLowerCase();

  function cellTables(ns: string, layer: TableLayer): LineageNode[] {
    const all = matrix.get(ns)?.get(layer) ?? [];
    if (!q) return all;
    return all.filter(tb =>
      tb.label.toLowerCase().includes(q) ||
      tb.id.toLowerCase().includes(q)
    );
  }

  const selectedNode = selectedId ? indexes.nodesById.get(selectedId) ?? null : null;

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-3 px-1 text-xs">
        <Select
          label={t("atlas.lineage.toolbar.namespace")}
          value={namespaceFilter}
          onChange={setNamespaceFilter}
          options={[
            { value: "all", label: t("common.all") },
            ...namespaces.map(n => ({ value: n, label: n })),
          ]}
        />
        <Select
          label={t("atlas.lineage.toolbar.layer")}
          value={layerFilter}
          onChange={setLayerFilter}
          options={[
            { value: "all", label: t("common.all") },
            ...TABLE_LAYERS.map(l => ({ value: l, label: layerLabel(l, t) })),
          ]}
        />
        <input
          type="text"
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder={t("atlas.lineage.tables.searchPlaceholder")}
          className="flex-1 max-w-xs bg-[var(--bg-base)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2 py-1 text-xs text-[var(--text-primary)] focus:outline-none focus:border-[var(--border-default)]"
        />
        <span className="ml-auto text-[var(--text-muted)]">
          {totalCount} {t("atlas.lineage.tables.tablesLabel")}
        </span>
      </div>

      {/* Grid */}
      <div className="overflow-x-auto">
        <div
          className="inline-grid gap-2 min-w-full"
          style={{
            gridTemplateColumns: `minmax(140px, max-content) repeat(${visibleLayers.length}, minmax(180px, 1fr))`,
          }}
        >
          {/* Header row */}
          <div />
          {visibleLayers.map(layer => (
            <div
              key={layer}
              className={`text-[10px] uppercase tracking-widest font-semibold px-2 py-1.5 text-center rounded-[var(--radius-sm)] ${LAYER_COLORS[layer] ?? "text-[var(--text-muted)]"}`}
              style={{ background: "var(--bg-surface)" }}
            >
              {layerLabel(layer, t)}
            </div>
          ))}

          {/* Rows */}
          {visibleNamespaces.map(ns => {
            const rowTotal = visibleLayers.reduce(
              (acc, l) => acc + cellTables(ns, l).length, 0,
            );
            return (
              <Row key={ns} ns={ns} total={rowTotal}>
                {visibleLayers.map(layer => {
                  const tables = cellTables(ns, layer);
                  return (
                    <Cell key={layer} layer={layer} count={tables.length}>
                      {tables.length === 0 ? (
                        <div className="text-[10px] text-[var(--text-faint)] italic text-center py-2">
                          —
                        </div>
                      ) : (
                        tables.map(tb => (
                          <TableCard
                            key={tb.id}
                            table={tb}
                            indexes={indexes}
                            onClick={() => setSelectedId(tb.id)}
                          />
                        ))
                      )}
                    </Cell>
                  );
                })}
              </Row>
            );
          })}
        </div>
      </div>

      <DetailDrawer
        node={selectedNode}
        indexes={indexes}
        onClose={() => setSelectedId(null)}
        onSelect={(id) => setSelectedId(id)}
        onOpenInGraph={(id) => { setSelectedId(null); onOpenInGraph(id); }}
      />
    </div>
  );
}

function Row({ ns, total, children }: { ns: string; total: number; children: React.ReactNode }) {
  return (
    <>
      <div className="flex flex-col justify-start sticky left-0 z-10 px-2 py-2 bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)]">
        <div className="text-sm font-semibold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>
          {ns}
        </div>
        <div className="text-[10px] text-[var(--text-muted)]">{total} tables</div>
      </div>
      {children}
    </>
  );
}

function Cell({ children, count }: { children: React.ReactNode; count: number; layer: TableLayer }) {
  return (
    <div
      className="bg-[var(--bg-base)] border border-dashed border-[var(--border-subtle)] rounded-[var(--radius-sm)] p-1.5 space-y-1 max-h-[420px] overflow-y-auto"
      style={{ minHeight: count > 0 ? undefined : 60 }}
    >
      {children}
    </div>
  );
}

function TableCard({
  table, indexes, onClick,
}: { table: LineageNode; indexes: LineageIndexes; onClick: () => void }) {
  const colCount = (indexes.childrenOf.get(table.id) ?? [])
    .filter(id => indexes.nodesById.get(id)?.type === "column").length;
  const hasWarning = (indexes.warningsByNode.get(table.id) ?? 0) > 0;
  const fq = `${table.data?.catalog ?? ""}.${table.data?.schema ?? ""}`;
  return (
    <button
      onClick={onClick}
      className="w-full text-left bg-[var(--bg-surface)] hover:bg-[var(--bg-elevated)] border border-[var(--border-subtle)] hover:border-[var(--border-default)] rounded-[var(--radius-sm)] px-2 py-1.5 transition-colors"
    >
      <div className="flex items-center gap-1.5">
        <span className="text-xs font-mono text-[var(--text-primary)] truncate flex-1" title={table.id}>
          {table.label}
        </span>
        {hasWarning && <span className="w-1.5 h-1.5 rounded-full bg-red-500 shrink-0" />}
        {colCount > 0 && (
          <span className="text-[9px] px-1 py-0.5 rounded-full bg-[var(--bg-elevated)] text-[var(--text-muted)] shrink-0">
            {colCount}
          </span>
        )}
      </div>
      {fq !== "." && (
        <div className="text-[9px] text-[var(--text-faint)] font-mono truncate">{fq}</div>
      )}
    </button>
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

function layerLabel(layer: TableLayer, t: (k: string) => string): string {
  if (layer === "source") return t("atlas.lineage.tables.layer.source");
  if (layer === "other") return t("atlas.lineage.tables.layer.other");
  return layer;
}
