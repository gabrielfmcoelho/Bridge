"use client";

import { useLocale } from "@/contexts/LocaleContext";
import type { TableRecord } from "@/lib/atlas/types";
import Card from "@/components/ui/Card";
import LayerBadge, { getLayerAccent } from "../shared/LayerBadge";
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
  const fq = qualifiedName(table);

  return (
    <Card
      as="button"
      onClick={onClick}
      selected={selected}
      accent={getLayerAccent(table.layer)}
      decorator="stripe-top"
      padding="none"
      className="flex flex-col overflow-hidden"
    >
      <div className="flex flex-col gap-2 p-3">
        <div className="flex items-start gap-2 justify-between">
          <div className="flex flex-col gap-0.5 min-w-0 flex-1">
            <span className="text-[var(--text-primary)] font-mono text-sm truncate" title={fq}>
              {table.node.label}
            </span>
            {fq && fq !== table.node.label && (
              <span className="text-2xs text-[var(--text-faint)] font-mono truncate" title={fq}>
                {fq}
              </span>
            )}
          </div>
          {table.hasWarning && (
            <span
              className="w-1.5 h-1.5 rounded-full bg-[var(--danger)] shrink-0 mt-1.5"
              aria-label={t("atlas.catalog.hasWarnings")}
              title={t("atlas.catalog.hasWarnings")}
            />
          )}
        </div>

        <div className="flex items-center gap-1.5 flex-wrap">
          <LayerBadge layer={table.layer} size="sm" />
          <RoleBadge role={table.role} size="sm" />
          <span className="ml-auto text-2xs text-[var(--text-muted)] tabular-nums">
            <span className="text-[var(--text-primary)] font-semibold">{table.columnCount}</span>{" "}
            {t("atlas.catalog.list.columnsLabel")}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          <span className="text-3xs uppercase tracking-[0.12em] text-[var(--text-faint)]">
            {table.namespace}
          </span>
        </div>
      </div>
    </Card>
  );
}

export function qualifiedName(table: TableRecord): string {
  const d = table.node.data as Record<string, unknown> | undefined;
  const parts = [d?.catalog, d?.schema, d?.table ?? table.node.label]
    .filter(p => p !== undefined && p !== "")
    .map(p => String(p));
  return parts.join(".");
}
