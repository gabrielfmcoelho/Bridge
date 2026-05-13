"use client";

import { useLocale } from "@/contexts/LocaleContext";
import EmptyState from "@/components/ui/EmptyState";
import type { TableRecord } from "@/lib/atlas/types";
import TableCard from "../TableCard";

interface Props {
  tables: TableRecord[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

/**
 * Mode 2: flat card grid. Tables are grouped by namespace as section headers
 * — keeps the "by domain" first principle even when there is no tree on the left.
 */
export default function CardGridView({ tables, selectedId, onSelect }: Props) {
  const { t } = useLocale();

  if (tables.length === 0) {
    return (
      <EmptyState
        icon="server"
        title={t("atlas.catalog.empty.title")}
        description={t("atlas.catalog.empty.description")}
      />
    );
  }

  // Group by namespace (preserving the order they appear in the indexes).
  const grouped = new Map<string, TableRecord[]>();
  for (const tb of tables) {
    const arr = grouped.get(tb.namespace) ?? [];
    arr.push(tb);
    grouped.set(tb.namespace, arr);
  }

  return (
    <div className="flex flex-col gap-6">
      {Array.from(grouped.entries()).map(([ns, items]) => (
        <section key={ns} className="flex flex-col gap-3">
          <div className="flex items-baseline gap-3 px-1">
            <h2
              className="text-sm font-semibold uppercase tracking-[0.16em] text-[var(--text-primary)]"
              style={{ fontFamily: "var(--font-display)" }}
            >
              {ns}
            </h2>
            <span className="text-[10px] tabular-nums text-[var(--text-faint)]">
              {items.length} {t("atlas.catalog.list.tablesLabel")}
            </span>
            <span className="flex-1 h-px bg-[var(--border-subtle)]" />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
            {items.map(tb => (
              <TableCard
                key={tb.node.id}
                table={tb}
                selected={tb.node.id === selectedId}
                onClick={() => onSelect(tb.node.id)}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
