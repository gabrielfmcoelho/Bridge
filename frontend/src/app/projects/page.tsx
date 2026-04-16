"use client";

import { useState, useMemo, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useExportCSV } from "@/hooks/useExportCSV";
import { useInventoryFilters } from "@/hooks/useInventoryFilters";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import SearchBadge from "@/components/ui/SearchBadge";
import ListingLabel from "@/components/ui/ListingLabel";
import InventoryPageHeader from "@/components/inventory/InventoryPageHeader";
import InventoryContent from "@/components/inventory/InventoryContent";
import ProjectCard from "./_components/ProjectCard";
import ProjectsTableView from "./_components/ProjectsTableView";
import KpiSection from "./_components/KpiSection";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import ProjectForm from "./ProjectForm";
import ProjectFilterDrawer, { emptyFilters, type ProjectFilters } from "./FilterDrawer";
import type { Project } from "@/lib/types";

export default function ProjectsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const { search, setSearch, filters, setFilters, viewMode, setViewMode, sort, setSort, activeFilterCount } =
    useInventoryFilters<ProjectFilters>({ storageKey: "projects", emptyFilters, defaultSort: { field: "name", direction: "asc" } });

  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Project | null>(null);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);

  const canEdit = user?.role === "admin" || user?.role === "editor";

  const { data: allProjects = [], isLoading } = useQuery({
    queryKey: ["projects"],
    queryFn: projectsAPI.list,
  });

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

  const exportCSV = useExportCSV(
    allProjects,
    [
      { key: "name", header: "name" },
      { key: "description", header: "description" },
      { key: "situacao", header: "situacao" },
      { key: "setor_responsavel", header: "setor_responsavel" },
      { key: "responsavel", header: "responsavel" },
      { key: "tem_empresa_externa_responsavel", header: "empresa_externa", transform: (p: Project) => p.tem_empresa_externa_responsavel ? "yes" : "no" },
      { key: "is_directly_managed", header: "managed", transform: (p: Project) => p.is_directly_managed ? "yes" : "no" },
      { key: "tags", header: "tags", transform: (p: Project) => (p.tags || []).join("; ") },
    ],
    "projects_export",
  );

  const openCreate = useCallback(() => { setEditing(null); setShowForm(true); }, []);

  return (
    <PageShell>
      <InventoryPageHeader
        title={t("project.title")}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        addLabel={canEdit ? t("project.addProject") : undefined}
        onAdd={canEdit ? openCreate : undefined}
      />

      {!isLoading && allProjects.length > 0 && <KpiSection projects={allProjects} t={t} />}

      <SearchBadge search={search} onClear={() => setSearch("")} />
      <ListingLabel label={t("project.listing") || "Projects"} show={!isLoading && allProjects.length > 0} />

      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          allProjects.length > 0 ? (
            <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export") || "Export"} onClick={exportCSV} />
          ) : undefined
        }
      />

      <InventoryContent
        isLoading={isLoading}
        items={filteredAndSorted}
        viewMode={viewMode}
        emptyIcon="folder"
        emptyTitle={t("common.noResults")}
        emptyDescription={search || activeFilterCount ? t("host.emptyStateFilter") || "Try adjusting your filters" : t("project.emptyStateAdd") || "Create your first project"}
        emptyAction={canEdit && !search && !activeFilterCount ? (
          <Button size="sm" onClick={openCreate}><span className="mr-1">+</span> {t("project.addProject")}</Button>
        ) : undefined}
        renderCard={(project) => <ProjectCard project={project} />}
        renderTable={(items) => <ProjectsTableView projects={items} t={t} />}
      />

      <ResponsiveModal open={showForm} onClose={() => setShowForm(false)} title={editing ? t("common.edit") : t("project.addProject")} subHeader={formSubHeader}>
        <ProjectForm
          initial={editing}
          onSubHeaderChange={setFormSubHeader}
          onSuccess={() => {
            setShowForm(false);
            setEditing(null);
            queryClient.invalidateQueries({ queryKey: ["projects"] });
          }}
        />
      </ResponsiveModal>

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
        addColor="#f59e0b"
      />
    </PageShell>
  );
}
