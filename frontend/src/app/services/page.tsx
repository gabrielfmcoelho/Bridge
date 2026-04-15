"use client";

import { useState, useMemo, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { servicesAPI } from "@/lib/api";
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
import { SkeletonCard } from "@/components/ui/Skeleton";
import ServiceCard from "./_components/ServiceCard";
import ServicesTableView from "./_components/ServicesTableView";
import KpiSection from "./_components/KpiSection";
import ServicesFAB from "./_components/ServicesFAB";
import ServiceForm from "./ServiceForm";
import ServiceFilterDrawer, { emptyFilters, type ServiceFilters } from "./FilterDrawer";
import type { Service } from "@/lib/types";

export default function ServicesPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const [showForm, setShowForm] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [showFilters, setShowFilters] = useState(false);
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<ServiceFilters>(emptyFilters);
  const [sortField, setSortField] = useLocalStorage<string>("services_sort", "nickname");

  const canEdit = user?.role === "admin" || user?.role === "editor";
  const activeFilterCount = Object.values(filters).filter(Boolean).length;

  const { data: allServices = [], isLoading } = useQuery({
    queryKey: ["services"],
    queryFn: servicesAPI.list,
  });

  const services = useMemo(() => {
    let filtered = allServices;
    if (search) {
      const s = search.toLowerCase();
      filtered = filtered.filter(svc =>
        svc.nickname.toLowerCase().includes(s) ||
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
    const arr = [...filtered];
    arr.sort((a, b) => {
      const av = (sortField === "technology_stack" ? (a.technology_stack || "") : a.nickname).toLowerCase();
      const bv = (sortField === "technology_stack" ? (b.technology_stack || "") : b.nickname).toLowerCase();
      return av.localeCompare(bv);
    });
    return arr;
  }, [allServices, search, filters, sortField]);

  const exportCSV = useExportCSV(
    allServices,
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

  return (
    <PageShell>
      {/* Header */}
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("service.title")}</h1>
        <div className="flex items-center gap-1.5">
          <div className="hidden sm:flex">
            <ViewToggle
              value={viewMode}
              onChange={(v) => setViewMode(v as "cards" | "table")}
              options={[
                { key: "cards", label: t("common.cardView") || "Card view", icon: VIEW_ICONS.cards },
                { key: "table", label: t("common.tableView") || "Table view", icon: VIEW_ICONS.table },
              ]}
            />
          </div>
          {canEdit && (
            <div className="hidden sm:block">
              <Button size="sm" onClick={openCreate}><span className="mr-1">+</span> {t("service.addService")}</Button>
            </div>
          )}
        </div>
      </div>

      {/* KPI indicators */}
      {!isLoading && allServices.length > 0 && <KpiSection services={allServices} t={t} />}

      {search && (
        <div className="flex items-center gap-2 mb-4">
          <span className="text-xs text-[var(--text-muted)]">{t("common.search")}:</span>
          <span className="text-xs text-[var(--text-primary)] font-medium">&ldquo;{search}&rdquo;</span>
          <button onClick={() => setSearch("")} className="text-xs text-[var(--text-faint)] hover:text-[var(--text-secondary)]">&times;</button>
        </div>
      )}

      {!isLoading && allServices.length > 0 && (
        <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("service.listing") || "Services"}</h2>
      )}

      {/* Toolbar */}
      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          allServices.length > 0 ? (
            <button
              onClick={exportCSV}
              className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] transition-all"
              title={t("common.export") || "Export"}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <span className="hidden sm:inline">{t("common.export") || "Exportar"}</span>
            </button>
          ) : undefined
        }
      />

      {/* Content */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : services.length === 0 ? (
        <EmptyState
          icon="box"
          title={t("common.noResults")}
          description={search || activeFilterCount > 0 ? "Try adjusting your filters" : "Add your first service"}
          action={canEdit && !search && activeFilterCount === 0 ? (
            <Button size="sm" onClick={openCreate}><span className="mr-1">+</span> {t("service.addService")}</Button>
          ) : undefined}
        />
      ) : viewMode === "cards" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {services.map((svc, i) => (
            <div key={svc.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
              <ServiceCard svc={svc} />
            </div>
          ))}
        </div>
      ) : (
        <ServicesTableView services={services} t={t} />
      )}

      {/* Create modal */}
      <ResponsiveModal open={showForm} onClose={() => setShowForm(false)} title={t("service.addService")} subHeader={formSubHeader}>
        <ServiceForm
          onSubHeaderChange={setFormSubHeader}
          onSuccess={() => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["services"] });
          }}
        />
      </ResponsiveModal>

      {/* Filter drawer */}
      <ServiceFilterDrawer
        open={showFilters}
        onClose={() => setShowFilters(false)}
        filters={filters}
        onFiltersChange={setFilters}
        sort={sortField}
        onSortChange={setSortField}
        search={search}
        onSearchChange={setSearch}
      />

      {/* Mobile FAB */}
      <ServicesFAB
        canEdit={canEdit}
        hasServices={allServices.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openCreate}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
      />
    </PageShell>
  );
}
