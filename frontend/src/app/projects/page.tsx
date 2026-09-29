"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectsAPI } from "@/lib/api";
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
import InventoryOverview from "@/components/inventory/InventoryOverview";
import InsightKpis from "@/components/inventory/InsightKpis";
import GroupByMenu from "@/components/inventory/GroupByMenu";
import PageHeader from "@/components/ui/PageHeader";
import InventoryContent from "@/components/inventory/InventoryContent";
import ProjectCard from "./_components/ProjectCard";
import ProjectsTableView from "./_components/ProjectsTableView";
import ProjectsDashboard from "./_components/ProjectsDashboard";
import { projectInsights, DEFAULT_PROJECT_INSIGHTS } from "./_components/projectInsights";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import ProjectForm from "./ProjectForm";
import ProjectFilterDrawer, { emptyFilters, type ProjectFilters } from "./FilterDrawer";
import type { Project } from "@/lib/types";

export default function ProjectsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const { search, setSearch, filters, setFilters, viewMode, setViewMode, sort, setSort, activeFilterCount, groupBy, setGroupBy } =
    useInventoryFilters<ProjectFilters>({ storageKey: "projects", emptyFilters, defaultSort: { field: "name", direction: "asc" } });

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
  const { data: allProjects = [], isLoading } = useQuery({
    queryKey: ["projects"],
    queryFn: projectsAPI.list,
  });

  // Table view drives REAL server-side filtering+sort+pagination (one page at a
  // time). Enabled only in table mode; KPIs/export/cards keep using allProjects.
  const TABLE_PER_PAGE = 20;
  const tableQuery = useQuery({
    queryKey: ["projects-table", search, filters, sort, tablePage],
    enabled: viewMode === "table" && !groupBy,
    queryFn: () => {
      const params: Record<string, string> = {};
      if (search) params.search = search;
      if (filters.situacao) params.situacao = filters.situacao;
      if (filters.tag) params.tag = filters.tag;
      params.sort_by = sort.field;
      params.sort_dir = sort.direction;
      params.page = String(tablePage);
      params.per_page = String(TABLE_PER_PAGE);
      return projectsAPI.listPaginated(params);
    },
  });
  const tableProjects = tableQuery.data?.data ?? [];
  const tableTotal = tableQuery.data?.meta.total ?? 0;

  useEffect(() => { setTablePage(1); setVisibleCount(24); }, [search, filters, sort]);

  const filteredAndSorted = useMemo(() => {
    let result = [...allProjects];
    if (search) {
      const s = search.toLowerCase();
      result = result.filter(p =>
        p.name.toLowerCase().includes(s) ||
        p.description?.toLowerCase().includes(s) ||
        p.setor_responsavel?.toLowerCase().includes(s)
      );
    }
    if (filters.tag) {
      result = result.filter(p => p.tags?.includes(filters.tag));
    }
    if (filters.situacao) {
      result = result.filter(p => p.situacao === filters.situacao);
    }
    result.sort((a, b) => {
      let cmp = 0;
      switch (sort.field) {
        case "situacao": cmp = a.situacao.localeCompare(b.situacao); break;
        case "setor": cmp = (a.setor_responsavel || "").localeCompare(b.setor_responsavel || ""); break;
        default: cmp = a.name.localeCompare(b.name);
      }
      return sort.direction === "desc" ? -cmp : cmp;
    });
    return result;
  }, [allProjects, search, filters, sort]);
  const grouping = useRelationGroups("project", groupBy, filteredAndSorted);

  const exportCSV = useExportCSV(
    filteredAndSorted,
    [
      { key: "name", header: "name" },
      { key: "description", header: "description" },
      { key: "situacao", header: "situacao" },
      { key: "setor_responsavel", header: "setor_responsavel" },
      { key: "main_responsavel_name", header: "responsavel" },
      { key: "services_count", header: "services" },
      { key: "issues_count", header: "open_issues" },
      { key: "tem_empresa_externa_responsavel", header: "empresa_externa", transform: (p: Project) => p.tem_empresa_externa_responsavel ? "yes" : "no" },
      { key: "is_directly_managed", header: "managed", transform: (p: Project) => p.is_directly_managed ? "yes" : "no" },
      { key: "tags", header: "tags", transform: (p: Project) => (p.tags || []).join("; ") },
    ],
    "projects_export",
  );

  const openCreate = useCallback(() => setShowForm(true), []);
  useOpenOnParam("new", openCreate, canEdit);

  return (
    <PageShell>
      <PageHeader
        title={t("project.title")}
        description={t("project.pageDescription")}
        addLabel={canEdit ? t("project.addProject") : undefined}
        onAdd={canEdit ? openCreate : undefined}
        hideAddOnPhone
        controlsKey="projects"
        controlsBadge={activeFilterCount}
        tabs={{
          idBase: "projects",
          label: t("project.title"),
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
              allProjects.length > 0 ? (
                <>
                  <GroupByMenu options={["host", "service", "dns", "contact", "entidade"]} value={groupBy} onChange={setGroupBy} />
                  <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export")} onClick={exportCSV} hideLabel="md" />
                </>
              ) : undefined
            }
          />
        }
      >
        {pageTab === "overview" ? (
          <InventoryOverview kpis={!isLoading && <InsightKpis layout="list" insights={projectInsights(allProjects, t)} defaults={DEFAULT_PROJECT_INSIGHTS} storageKey="projects_kpis" filters={filters} onFiltersChange={setFilters} />}>
            <SearchBadge search={search} onClear={() => setSearch("")} />
            {!isLoading && allProjects.length > 0 && <SectionHeading>{t("project.listing")}</SectionHeading>}
            <InventoryContent
              columns={3}
              isLoading={grouping.isLoading || (viewMode === "table" && !groupBy ? tableQuery.isLoading : isLoading)}
              items={viewMode === "table" && !groupBy ? tableProjects : filteredAndSorted}
              groups={grouping.groups}
              viewMode={viewMode}
              emptyIcon="folder"
              emptyTitle={t("common.noResults")}
              emptyDescription={search || activeFilterCount ? t("host.emptyStateFilter") : t("project.emptyStateAdd")}
              emptyAction={canEdit && !search && !activeFilterCount ? (
                <Button size="sm" onClick={openCreate}>+ {t("project.addProject")}</Button>
              ) : undefined}
              renderCard={(project) => <ProjectCard project={project} />}
              renderTable={(items) => <ProjectsTableView projects={items} total={groupBy ? items.length : tableTotal} tablePage={tablePage} onPageChange={setTablePage} sort={sort} onSortChange={setSort} t={t} />}
              visibleCount={visibleCount}
              loadMoreRef={loadMoreRef}
              onLoadMore={() => setVisibleCount((c) => c + 24)}
              loadingMoreLabel={t("common.loadingMore")}
              loadMoreLabel={t("common.loadMore")}
            />
          </InventoryOverview>
        ) : (
          <ProjectsDashboard
            projects={allProjects}
            filters={filters}
            onApplyFilter={(f) => { setFilters({ ...filters, ...f }); selectTab("overview"); }}
          />
        )}
      </PageHeader>

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={t("project.addProject")} subHeader={formSubHeader} footer={formFooter}>
        <ProjectForm
          initial={null}
          onClose={() => setShowForm(false)}
          onSubHeaderChange={setFormSubHeader}
          onFooterChange={setFormFooter}
          onSuccess={() => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["projects"] });
          }}
        />
      </Drawer>

      <ProjectFilterDrawer
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
        hasItems={allProjects.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openCreate}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
        addLabel={t("project.addProject")}
      />
    </PageShell>
  );
}
