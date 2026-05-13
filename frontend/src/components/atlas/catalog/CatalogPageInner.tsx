"use client";

import { useEffect, useMemo, useState } from "react";
import PageShell from "@/components/layout/PageShell";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import Drawer from "@/components/ui/Drawer";
import SearchOmnibar from "@/components/lineage/SearchOmnibar";
import { useLocale } from "@/contexts/LocaleContext";
import { useAtlasIndexes } from "@/lib/atlas/useAtlasIndexes";
import { useAtlasParams } from "@/lib/atlas/useAtlasParams";
import { CATALOG_VIEW_MODES, type CatalogViewMode, type TableRecord } from "@/lib/atlas/types";
import AtlasToolbar from "../shared/AtlasToolbar";
import ViewModeToggle from "../shared/ViewModeToggle";
import CardGridView from "./modes/CardGridView";
import TreeListView from "./modes/TreeListView";
import SplitView from "./modes/SplitView";
import TableDetailPanel from "./TableDetailPanel";
import ColumnLineagePanel from "./ColumnLineagePanel";
import LayerBadge from "../shared/LayerBadge";

const MODE_ICONS: Record<CatalogViewMode, React.ReactNode> = {
  tree: (
    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h8m-8 5h6m-6 5h7M14 7h6M14 11h6M14 15h6" />
    </svg>
  ),
  cards: (
    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zm10 0a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zm10 0a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
    </svg>
  ),
  split: (
    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 4h16v16H4V4zm6 0v16M14 4v16" />
    </svg>
  ),
};

export default function CatalogPageInner() {
  const { t } = useLocale();
  const { indexes, isLoading, error, generatedAt } = useAtlasIndexes();
  const { view, setView, filters, setFilters, selected, setSelected, col, setCol } =
    useAtlasParams<CatalogViewMode>("cards");

  const [searchOpen, setSearchOpen] = useState(false);

  // Cmd-K → open search
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setSearchOpen(true);
      }
      if (e.key === "Escape") setSearchOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const filteredTables = useMemo<TableRecord[]>(() => {
    if (!indexes) return [];
    let out = indexes.tables;
    if (filters.domains.length > 0) out = out.filter(t => filters.domains.includes(t.namespace));
    if (filters.layers.length > 0) out = out.filter(t => filters.layers.includes(t.layer));
    if (filters.role !== "all") out = out.filter(t => t.role === filters.role);
    if (filters.query.trim()) {
      const q = filters.query.trim().toLowerCase();
      out = out.filter(t =>
        t.node.label.toLowerCase().includes(q) ||
        t.node.id.toLowerCase().includes(q) ||
        t.namespace.toLowerCase().includes(q)
      );
    }
    return out;
  }, [indexes, filters]);

  const viewOptions = useMemo(() => CATALOG_VIEW_MODES.map(m => ({
    value: m,
    label: t(`atlas.catalog.modes.${m}`),
    icon: MODE_ICONS[m],
  })), [t]);

  // When a node is picked from the omnibar that's a column, set both selected (the parent table) + col.
  function handleSearchPick(id: string) {
    setSearchOpen(false);
    if (!indexes) return;
    const node = indexes.nodesById.get(id);
    if (!node) return;
    if (node.type === "column" && node.parent) {
      setSelected(node.parent);
      setTimeout(() => setCol(id), 0);
    } else if (node.type === "table") {
      setSelected(id);
    } else {
      // For dag/task/model we don't have a panel yet — fall back to the lineage page.
      // (Future: dedicated panels per type.)
      setSelected(id);
    }
  }

  const selectedTableRec = selected && indexes ? indexes.tablesById.get(selected) : null;
  const drawerOpen = Boolean(selected);

  return (
    <PageShell>
      <div className="flex flex-col gap-5">
        {/* Header */}
        <header className="flex flex-col gap-1.5">
          <div className="flex items-center gap-3 flex-wrap">
            <h1 className="text-2xl font-bold tracking-tight" style={{ fontFamily: "var(--font-display)" }}>
              {t("atlas.catalog.title")}
            </h1>
            {generatedAt && (
              <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)]">
                {t("atlas.lineage.generatedAt")} · {new Date(generatedAt).toLocaleString()}
              </span>
            )}
          </div>
          <p className="text-sm text-[var(--text-muted)] max-w-2xl">{t("atlas.catalog.subtitle")}</p>
        </header>

        {/* Toolbar */}
        {indexes && (
          <AtlasToolbar
            indexes={indexes}
            filters={filters}
            onChange={setFilters}
            showRoleFilter
            onOpenSearch={() => setSearchOpen(true)}
            rightSlot={
              <ViewModeToggle<CatalogViewMode>
                value={view}
                onChange={setView}
                options={viewOptions}
                ariaLabel={t("atlas.catalog.toolbar.view")}
              />
            }
          />
        )}

        {/* Active filter readout */}
        {indexes && (
          <div className="flex items-center gap-2 text-[11px] text-[var(--text-muted)] px-1">
            <span className="tabular-nums text-[var(--text-secondary)] font-semibold">{filteredTables.length}</span>
            <span>of</span>
            <span className="tabular-nums">{indexes.tables.length}</span>
            <span>tables</span>
            {filters.layers.length > 0 && (
              <span className="flex items-center gap-1 ml-2">
                ·
                {filters.layers.map(l => <LayerBadge key={l} layer={l as never} size="sm" dot={false} />)}
              </span>
            )}
          </div>
        )}

        {/* Body */}
        <div>
          {isLoading ? (
            <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
          ) : error ? (
            <EmptyState icon="server" title="Error" description={error instanceof Error ? error.message : String(error)} />
          ) : !indexes ? (
            <EmptyState icon="server" title={t("common.noResults")} description={t("atlas.lineage.notGenerated")} />
          ) : view === "cards" ? (
            <CardGridView tables={filteredTables} selectedId={selected} onSelect={setSelected} />
          ) : view === "tree" ? (
            <TreeListView indexes={indexes} tables={filteredTables} selectedId={selected} onSelect={setSelected} filters={filters} onFiltersChange={setFilters} />
          ) : (
            <SplitView indexes={indexes} tables={filteredTables} selectedId={selected} onSelect={setSelected} filters={filters} onFiltersChange={setFilters} />
          )}
        </div>
      </div>

      {/* Drawer */}
      <Drawer
        open={drawerOpen}
        onClose={() => setSelected(null)}
        title={col ? "" : selectedTableRec?.node.label}
        wide
      >
        {indexes && selected && !col && (
          <TableDetailPanel
            indexes={indexes}
            tableId={selected}
            onSelectTable={setSelected}
            onSelectColumn={setCol}
          />
        )}
        {indexes && selected && col && (
          <ColumnLineagePanel
            indexes={indexes}
            columnId={col}
            onBack={() => setCol(null)}
            onSelectColumn={(id) => {
              // Navigate inside the trail: switch to that column's parent table + column.
              const node = indexes.nodesById.get(id);
              if (!node) return;
              if (node.parent && node.parent !== selected) {
                setSelected(node.parent);
                setTimeout(() => setCol(id), 0);
              } else {
                setCol(id);
              }
            }}
          />
        )}
      </Drawer>

      {/* Search omnibar */}
      {indexes && (
        <SearchOmnibar
          open={searchOpen}
          indexes={indexes}
          onClose={() => setSearchOpen(false)}
          onPick={handleSearchPick}
        />
      )}
    </PageShell>
  );
}
