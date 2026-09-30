"use client";

import { useState, useMemo, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiCatalogAPI } from "@/lib/api";
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
import PageHeader from "@/components/ui/PageHeader";
import TrashButton from "@/components/inventory/TrashDrawer";
import InventoryOverview from "@/components/inventory/InventoryOverview";
import InsightKpis from "@/components/inventory/InsightKpis";
import InventoryContent from "@/components/inventory/InventoryContent";
import GroupByMenu from "@/components/inventory/GroupByMenu";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import ApiCard from "./_components/ApiCard";
import ApisTableView, { API_TABLE_PER_PAGE } from "./_components/ApisTableView";
import ApisDashboard from "./_components/ApisDashboard";
import EndpointSearch from "./_components/EndpointSearch";
import { apiInsights, matchesApiFilters, emptyApiFilters, DEFAULT_API_INSIGHTS, type ApiFilters } from "./_components/apiInsights";
import ApiForm from "./ApiForm";
import ApiFilterDrawer from "./FilterDrawer";
import type { ApiCatalog } from "@/lib/types";

export default function ApisPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();

  const { search, setSearch, filters, setFilters, viewMode, setViewMode, sort, setSort, activeFilterCount, groupBy, setGroupBy } =
    useInventoryFilters<ApiFilters>({ storageKey: "apis", emptyFilters: emptyApiFilters, defaultSort: { field: "name", direction: "asc" } });

  const [showForm, setShowForm] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [tablePage, setTablePage] = useState(1);
  const [pageTab, selectTab] = usePageTab();
  // Endpoint search across APIs: a third tab over the same toolbar search.
  const [endpoints, setEndpoints] = useState(false);
  const [visibleCount, setVisibleCount] = useState(24);
  const loadMoreRef = useRef<HTMLDivElement>(null);

  const canEdit = user?.role === "admin" || user?.role === "editor";

  // ponytail: the catalog is small, so one unpaged list feeds KPIs, cards and
  // the table; server paging when it isn't.
  const { data: allApis = [], isLoading } = useQuery({ queryKey: ["api-catalog"], queryFn: () => apiCatalogAPI.list() });

  // A new search/filter/sort starts from the first page (reset during render,
  // not in an effect).
  const listKey = JSON.stringify([search, filters, sort]);
  const [shownKey, setShownKey] = useState(listKey);
  if (shownKey !== listKey) {
    setShownKey(listKey);
    setTablePage(1);
    setVisibleCount(24);
  }

  const apis = useMemo(() => {
    const s = search.trim().toLowerCase();
    const filtered = allApis.filter((a) =>
      matchesApiFilters(a, filters) &&
      (!s || [a.name, a.title, a.description, a.base_url].some((v) => v?.toLowerCase().includes(s))));
    const dir = sort.direction === "desc" ? -1 : 1;
    return filtered.sort((a, b) => {
      if (sort.field === "operation_count") return dir * (a.operation_count - b.operation_count);
      if (sort.field === "updated_at") return dir * a.updated_at.localeCompare(b.updated_at);
      return dir * a.name.localeCompare(b.name);
    });
  }, [allApis, search, filters, sort]);
  const grouping = useRelationGroups("api_catalog", groupBy, apis);
  const tableRows = groupBy ? apis : apis.slice((tablePage - 1) * API_TABLE_PER_PAGE, tablePage * API_TABLE_PER_PAGE);
  const specVersions = useMemo(() => [...new Set(allApis.map((a) => a.spec_version).filter(Boolean))].sort(), [allApis]);

  const exportCSV = useExportCSV(
    apis,
    [
      { key: "name", header: "name" },
      { key: "title", header: "title" },
      { key: "version_label", header: "version" },
      { key: "spec_version", header: "spec_version" },
      { key: "source_type", header: "source_type" },
      { key: "source_url", header: "source_url" },
      { key: "base_url", header: "base_url" },
      { key: "docs_url", header: "docs_url" },
      { key: "operation_count", header: "operation_count" },
      { key: "service_ids", header: "service_ids", transform: (a: ApiCatalog) => (a.service_ids ?? []).join("; ") },
      { key: "project_ids", header: "project_ids", transform: (a: ApiCatalog) => (a.project_ids ?? []).join("; ") },
    ],
    "apis_export",
  );

  const openCreate = useCallback(() => setShowForm(true), []);
  useOpenOnParam("new", openCreate, canEdit);

  const activeTab = endpoints ? "endpoints" : pageTab;
  const selectPageTab = (k: string) => {
    setEndpoints(k === "endpoints");
    if (k !== "endpoints") selectTab(k);
  };

  return (
    <PageShell>
      <PageHeader
        title={t("atlas.apis.title")}
        description={t("atlas.apis.subtitle")}
        addLabel={canEdit ? t("atlas.apis.add") : undefined}
        onAdd={canEdit ? openCreate : undefined}
        hideAddOnPhone
        controlsKey="apis"
        controlsBadge={activeFilterCount}
        tabs={{
          idBase: "apis",
          label: t("atlas.apis.title"),
          active: activeTab,
          onChange: selectPageTab,
          items: [
            { key: "overview", label: t("inventory.view.overview"), icon: ICON_PATHS.viewCards },
            { key: "dashboard", label: t("inventory.view.dashboard"), icon: ICON_PATHS.layoutGrid },
            { key: "endpoints", label: t("atlas.apis.tabEndpoints"), icon: ICON_PATHS.code },
          ],
        }}
        controls={
          <ListToolbar
            viewMode={endpoints ? undefined : viewMode}
            onViewModeChange={endpoints ? undefined : setViewMode}
            search={search}
            onSearchChange={setSearch}
            onFilterClick={() => setShowFilters(true)}
            activeFilterCount={activeFilterCount}
            searchPlaceholder={endpoints ? t("atlas.apis.searchEndpoints") : t("atlas.apis.search")}
            actions={
              <>
                {allApis.length > 0 && !endpoints && (
                  <>
                    <GroupByMenu options={["service", "project", "contact", "entidade"]} value={groupBy} onChange={setGroupBy} />
                    <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export")} onClick={exportCSV} hideLabel="md" />
                  </>
                )}
                <TrashButton title={t("trash.title", { module: t("nav.apis") })} source={{
                  key: ["api-catalog-trash"], fetch: apiCatalogAPI.trash, restore: apiCatalogAPI.restore, invalidate: [["api-catalog"], ["api-ops"]],
                  label: (x) => x.name, meta: (x) => x.title,
                }} />
              </>
            }
          />
        }
      >
        {endpoints ? (
          <EndpointSearch search={search} />
        ) : pageTab === "overview" ? (
          <InventoryOverview kpis={!isLoading && <InsightKpis layout="list" insights={apiInsights(allApis, t)} defaults={DEFAULT_API_INSIGHTS} storageKey="apis_kpis" filters={filters} onFiltersChange={setFilters} />}>
            <SearchBadge search={search} onClear={() => setSearch("")} />
            {!isLoading && allApis.length > 0 && <SectionHeading>{t("atlas.apis.listing")}</SectionHeading>}
            <InventoryContent
              columns={3}
              isLoading={isLoading || grouping.isLoading}
              items={viewMode === "table" ? tableRows : apis}
              groups={grouping.groups}
              viewMode={viewMode}
              emptyIcon="box"
              emptyTitle={search || activeFilterCount > 0 ? t("common.noResults") : t("atlas.apis.empty")}
              emptyDescription={search || activeFilterCount > 0 ? t("host.emptyStateFilter") : t("atlas.apis.emptyHint")}
              emptyAction={canEdit && !search && activeFilterCount === 0 ? (
                <Button size="sm" onClick={openCreate}>+ {t("atlas.apis.add")}</Button>
              ) : undefined}
              renderCard={(a) => <ApiCard api={a} />}
              renderTable={(items) => (
                <ApisTableView apis={items} total={groupBy ? undefined : apis.length} page={tablePage} onPageChange={setTablePage} sort={sort} onSortChange={setSort} />
              )}
              visibleCount={visibleCount}
              loadMoreRef={loadMoreRef}
              onLoadMore={() => setVisibleCount((c) => c + 24)}
              loadingMoreLabel={t("common.loadingMore")}
              loadMoreLabel={t("common.loadMore")}
            />
          </InventoryOverview>
        ) : (
          <ApisDashboard
            apis={allApis}
            filters={filters}
            onApplyFilter={(f) => { setFilters({ ...filters, ...f }); selectTab("overview"); }}
          />
        )}
      </PageHeader>

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={t("atlas.apis.importTitle")} subHeader={formSubHeader} footer={formFooter}>
        <ApiForm
          onClose={() => setShowForm(false)}
          onSubHeaderChange={setFormSubHeader}
          onFooterChange={setFormFooter}
          onSuccess={(api) => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["api-catalog"] });
            router.push(`/atlas/apis/${api.id}`);
          }}
        />
      </Drawer>

      <ApiFilterDrawer
        open={showFilters}
        onClose={() => setShowFilters(false)}
        filters={filters}
        onFiltersChange={setFilters}
        sort={sort}
        onSortChange={setSort}
        search={search}
        onSearchChange={setSearch}
        specVersions={specVersions}
      />

      <InventoryFAB
        canEdit={canEdit}
        hasItems={allApis.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openCreate}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
        addLabel={t("atlas.apis.add")}
      />
    </PageShell>
  );
}
