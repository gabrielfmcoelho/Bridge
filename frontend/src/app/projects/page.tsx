"use client";

import { useState, useMemo, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useExportCSV } from "@/hooks/useExportCSV";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import EmptyState from "@/components/ui/EmptyState";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import ListToolbar from "@/components/ui/ListToolbar";
import { SkeletonCard, SkeletonTable } from "@/components/ui/Skeleton";
import ProjectCard from "./_components/ProjectCard";
import ProjectsTableView from "./_components/ProjectsTableView";
import KpiSection from "./_components/KpiSection";
import ProjectsFAB from "./_components/ProjectsFAB";
import ProjectForm from "./ProjectForm";
import ProjectFilterDrawer, { emptyFilters, type ProjectFilters } from "./FilterDrawer";
import type { Project } from "@/lib/types";

export default function ProjectsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();


  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Project | null>(null);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<ProjectFilters>(emptyFilters);
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [sort, setSort] = useLocalStorage("projects_sort", "name");

  const canEdit = user?.role === "admin" || user?.role === "editor";
  const activeFilterCount = Object.values(filters).filter(Boolean).length;

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
      switch (sort) {
        case "situacao": return a.situacao.localeCompare(b.situacao);
        case "setor": return (a.setor_responsavel || "").localeCompare(b.setor_responsavel || "");
        default: return a.name.localeCompare(b.name);
      }
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
      {/* Header */}
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("project.title")}</h1>
        <div className="flex items-center gap-1.5">
          <div className="hidden sm:flex">
            <ViewToggle
              value={viewMode}
              onChange={(v) => setViewMode(v as "cards" | "table")}
              options={[
                { key: "cards", label: t("common.cardView") || "Cards", icon: VIEW_ICONS.cards },
                { key: "table", label: t("common.tableView") || "Table", icon: VIEW_ICONS.table },
              ]}
            />
          </div>
          {canEdit && (
            <div className="hidden sm:block">
              <Button size="sm" onClick={openCreate}><span className="mr-1">+</span> {t("project.addProject")}</Button>
            </div>
          )}
        </div>
      </div>

      {!isLoading && allProjects.length > 0 && <KpiSection projects={allProjects} t={t} />}

      {search && (
        <div className="flex items-center gap-2 mb-4">
          <span className="text-xs text-[var(--text-muted)]">{t("common.search")}:</span>
          <span className="text-xs text-[var(--text-primary)] font-medium">&ldquo;{search}&rdquo;</span>
          <button onClick={() => setSearch("")} className="text-xs text-[var(--text-faint)] hover:text-[var(--text-secondary)]">&times;</button>
        </div>
      )}

      {!isLoading && allProjects.length > 0 && (
        <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("project.listing") || "Projects"}</h2>
      )}

      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          allProjects.length > 0 ? (
            <button
              onClick={exportCSV}
              className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] transition-all"
              title={t("common.export") || "Export"}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <span className="hidden sm:inline">{t("common.export") || "Export"}</span>
            </button>
          ) : undefined
        }
      />

      {/* Content */}
      {isLoading ? (
        viewMode === "cards" ? (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}
          </div>
        ) : (
          <SkeletonTable rows={5} />
        )
      ) : filteredAndSorted.length === 0 ? (
        <EmptyState
          icon="folder"
          title={t("common.noResults")}
          description={search || activeFilterCount ? "Try adjusting your filters" : "Create your first project"}
          action={canEdit && !search && !activeFilterCount ? (
            <Button size="sm" onClick={openCreate}><span className="mr-1">+</span> {t("project.addProject")}</Button>
          ) : undefined}
        />
      ) : viewMode === "cards" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {filteredAndSorted.map((project, i) => (
            <div key={project.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
              <ProjectCard project={project} />
            </div>
          ))}
        </div>
      ) : (
        <ProjectsTableView projects={filteredAndSorted} t={t} />
      )}

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

      <ProjectsFAB
        canEdit={canEdit}
        hasProjects={allProjects.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openCreate}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
      />
    </PageShell>
  );
}
