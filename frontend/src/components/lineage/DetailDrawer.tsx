"use client";

import { useMemo } from "react";
import Drawer from "@/components/ui/Drawer";
import { useLocale } from "@/contexts/LocaleContext";
import type { LineageIndexes } from "@/lib/lineage/indexes";
import { neighborsIn, neighborsOut } from "@/lib/lineage/indexes";
import type { LineageNode, LineageEdge } from "@/lib/lineage/types";
import { NODE_COLORS, NODE_TYPE_LABELS, LAYER_COLORS } from "@/lib/lineage/style";

interface Props {
  node: LineageNode | null;
  indexes: LineageIndexes;
  onClose: () => void;
  onSelect: (id: string) => void;
  /** Optional: when provided, the drawer shows a "Show in graph" footer
   *  that pivots the page to the Graph tab with this node focused. */
  onOpenInGraph?: (id: string) => void;
}

export default function DetailDrawer({ node, indexes, onClose, onSelect, onOpenInGraph }: Props) {
  const { t } = useLocale();
  const data = useMemo(() => {
    if (!node) return null;
    const upstream = neighborsIn(indexes, node.id, ["ref", "uses_source", "reads", "executes", "invokes", "triggers"]);
    const downstream = neighborsOut(indexes, node.id, ["ref", "uses_source", "writes", "executes", "invokes", "triggers"]);
    const children = (indexes.childrenOf.get(node.id) ?? [])
      .map(id => indexes.nodesById.get(id))
      .filter((n): n is LineageNode => Boolean(n));
    const columns = children.filter(n => n.type === "column");
    const warnings = (indexes.raw.warnings ?? []).filter(w => w.source_id === node.id);
    return { upstream, downstream, columns, children, warnings };
  }, [node, indexes]);

  if (!node || !data) return null;

  const c = NODE_COLORS[node.type] ?? NODE_COLORS.table;

  return (
    <Drawer
      open={!!node}
      onClose={onClose}
      title={node.label}
      subHeader={
        <div className="flex items-center gap-2 flex-wrap">
          <span className={`text-[10px] px-2 py-0.5 rounded-full ${c.bg} ${c.text} ${c.border} border font-semibold uppercase tracking-wider`}>
            {NODE_TYPE_LABELS[node.type] ?? node.type}
          </span>
          {node.layer && (
            <span className={`text-[10px] uppercase tracking-wider font-semibold ${LAYER_COLORS[node.layer] ?? "text-[var(--text-muted)]"}`}>
              {node.layer}
            </span>
          )}
          {node.namespace && (
            <span className="text-[10px] text-[var(--text-muted)]">📁 {node.namespace}</span>
          )}
          {node.file && (
            <span className="text-[10px] text-[var(--text-muted)] font-mono">
              {node.file}{node.lines?.[0] ? `:${node.lines[0]}` : ""}
            </span>
          )}
        </div>
      }
      wide
    >
      {/* ID */}
      <div className="mb-4">
        <Label>{t("atlas.lineage.drawer.id")}</Label>
        <code className="text-xs text-[var(--text-secondary)] break-all">{node.id}</code>
      </div>

      {/* Description */}
      {node.description && (
        <div className="mb-4">
          <Label>{t("atlas.lineage.drawer.description")}</Label>
          <p className="text-sm text-[var(--text-primary)]">{node.description}</p>
        </div>
      )}

      {/* Data fields */}
      {node.data && Object.keys(node.data).length > 0 && (
        <div className="mb-4">
          <Label>{t("atlas.lineage.drawer.metadata")}</Label>
          <div className="space-y-1 mt-1">
            {Object.entries(node.data).map(([k, v]) => (
              <div key={k} className="flex gap-2 text-xs">
                <span className="text-[var(--text-muted)] min-w-[120px]">{k}</span>
                <span className="text-[var(--text-primary)] font-mono break-all">{renderValue(v)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Warnings */}
      {data.warnings.length > 0 && (
        <div className="mb-4">
          <Label>{t("atlas.lineage.drawer.warnings")}</Label>
          <div className="space-y-2 mt-1">
            {data.warnings.map((w, i) => (
              <div key={i} className={`p-2 rounded-[var(--radius-sm)] border text-xs ${
                w.severity === "error"
                  ? "bg-red-500/10 border-red-500/30 text-red-300"
                  : "bg-amber-500/10 border-amber-500/30 text-amber-300"
              }`}>
                <div className="font-mono text-[10px] uppercase opacity-80">{w.kind}</div>
                <div className="mt-0.5">{w.message}</div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Upstream */}
      <NeighborList
        title={t("atlas.lineage.drawer.upstream")}
        edges={data.upstream}
        side="source"
        indexes={indexes}
        onSelect={onSelect}
        t={t}
      />

      {/* Downstream */}
      <NeighborList
        title={t("atlas.lineage.drawer.downstream")}
        edges={data.downstream}
        side="target"
        indexes={indexes}
        onSelect={onSelect}
        t={t}
      />

      {/* Columns + per-column lineage (the part the column lineage data unlocks) */}
      {data.columns.length > 0 && (
        <div className="mb-4">
          <Label>{t("atlas.lineage.drawer.columns")} ({data.columns.length})</Label>
          <div className="space-y-1.5 mt-1">
            {data.columns.map(col => (
              <ColumnRow key={col.id} column={col} indexes={indexes} onSelect={onSelect} t={t} />
            ))}
          </div>
        </div>
      )}

      {/* "Show in graph" CTA — only when caller provided the handler. */}
      {onOpenInGraph && (
        <div className="pt-3 mt-4 border-t border-[var(--border-subtle)]">
          <button
            onClick={() => onOpenInGraph(node.id)}
            className="w-full flex items-center justify-center gap-1.5 px-3 py-2 rounded-[var(--radius-md)] bg-cyan-500/15 hover:bg-cyan-500/25 border border-cyan-500/40 text-cyan-300 text-sm font-medium transition-colors"
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M14 5l7 7m0 0l-7 7m7-7H3" />
            </svg>
            {t("atlas.lineage.drawer.openInGraph")}
          </button>
        </div>
      )}
    </Drawer>
  );
}

function ColumnRow({ column, indexes, onSelect, t }: {
  column: LineageNode;
  indexes: LineageIndexes;
  onSelect: (id: string) => void;
  t: (k: string) => string;
}) {
  const upstream = neighborsIn(indexes, column.id, ["column_lineage"]);
  const downstream = neighborsOut(indexes, column.id, ["column_lineage"]);

  return (
    <div className="bg-[var(--bg-base)] rounded-[var(--radius-sm)] border border-[var(--border-subtle)] p-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-mono text-[var(--text-primary)]">{column.label}</span>
        <span className="text-[9px] text-[var(--text-muted)]">
          ↑{upstream.length} ↓{downstream.length}
        </span>
      </div>
      {upstream.length > 0 && (
        <div className="mt-1.5 pl-2 border-l border-[var(--border-subtle)] space-y-0.5">
          {upstream.slice(0, 6).map((e, i) => {
            const src = indexes.nodesById.get(e.source);
            return (
              <button
                key={i}
                onClick={() => onSelect(e.source)}
                className="block w-full text-left text-[10px] text-[var(--text-secondary)] hover:text-[var(--text-primary)] truncate"
                title={e.source}
              >
                <ConfidenceDot c={e.confidence as string | undefined} />
                <span className="font-mono">{src?.label ?? e.source}</span>
                {Boolean(e.data?.via) && <span className="text-[var(--text-faint)] ml-1">[{String(e.data?.via)}]</span>}
              </button>
            );
          })}
          {upstream.length > 6 && (
            <span className="text-[9px] text-[var(--text-muted)]">+{upstream.length - 6} {t("common.more")}</span>
          )}
        </div>
      )}
    </div>
  );
}

function NeighborList({ title, edges, side, indexes, onSelect, t }: {
  title: string;
  edges: LineageEdge[];
  side: "source" | "target";
  indexes: LineageIndexes;
  onSelect: (id: string) => void;
  t: (k: string) => string;
}) {
  if (edges.length === 0) return null;
  return (
    <div className="mb-4">
      <Label>{title} ({edges.length})</Label>
      <div className="space-y-1 mt-1">
        {edges.slice(0, 30).map((e, i) => {
          const otherId = side === "source" ? e.source : e.target;
          const other = indexes.nodesById.get(otherId);
          return (
            <button
              key={i}
              onClick={() => onSelect(otherId)}
              className="w-full flex items-center gap-2 text-xs text-left hover:bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] px-2 py-1 transition-colors"
            >
              <span className="text-[9px] uppercase tracking-wider text-[var(--text-faint)] font-mono w-20 shrink-0">{e.kind}</span>
              <span className="text-[var(--text-primary)] font-mono truncate flex-1">{other?.label ?? otherId}</span>
              {other?.type && (
                <span className="text-[9px] text-[var(--text-muted)] shrink-0">{NODE_TYPE_LABELS[other.type] ?? other.type}</span>
              )}
            </button>
          );
        })}
        {edges.length > 30 && (
          <span className="text-[10px] text-[var(--text-muted)] ml-2">+{edges.length - 30} {t("common.more")}</span>
        )}
      </div>
    </div>
  );
}

function ConfidenceDot({ c }: { c?: string }) {
  const color =
    c === "exact" ? "bg-emerald-400" :
    c === "inferred" ? "bg-amber-400" :
    c === "dynamic" ? "bg-rose-400" : "bg-[var(--border-strong)]";
  return <span className={`inline-block w-1.5 h-1.5 rounded-full mr-1.5 align-middle ${color}`} title={c ?? ""} />;
}

function Label({ children }: { children: React.ReactNode }) {
  return <div className="text-[10px] text-[var(--text-faint)] uppercase tracking-wider font-semibold">{children}</div>;
}

function renderValue(v: unknown): string {
  if (v === null || v === undefined) return "—";
  if (Array.isArray(v)) return v.slice(0, 8).map(renderValue).join(", ") + (v.length > 8 ? `, +${v.length - 8}` : "");
  if (typeof v === "object") return JSON.stringify(v).slice(0, 140);
  return String(v);
}
