"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { servicesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useOpenOnParam } from "@/hooks/useOpenOnParam";
import { useExportCSV } from "@/hooks/useExportCSV";
import { useInventoryFilters } from "@/hooks/useInventoryFilters";
import { usePageTab } from "@/hooks/usePageTab";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import SearchBadge from "@/components/ui/SearchBadge";
import SectionHeading from "@/components/ui/SectionHeading";
import InventoryOverview from "@/components/inventory/InventoryOverview";
import InsightKpis from "@/components/inventory/InsightKpis";
import PageHeader from "@/components/ui/PageHeader";
import InventoryContent from "@/components/inventory/InventoryContent";
import ServiceCard from "./_components/ServiceCard";
import ServicesTableView from "./_components/ServicesTableView";
import GroupByMenu from "@/components/inventory/GroupByMenu";
import { useRelationGroups } from "@/hooks/useRelationGroups";
import ServicesDashboard from "./_components/ServicesDashboard";
import { serviceInsights, DEFAULT_SERVICE_INSIGHTS } from "./_components/serviceInsights";
import { serviceTitle } from "@/lib/serviceDisplay";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import ServiceForm from "./ServiceForm";
import ServiceFilterDrawer, { emptyFilters, originParams, type ServiceFilters } from "./FilterDrawer";
import type { Service } from "@/lib/types";

export default function ServicesPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const { search, setSearch, filters, setFilters, viewMode, setViewMode, sort, setSort, activeFilterCount, groupBy, setGroupBy } =
    useInventoryFilters<ServiceFilters>({ storageKey: "services", emptyFilters, defaultSort: { field: "nickname", direction: "asc" } });

  const [showForm, setShowForm] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [tablePage, setTablePage] = useState(1);
  const [pageTab, selectTab] = usePageTab();
  const [visibleCount, setVisibleCount] = useState(24);
  const loadMoreRef = useRef<HTMLDivElement>(null);

  const canEdit = user?.role === "admin" || user?.role === "editor";

  // Full list — feeds KPIs, CSV export, and the card view (which client-filters).
  const { data: allServices = [], isLoading } = useQuery({
    queryKey: ["services"],
    queryFn: servicesAPI.list,
  });

  // Table view drives REAL server-side filtering+sort+pagination (one page at a
  // time). Enabled only in table mode; KPIs/export/cards keep using allServices.
  const TABLE_PER_PAGE = 20;
  const tableQuery = useQuery({
    queryKey: ["services-table", search, filters, sort, tablePage],
    // Grouped, the table shows every (filtered) row per group from the full list.
    enabled: viewMode === "table" && !groupBy,
    queryFn: () => {
      const params: Record<string, string> = {};
      if (search) params.search = search;
      if (filters.tag) params.tag = filters.tag;
      if (filters.developed_by) params.developed_by = filters.developed_by;
      if (filters.is_external_dependency) params.is_external_dependency = filters.is_external_dependency;
      if (filters.orchestrator_managed) params.orchestrator_managed = filters.orchestrator_managed;
      if (filters.kind) params.kind = filters.kind;
      if (filters.status) params.status = filters.status;
      Object.assign(params, originParams(filters.origin));
      params.sort_by = sort.field;
      params.sort_dir = sort.direction;
      params.page = String(tablePage);
      params.per_page = String(TABLE_PER_PAGE);
      return servicesAPI.listPaginated(params);
    },
  });
  const tableServices = tableQuery.data?.data ?? [];
  const tableTotal = tableQuery.data?.meta.total ?? 0;

  useEffect(() => { setTablePage(1); setVisibleCount(24); }, [search, filters, sort]);

  const services = useMemo(() => {
    let filtered = allServices;
    if (search) {
      const s = search.toLowerCase();
      filtered = filtered.filter(svc =>
        svc.nickname.toLowerCase().includes(s) ||
        serviceTitle(svc).title.toLowerCase().includes(s) ||
        svc.description?.toLowerCase().includes(s) ||
        svc.technology_stack?.toLowerCase().includes(s)
      );
    }
    if (filters.tag) {
      filtered = filtered.filter(svc => svc.tags?.includes(filters.tag));
    }
    if (filters.developed_by) {
      filtered = filtered.filter(svc => svc.developed_by === filters.developed_by);
    }
    if (filters.is_external_dependency) {
      const val = filters.is_external_dependency === "yes";
      filtered = filtered.filter(svc => !!svc.is_external_dependency === val);
    }
    if (filters.orchestrator_managed) {
      const val = filters.orchestrator_managed === "yes";
      filtered = filtered.filter(svc => !!svc.orchestrator_managed === val);
    }
    if (filters.kind) filtered = filtered.filter(svc => (svc.service_kind || "none") === filters.kind);
    if (filters.status) filtered = filtered.filter(svc => svc.container_status === filters.status);
    if (filters.origin) {
      const o = originParams(filters.origin);
      filtered = filtered.filter(svc => (!o.source || (svc.source || "manual") === o.source) && (!o.discovery_kind || svc.discovery_kind === o.discovery_kind));
    }
    const arr = [...filtered];
    arr.sort((a, b) => {
      const av = (sort.field === "technology_stack" ? (a.technology_stack || "") : a.nickname).toLowerCase();
      const bv = (sort.field === "technology_stack" ? (b.technology_stack || "") : b.nickname).toLowerCase();
      const cmp = av.localeCompare(bv);
      return sort.direction === "desc" ? -cmp : cmp;
    });
    return arr;
  }, [allServices, search, filters, sort]);
  const grouping = useRelationGroups("service", groupBy, services);

  const exportCSV = useExportCSV(
    services,
    [
      { key: "nickname", header: "nickname" },
      { key: "description", header: "description" },
      { key: "technology_stack", header: "technology_stack" },
      { key: "developed_by", header: "developed_by" },
      { key: "orchestrator_managed", header: "orchestrator_managed" },
      { key: "is_external_dependency", header: "is_external_dependency" },
      { key: "external_provider", header: "external_provider" },
      { key: "tags", header: "tags", transform: (s: Service) => (s.tags || []).join("; ") },
    ],
    "services_export",
  );

  const openCreate = useCallback(() => setShowForm(true), []);
  useOpenOnParam("new", openCreate, canEdit);

  return (
    <PageShell>
      <PageHeader
        title={t("service.title")}
        description={t("service.pageDescription")}
        addLabel={canEdit ? t("service.addService") : undefined}
        onAdd={canEdit ? openCreate : undefined}
        hideAddOnPhone
        controlsKey="services"
        controlsBadge={activeFilterCount}
        tabs={{
          idBase: "services",
          label: t("service.title"),
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
            searchPlaceholder={t("common.search")}
            actions={
              allServices.length > 0 ? (
                <>
                  <GroupByMenu options={["host", "dns", "project", "contact", "entidade"]} value={groupBy} onChange={setGroupBy} />
                  <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export")} onClick={exportCSV} hideLabel="md" />
                </>
              ) : undefined
            }
          />
        }
      >
        {pageTab === "overview" ? (
          <InventoryOverview kpis={!isLoading && <InsightKpis layout="list" insights={serviceInsights(allServices, t)} defaults={DEFAULT_SERVICE_INSIGHTS} storageKey="services_kpis" filters={filters} onFiltersChange={setFilters} />}>
            <SearchBadge search={search} onClear={() => setSearch("")} />
            {!isLoading && allServices.length > 0 && <SectionHeading>{t("service.listing")}</SectionHeading>}
            <InventoryContent
              columns={3}
              isLoading={grouping.isLoading || (viewMode === "table" && !groupBy ? tableQuery.isLoading : isLoading)}
              items={viewMode === "table" && !groupBy ? tableServices : services}
              groups={grouping.groups}
              viewMode={viewMode}
              emptyIcon="box"
              emptyTitle={t("common.noResults")}
              emptyDescription={search || activeFilterCount > 0 ? t("host.emptyStateFilter") : t("service.emptyStateAdd")}
              emptyAction={canEdit && !search && activeFilterCount === 0 ? (
                <Button size="sm" onClick={openCreate}>+ {t("service.addService")}</Button>
              ) : undefined}
              renderCard={(svc) => <ServiceCard svc={svc} />}
              renderTable={(items) => <ServicesTableView services={items} total={groupBy ? undefined : tableTotal} tablePage={tablePage} onPageChange={setTablePage} sort={sort} onSortChange={setSort} t={t} />}
              visibleCount={visibleCount}
              loadMoreRef={loadMoreRef}
              onLoadMore={() => setVisibleCount((c) => c + 24)}
              loadingMoreLabel={t("common.loadingMore")}
              loadMoreLabel={t("common.loadMore")}
            />
          </InventoryOverview>
        ) : (
          <ServicesDashboard
            services={allServices}
            filters={filters}
            onApplyFilter={(f) => { setFilters({ ...filters, ...f }); selectTab("overview"); }}
          />
        )}
      </PageHeader>

      {/* Create modal */}
      <Drawer open={showForm} onClose={() => setShowForm(false)} title={t("service.addService")} subHeader={formSubHeader} footer={formFooter}>
        <ServiceForm
          onClose={() => setShowForm(false)}
          onSubHeaderChange={setFormSubHeader}
          onFooterChange={setFormFooter}
          onSuccess={() => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["services"] });
          }}
        />
      </Drawer>

      {/* Filter drawer */}
      <ServiceFilterDrawer
        open={showFilters}
        onClose={() => setShowFilters(false)}
        filters={filters}
        onFiltersChange={setFilters}
        sort={sort}
        onSortChange={setSort}
        search={search}
        onSearchChange={setSearch}
      />

      {/* Mobile FAB */}
      <InventoryFAB
        canEdit={canEdit}
        hasItems={allServices.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openCreate}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
        addLabel={t("service.addService")}
      />
    </PageShell>
  );
}
