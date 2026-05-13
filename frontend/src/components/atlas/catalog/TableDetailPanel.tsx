"use client";

import { useMemo } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { AtlasIndexes } from "@/lib/atlas/types";
import type { LineageNode } from "@/lib/lineage/types";
import { buildTableLineageChain, findDirectNeighbors, findTableProducers } from "@/lib/atlas/tableLineageChain";
import { neighborsIn, neighborsOut } from "@/lib/lineage/indexes";
import LineageChain from "../shared/LineageChain";
import LayerBadge from "../shared/LayerBadge";
import RoleBadge from "../shared/RoleBadge";
import { qualifiedName } from "./TableCard";

interface Props {
  indexes: AtlasIndexes;
  tableId: string;
  onSelectTable: (id: string) => void;
  onSelectColumn: (columnId: string) => void;
}

export default function TableDetailPanel({ indexes, tableId, onSelectTable, onSelectColumn }: Props) {
  const { t } = useLocale();
  const rec = indexes.tablesById.get(tableId);
  const fq = useMemo(() => rec ? qualifiedName(rec) : tableId, [rec, tableId]);

  const chain = useMemo(() => buildTableLineageChain(indexes, tableId), [indexes, tableId]);
  const neighbors = useMemo(() => findDirectNeighbors(indexes, tableId), [indexes, tableId]);
  const producers = useMemo(() => findTableProducers(indexes, tableId), [indexes, tableId]);

  const columns = useMemo<LineageNode[]>(() => {
    return (indexes.childrenOf.get(tableId) ?? [])
      .map(id => indexes.nodesById.get(id))
      .filter((n): n is LineageNode => n?.type === "column")
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [indexes, tableId]);

  if (!rec) {
    return <div className="text-sm text-[var(--text-muted)]">Table not found in the index.</div>;
  }

  const downstreamCount = neighbors.downstream.length;

  return (
    <div className="flex flex-col gap-5">
      {/* Header — qualified name, badges */}
      <header className="flex flex-col gap-2 -mt-1">
        <div className="flex items-center gap-1.5 flex-wrap">
          <LayerBadge layer={rec.layer} size="md" />
          <RoleBadge role={rec.role} size="md" />
          <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--text-faint)] ml-1">
            {rec.namespace}
          </span>
        </div>
        <p className="font-mono text-xs text-[var(--text-secondary)] break-all">{fq}</p>
      </header>

      {/* Stats row */}
      <div className="grid grid-cols-2 gap-2">
        <Stat label={t("atlas.catalog.detail.columnsCount")} value={rec.columnCount} />
        <Stat label={t("atlas.catalog.detail.downstreamCount")} value={downstreamCount} />
      </div>

      {/* Lineage chain */}
      <section className="flex flex-col gap-2">
        <SectionLabel>{t("atlas.catalog.detail.lineageChain")}</SectionLabel>
        <LineageChain steps={chain} focalTableId={tableId} onSelectTable={onSelectTable} />
      </section>

      {/* Producers (task / dbt model that materializes this table) */}
      {producers.length > 0 && (
        <section className="flex flex-col gap-2">
          <SectionLabel>{t("atlas.lineage.drawer.upstream")}</SectionLabel>
          <div className="flex flex-wrap gap-1.5">
            {producers.map(p => (
              <NodeChip key={p.id} node={p} />
            ))}
          </div>
        </section>
      )}

      {/* Built from */}
      <section className="flex flex-col gap-2">
        <SectionLabel>{t("atlas.catalog.detail.upstream")}</SectionLabel>
        {neighbors.upstream.length === 0 ? (
          <p className="text-xs text-[var(--text-muted)] italic">{t("atlas.catalog.detail.noUpstream")}</p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {neighbors.upstream.map(n => (
              <TableChip key={n.id} node={n} indexes={indexes} onClick={() => onSelectTable(n.id)} />
            ))}
          </div>
        )}
      </section>

      {/* Used by */}
      <section className="flex flex-col gap-2">
        <SectionLabel>{t("atlas.catalog.detail.downstream")}</SectionLabel>
        {neighbors.downstream.length === 0 ? (
          <p className="text-xs text-[var(--text-muted)] italic">{t("atlas.catalog.detail.noDownstream")}</p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {neighbors.downstream.map(n => (
              <TableChip key={n.id} node={n} indexes={indexes} onClick={() => onSelectTable(n.id)} />
            ))}
          </div>
        )}
      </section>

      {/* Columns */}
      <section className="flex flex-col gap-2">
        <SectionLabel>
          {t("atlas.catalog.detail.columnsSection")}{" "}
          <span className="text-[var(--text-muted)] tabular-nums normal-case tracking-normal">({columns.length})</span>
        </SectionLabel>
        <div className="flex flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--border-subtle)] overflow-hidden bg-[var(--bg-base)]">
          {columns.length === 0 ? (
            <p className="text-xs text-[var(--text-muted)] italic p-3">No columns indexed.</p>
          ) : (
            columns.map((col, i) => (
              <ColumnRow
                key={col.id}
                col={col}
                indexes={indexes}
                isLast={i === columns.length - 1}
                onClick={() => onSelectColumn(col.id)}
              />
            ))
          )}
        </div>
      </section>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] p-3 flex flex-col gap-1">
      <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--text-faint)] font-semibold">{label}</span>
      <span className="text-2xl text-[var(--text-primary)] font-semibold tabular-nums" style={{ fontFamily: "var(--font-display)" }}>
        {value}
      </span>
    </div>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)] font-semibold">
      {children}
    </span>
  );
}

function NodeChip({ node }: { node: LineageNode }) {
  const isTask = node.type === "task";
  const isModel = node.type === "dbt_model";
  const color = isModel ? "bg-emerald-500/10 text-emerald-300 border-emerald-500/30"
              : isTask ? "bg-purple-500/10 text-purple-300 border-purple-500/30"
              : "bg-[var(--bg-base)] text-[var(--text-muted)] border-[var(--border-subtle)]";
  return (
    <span className={`inline-flex items-center gap-1.5 text-[11px] font-mono px-2 py-1 rounded-[var(--radius-sm)] border ${color}`}>
      <span className="text-[9px] uppercase tracking-wider opacity-80">{node.type.replace("_", " ")}</span>
      <span>{node.label}</span>
    </span>
  );
}

function TableChip({ node, indexes, onClick }: { node: LineageNode; indexes: AtlasIndexes; onClick: () => void }) {
  const rec = indexes.tablesById.get(node.id);
  return (
    <button
      onClick={onClick}
      className="inline-flex items-center gap-1.5 text-[11px] font-mono px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-base)] hover:bg-[var(--bg-elevated)] hover:border-[var(--border-default)] transition-colors"
      title={node.id}
    >
      {rec && <span className={`w-1.5 h-1.5 rounded-full ${rec.layer === "source" ? "bg-blue-400" : rec.layer === "bronze" ? "bg-amber-500" : rec.layer === "silver" ? "bg-slate-300" : rec.layer === "gold" ? "bg-yellow-400" : "bg-gray-400"}`} />}
      <span>{node.label}</span>
    </button>
  );
}

function ColumnRow({
  col,
  indexes,
  isLast,
  onClick,
}: {
  col: LineageNode;
  indexes: AtlasIndexes;
  isLast: boolean;
  onClick: () => void;
}) {
  const upstreamCount = neighborsIn(indexes, col.id, ["column_lineage"]).length;
  const downstreamCount = neighborsOut(indexes, col.id, ["column_lineage"]).length;
  const dtype = (col.data as Record<string, unknown> | undefined)?.data_type;
  const inferred = Boolean((col.data as Record<string, unknown> | undefined)?.inferred_from_sql);

  return (
    <button
      onClick={onClick}
      className={`group flex items-center gap-3 w-full text-left px-3 py-2 hover:bg-[var(--bg-elevated)] transition-colors ${
        isLast ? "" : "border-b border-[var(--border-subtle)]"
      }`}
    >
      <span className="font-mono text-[12px] text-[var(--text-primary)] flex-1 truncate">
        {col.label}
      </span>
      {Boolean(dtype) && (
        <span className="text-[10px] font-mono text-[var(--text-muted)] truncate">{String(dtype)}</span>
      )}
      {inferred && (
        <span className="text-[9px] uppercase tracking-wider text-[var(--text-faint)]">inferred</span>
      )}
      <span className="text-[9px] tabular-nums text-[var(--text-muted)] shrink-0 inline-flex items-center gap-2">
        <span title="upstream column edges">↑ {upstreamCount}</span>
        <span title="downstream column edges">↓ {downstreamCount}</span>
      </span>
      <svg
        className="w-3 h-3 text-[var(--text-faint)] opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
      </svg>
    </button>
  );
}
