"use client";

import { useLocale } from "@/contexts/LocaleContext";
import EmptyState from "@/components/ui/EmptyState";
import type { AtlasIndexes, AtlasFilters, TableRecord } from "@/lib/atlas/types";
import DomainTree from "../DomainTree";
import TableRow from "../TableRow";

interface Props {
  indexes: AtlasIndexes;
  tables: TableRecord[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  filters: AtlasFilters;
  onFiltersChange: (next: Partial<AtlasFilters>) => void;
}

export default function TreeListView({ indexes, tables, selectedId, onSelect, filters, onFiltersChange }: Props) {
  const { t } = useLocale();

  return (
    <div className="grid gap-3 md:grid-cols-[280px_1fr]">
      <aside className="rounded-[var(--radius-lg)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 max-h-[70vh] overflow-y-auto sticky top-0 self-start">
        <DomainTree
          indexes={indexes}
          tables={tables}
          selectedId={selectedId}
          onSelect={onSelect}
          filters={filters}
          onFiltersChange={onFiltersChange}
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
          <div className="flex flex-col gap-1.5">
            {tables.map(tb => (
              <TableRow
                key={tb.node.id}
                table={tb}
                selected={tb.node.id === selectedId}
                onClick={() => onSelect(tb.node.id)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
