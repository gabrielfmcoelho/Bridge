"use client";

import { useState, useMemo, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { databasesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useOpenOnParam } from "@/hooks/useOpenOnParam";
import { useExportCSV } from "@/hooks/useExportCSV";
import { useInventoryFilters } from "@/hooks/useInventoryFilters";
import { usePageTab } from "@/hooks/usePageTab";
import { useRelationGroups } from "@/hooks/useRelationGroups";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import SearchBadge from "@/components/ui/SearchBadge";
import SectionHeading from "@/components/ui/SectionHeading";
import StatusAlert from "@/components/ui/StatusAlert";
import PageHeader from "@/components/ui/PageHeader";
import TrashButton from "@/components/inventory/TrashDrawer";
import InventoryOverview from "@/components/inventory/InventoryOverview";
import InsightKpis from "@/components/inventory/InsightKpis";
import InventoryContent from "@/components/inventory/InventoryContent";
import GroupByMenu from "@/components/inventory/GroupByMenu";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import DatabaseCard from "./_components/DatabaseCard";
import DatabasesTableView, { DATABASE_TABLE_PER_PAGE } from "./_components/DatabasesTableView";
import DatabasesDashboard from "./_components/DatabasesDashboard";
import InstancesDrawer from "./_components/InstancesDrawer";
import { databaseInsights, matchesDatabaseFilters, emptyDatabaseFilters, DEFAULT_DATABASE_INSIGHTS, type DatabaseFilters } from "./_components/databaseInsights";
import DatabaseForm, { type DatabaseFormPrefill } from "./DatabaseForm";
import DatabaseFilterDrawer from "./FilterDrawer";
import type { Database } from "@/lib/types";

export default function DatabasesPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();

  const { search, setSearch, filters, setFilters, viewMode, setViewMode, sort, setSort, activeFilterCount, groupBy, setGroupBy } =
    useInventoryFilters<DatabaseFilters>({ storageKey: "databases", emptyFilters: emptyDatabaseFilters, defaultSort: { field: "name", direction: "asc" } });

  const [showForm, setShowForm] = useState(false);
  const [prefill, setPrefill] = useState<DatabaseFormPrefill | undefined>();
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [showInstances, setShowInstances] = useState(false);
  const [tablePage, setTablePage] = useState(1);
  const [pageTab, selectTab] = usePageTab();
  const [visibleCount, setVisibleCount] = useState(24);
  const loadMoreRef = useRef<HTMLDivElement>(null);

  const canEdit = user?.role === "admin" || user?.role === "editor";

  // ponytail: the catalogue is small, so one unpaged list feeds KPIs, cards
  // and the table; server paging when it isn't.
  const { data: allDbs = [], isLoading } = useQuery({ queryKey: ["databases"], queryFn: () => databasesAPI.list() });
  const { data: instances = [] } = useQuery({ queryKey: ["database-instances"], queryFn: databasesAPI.instances });
  const unregistered = instances.filter((i) => i.registered_count === 0).length;

  const listKey = JSON.stringify([search, filters, sort]);
  const [shownKey, setShownKey] = useState(listKey);
  if (shownKey !== listKey) {
    setShownKey(listKey);
    setTablePage(1);
    setVisibleCount(24);
  }

  const dbs = useMemo(() => {
    const s = search.trim().toLowerCase();
    const filtered = allDbs.filter((d) =>
      matchesDatabaseFilters(d, filters) &&
      (!s || [d.name, d.db_name, d.description, d.use_cases, d.external_provider, d.instance?.nickname, d.host].some((v) => v?.toLowerCase().includes(s))));
    const dir = sort.direction === "desc" ? -1 : 1;
    return filtered.sort((a, b) => {
      if (sort.field === "engine") return dir * a.engine.localeCompare(b.engine) || a.name.localeCompare(b.name);
      if (sort.field === "updated_at") return dir * a.updated_at.localeCompare(b.updated_at);
      return dir * a.name.localeCompare(b.name);
    });
  }, [allDbs, search, filters, sort]);
  const grouping = useRelationGroups("database", groupBy, dbs);
  const tableRows = groupBy ? dbs : dbs.slice((tablePage - 1) * DATABASE_TABLE_PER_PAGE, tablePage * DATABASE_TABLE_PER_PAGE);

  const exportCSV = useExportCSV(
    dbs,
    [
      { key: "name", header: "name" },
      { key: "db_name", header: "db_name" },
      { key: "engine", header: "engine" },
      { key: "environment", header: "environment" },
      { key: "sensitivity", header: "sensitivity" },
      { key: "instance", header: "instance", transform: (d: Database) => d.instance?.nickname ?? "" },
      { key: "external_provider", header: "external_provider" },
      { key: "backup_policy", header: "backup_policy" },
      { key: "consumer_service_ids", header: "consumer_service_ids", transform: (d: Database) => (d.consumer_service_ids ?? []).join("; ") },
      { key: "project_ids", header: "project_ids", transform: (d: Database) => (d.project_ids ?? []).join("; ") },
    ],
    "databases_export",
  );

  const openCreate = useCallback((p?: DatabaseFormPrefill) => { setPrefill(p); setShowForm(true); }, []);
  const openBlank = useCallback(() => openCreate(), [openCreate]);
  useOpenOnParam("new", openBlank, canEdit);

  return (
    <PageShell>
      <PageHeader
        title={t("atlas.databases.title")}
        description={t("atlas.databases.subtitle")}
        addLabel={canEdit ? t("atlas.databases.add") : undefined}
        onAdd={canEdit ? openBlank : undefined}
        hideAddOnPhone
        controlsKey="databases"
        controlsBadge={activeFilterCount}
        tabs={{
          idBase: "databases",
          label: t("atlas.databases.title"),
          active: pageTab,
          onChange: selectTab,
          items: [
            { key: "overview", label: t("inventory.view.overview"), icon: ICON_PATHS.viewCards },
            { key: "dashboard", label: t("inventory.view.dashboard"), icon: ICON_PATHS.layoutGrid },
          ],
        }}
        controls={
          <ListToolbar
            viewMode={viewMode}
            onViewModeChange={setViewMode}
            search={search}
            onSearchChange={setSearch}
            onFilterClick={() => setShowFilters(true)}
            activeFilterCount={activeFilterCount}
            searchPlaceholder={t("atlas.databases.search")}
            actions={
              <>
                <ToolbarActionButton icon={ICON_PATHS.serverStack} label={t("atlas.databases.instances.button")} onClick={() => setShowInstances(true)} hideLabel="md" />
                {allDbs.length > 0 && (
                  <>
                    <GroupByMenu options={["service", "project", "contact", "entidade"]} value={groupBy} onChange={setGroupBy} />
                    <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export")} onClick={exportCSV} hideLabel="md" />
                  </>
                )}
                <TrashButton title={t("trash.title", { module: t("nav.databases") })} source={{
                  key: ["databases-trash"], fetch: databasesAPI.trash, restore: databasesAPI.restore, invalidate: [["databases"], ["database-instances"]],
                  label: (x) => x.name, meta: (x) => x.db_name,
                }} />
              </>
            }
          />
        }
      >
        {pageTab === "overview" ? (
          <InventoryOverview kpis={!isLoading && <InsightKpis layout="list" insights={databaseInsights(allDbs, t)} defaults={DEFAULT_DATABASE_INSIGHTS} storageKey="databases_kpis" filters={filters} onFiltersChange={setFilters} />}>
            {unregistered > 0 && (
              <StatusAlert variant="info">
                <span className="flex flex-wrap items-center justify-between gap-2">
                  {t("atlas.databases.instances.banner", { count: String(unregistered) })}
                  <Button size="sm" variant="secondary" onClick={() => setShowInstances(true)}>{t("atlas.databases.instances.review")}</Button>
                </span>
              </StatusAlert>
            )}
            <SearchBadge search={search} onClear={() => setSearch("")} />
            {!isLoading && allDbs.length > 0 && <SectionHeading>{t("atlas.databases.listing")}</SectionHeading>}
            <InventoryContent
              columns={3}
              isLoading={isLoading || grouping.isLoading}
              items={viewMode === "table" ? tableRows : dbs}
              groups={grouping.groups}
              viewMode={viewMode}
              emptyIcon="box"
              emptyTitle={search || activeFilterCount > 0 ? t("common.noResults") : t("atlas.databases.empty")}
              emptyDescription={search || activeFilterCount > 0 ? t("host.emptyStateFilter") : t("atlas.databases.emptyHint")}
              emptyAction={canEdit && !search && activeFilterCount === 0 ? (
                <Button size="sm" onClick={openBlank}>+ {t("atlas.databases.add")}</Button>
              ) : undefined}
              renderCard={(d) => <DatabaseCard db={d} />}
              renderTable={(items) => (
                <DatabasesTableView dbs={items} total={groupBy ? undefined : dbs.length} page={tablePage} onPageChange={setTablePage} sort={sort} onSortChange={setSort} />
              )}
              visibleCount={visibleCount}
              loadMoreRef={loadMoreRef}
              onLoadMore={() => setVisibleCount((c) => c + 24)}
              loadingMoreLabel={t("common.loadingMore")}
              loadMoreLabel={t("common.loadMore")}
            />
          </InventoryOverview>
        ) : (
          <DatabasesDashboard
            dbs={allDbs}
            filters={filters}
            onApplyFilter={(f) => { setFilters({ ...filters, ...f }); selectTab("overview"); }}
          />
        )}
      </PageHeader>

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={t("atlas.databases.addTitle")} subHeader={formSubHeader} footer={formFooter}>
        <DatabaseForm
          key={JSON.stringify(prefill ?? {})}
          prefill={prefill}
          onClose={() => setShowForm(false)}
          onSubHeaderChange={setFormSubHeader}
          onFooterChange={setFormFooter}
          onSuccess={(db) => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["databases"] });
            queryClient.invalidateQueries({ queryKey: ["database-instances"] });
            router.push(`/atlas/databases/${db.id}`);
          }}
        />
      </Drawer>

      <InstancesDrawer
        open={showInstances}
        onClose={() => setShowInstances(false)}
        instances={instances}
        canEdit={canEdit}
        onRegister={(i) => {
          setShowInstances(false);
          openCreate({ instanceServiceId: i.id, engine: i.engine === "other" ? undefined : i.engine });
        }}
      />

      <DatabaseFilterDrawer
        open={showFilters}
        onClose={() => setShowFilters(false)}
        filters={filters}
        onFiltersChange={setFilters}
        sort={sort}
        onSortChange={setSort}
        search={search}
        onSearchChange={setSearch}
      />

      <InventoryFAB
        canEdit={canEdit}
        hasItems={allDbs.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openBlank}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
        addLabel={t("atlas.databases.add")}
      />
    </PageShell>
  );
}
