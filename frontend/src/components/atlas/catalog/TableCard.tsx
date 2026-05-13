"use client";

import { useLocale } from "@/contexts/LocaleContext";
import type { TableRecord } from "@/lib/atlas/types";
import LayerBadge, { getLayerStyle } from "../shared/LayerBadge";
import RoleBadge from "../shared/RoleBadge";

interface Props {
  table: TableRecord;
  selected?: boolean;
  onClick: () => void;
}

/**
 * Catalog card. Vertical layout — layer color stripe at the top, qualified name
 * in mono with namespace + role secondary line, and metric chips at the bottom.
 */
export default function TableCard({ table, selected, onClick }: Props) {
  const { t } = useLocale();
  const s = getLayerStyle(table.layer);
  const fq = qualifiedName(table);

  return (
    <button
      onClick={onClick}
      className={`group relative w-full text-left flex flex-col overflow-hidden rounded-[var(--radius-md)] border bg-[var(--bg-surface)] transition-all duration-150 ${
        selected
          ? "border-[var(--accent)] ring-2 ring-[var(--accent)]/30 -translate-y-0.5 shadow-[var(--shadow-md)]"
          : "border-[var(--border-subtle)] hover:border-[var(--border-default)] hover:-translate-y-0.5 hover:shadow-[var(--shadow-md)]"
      }`}
    >
      {/* Layer color stripe */}
      <span className={`h-[3px] w-full ${s.dot.replace("bg-", "bg-")}`} />

      <div className="flex flex-col gap-2 p-3">
        <div className="flex items-start gap-2 justify-between">
          <div className="flex flex-col gap-0.5 min-w-0 flex-1">
            <span className="text-[var(--text-primary)] font-mono text-[13px] truncate" title={fq}>
              {table.node.label}
            </span>
            {fq && fq !== table.node.label && (
              <span className="text-[10px] text-[var(--text-faint)] font-mono truncate" title={fq}>
                {fq}
              </span>
            )}
          </div>
          {table.hasWarning && (
            <span
              className="w-1.5 h-1.5 rounded-full bg-red-500 shrink-0 mt-1.5"
              aria-label="Has warnings"
              title="Has warnings"
            />
          )}
        </div>

        <div className="flex items-center gap-1.5 flex-wrap">
          <LayerBadge layer={table.layer} size="sm" />
          <RoleBadge role={table.role} size="sm" />
          <span className="ml-auto text-[10px] text-[var(--text-muted)] tabular-nums">
            <span className="text-[var(--text-primary)] font-semibold">{table.columnCount}</span>{" "}
            {t("atlas.catalog.list.columnsLabel")}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          <span className="text-[9px] uppercase tracking-[0.12em] text-[var(--text-faint)]">
            {table.namespace}
          </span>
        </div>
      </div>
    </button>
  );
}

export function qualifiedName(table: TableRecord): string {
  const d = table.node.data as Record<string, unknown> | undefined;
  const parts = [d?.catalog, d?.schema, d?.table ?? table.node.label]
    .filter(p => p !== undefined && p !== "")
    .map(p => String(p));
  return parts.join(".");
}
