"use client";

import { useEffect, useState } from "react";
import PageShell from "@/components/layout/PageShell";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import Drawer from "@/components/ui/Drawer";
import SearchOmnibar from "@/components/lineage/SearchOmnibar";
import { useLocale } from "@/contexts/LocaleContext";
import { useAtlasIndexes } from "@/lib/atlas/useAtlasIndexes";
import { useAtlasParams } from "@/lib/atlas/useAtlasParams";
import { PIPELINE_VIEW_MODES, type PipelineViewMode } from "@/lib/atlas/types";
import AtlasToolbar from "../shared/AtlasToolbar";
import ViewModeToggle from "../shared/ViewModeToggle";
import SwimlanesGraph from "./modes/SwimlanesGraph";
import NestedGroupsGraph from "./modes/NestedGroupsGraph";
import PipelineNodeDetail from "./PipelineNodeDetail";
import ColumnLineagePanel from "../catalog/ColumnLineagePanel";

const MODE_ICONS: Record<PipelineViewMode, React.ReactNode> = {
  groups: (
    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 4h6v6H3V4zm0 10h6v6H3v-6zm10-10h8v6h-8V4zm0 10h8v6h-8v-6z" />
    </svg>
  ),
  lanes: (
    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M4 4v16M10 4v16M16 4v16M22 4v16" />
    </svg>
  ),
};

export default function PipelinePageInner() {
  const { t } = useLocale();
  const { indexes, isLoading, error, generatedAt } = useAtlasIndexes();
  const { view, setView, filters, setFilters, selected, setSelected, col, setCol } =
    useAtlasParams<PipelineViewMode>("lanes");

  const [searchOpen, setSearchOpen] = useState(false);

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

  const viewOptions = PIPELINE_VIEW_MODES.map(m => ({
    value: m,
    label: t(`atlas.pipeline.modes.${m}`),
    icon: MODE_ICONS[m],
  }));

  function handleSearchPick(id: string) {
    setSearchOpen(false);
    if (!indexes) return;
    const node = indexes.nodesById.get(id);
    if (!node) return;
    if (node.type === "column" && node.parent) {
      setSelected(node.parent);
      setTimeout(() => setCol(id), 0);
    } else {
      setSelected(id);
    }
  }

  const drawerOpen = Boolean(selected);
  const selectedNode = selected && indexes ? indexes.nodesById.get(selected) : null;

  return (
    <PageShell fullBleed>
      <div className="flex flex-col h-full">
        {/* Header / toolbar */}
        <div className="px-4 md:px-6 pt-4 pb-3 border-b border-[var(--border-subtle)] flex flex-col gap-3">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-2xl font-bold tracking-tight" style={{ fontFamily: "var(--font-display)" }}>
                {t("atlas.pipeline.title")}
              </h1>
              {generatedAt && (
                <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)]">
                  {t("atlas.lineage.generatedAt")} · {new Date(generatedAt).toLocaleString()}
                </span>
              )}
            </div>
            <p className="text-sm text-[var(--text-muted)] max-w-2xl">{t("atlas.pipeline.subtitle")}</p>
          </div>
          {indexes && (
            <AtlasToolbar
              indexes={indexes}
              filters={filters}
              onChange={setFilters}
              showRoleFilter={false}
              onOpenSearch={() => setSearchOpen(true)}
              rightSlot={
                <ViewModeToggle<PipelineViewMode>
                  value={view}
                  onChange={setView}
                  options={viewOptions}
                  ariaLabel={t("atlas.pipeline.toolbar.view")}
                />
              }
            />
          )}
        </div>

        {/* Graph */}
        <div className="flex-1 min-h-0 p-4 md:p-6">
          {isLoading ? (
            <Skeleton className="w-full h-full rounded-[var(--radius-lg)]" />
          ) : error ? (
            <EmptyState icon="server" title="Error" description={error instanceof Error ? error.message : String(error)} />
          ) : !indexes ? (
            <EmptyState icon="server" title={t("common.noResults")} description={t("atlas.lineage.notGenerated")} />
          ) : view === "lanes" ? (
            <SwimlanesGraph indexes={indexes} filters={filters} selectedId={selected} onSelect={setSelected} />
          ) : (
            <NestedGroupsGraph indexes={indexes} filters={filters} selectedId={selected} onSelect={setSelected} />
          )}
        </div>
      </div>

      {/* Drawer */}
      <Drawer
        open={drawerOpen}
        onClose={() => setSelected(null)}
        title={col ? "" : selectedNode?.label}
        wide
      >
        {indexes && selected && !col && (
          <PipelineNodeDetail
            indexes={indexes}
            nodeId={selected}
            onSelectNode={setSelected}
            onSelectColumn={setCol}
          />
        )}
        {indexes && selected && col && (
          <ColumnLineagePanel
            indexes={indexes}
            columnId={col}
            onBack={() => setCol(null)}
            onSelectColumn={(id) => {
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
