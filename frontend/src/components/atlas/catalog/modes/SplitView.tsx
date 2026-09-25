"use client";

import { useLocale } from "@/contexts/LocaleContext";
import EmptyState from "@/components/ui/EmptyState";
import SectionCard from "@/components/ui/SectionCard";
import type { AtlasIndexes, AtlasFilters, TableRecord } from "@/lib/atlas/types";
import DomainTree from "../DomainTree";
import TableCard from "../TableCard";

interface Props {
  indexes: AtlasIndexes;
  tables: TableRecord[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  filters: AtlasFilters;
  onFiltersChange: (next: Partial<AtlasFilters>) => void;
}

/**
 * Densest of the three catalog modes: tree on the left for fast scoping,
 * card grid in the center for scanability. Detail still opens in the drawer.
 */
export default function SplitView({ indexes, tables, selectedId, onSelect, filters, onFiltersChange }: Props) {
  const { t } = useLocale();

  // Group tables by namespace so the grid keeps "by domain" structure even with the tree filtering.
  const grouped = new Map<string, TableRecord[]>();
  for (const tb of tables) {
    const arr = grouped.get(tb.namespace) ?? [];
    arr.push(tb);
    grouped.set(tb.namespace, arr);
  }

  return (
    <div className="grid gap-3 md:grid-cols-[260px_1fr]">
      <aside className="rounded-[var(--radius-lg)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 max-h-[70vh] overflow-y-auto sticky top-0 self-start">
        <DomainTree
          indexes={indexes}
          tables={tables}
          selectedId={selectedId}
          onSelect={onSelect}
          filters={filters}
          onFiltersChange={onFiltersChange}
          scaffoldOnly
        />
      </aside>

      <div className="min-w-0">
        {tables.length === 0 ? (
          <EmptyState
            icon="server"
            title={t("atlas.catalog.empty.title")}
            description={t("atlas.catalog.empty.description")}
          />
        ) : (
          <div className="flex flex-col gap-5">
            {Array.from(grouped.entries()).map(([ns, items]) => (
              <SectionCard key={ns} as="h3" variant="plain" title={ns} count={items.length}>
                <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-2.5">
                  {items.map(tb => (
                    <TableCard
                      key={tb.node.id}
                      table={tb}
                      selected={tb.node.id === selectedId}
                      onClick={() => onSelect(tb.node.id)}
                    />
                  ))}
                </div>
              </SectionCard>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
