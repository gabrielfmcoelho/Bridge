"use client";

import { useState, useMemo, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { servicesAPI } from "@/lib/api";
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
import ServiceCard from "./_components/ServiceCard";
import ServicesTableView from "./_components/ServicesTableView";
import KpiSection from "./_components/KpiSection";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import ServiceForm from "./ServiceForm";
import ServiceFilterDrawer, { emptyFilters, type ServiceFilters } from "./FilterDrawer";
import type { Service } from "@/lib/types";

export default function ServicesPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const { search, setSearch, filters, setFilters, viewMode, setViewMode, sort, setSort, activeFilterCount } =
    useInventoryFilters<ServiceFilters>({ storageKey: "services", emptyFilters, defaultSort: { field: "nickname", direction: "asc" } });

  const [showForm, setShowForm] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);

  const canEdit = user?.role === "admin" || user?.role === "editor";

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
      const av = (sort.field === "technology_stack" ? (a.technology_stack || "") : a.nickname).toLowerCase();
      const bv = (sort.field === "technology_stack" ? (b.technology_stack || "") : b.nickname).toLowerCase();
      const cmp = av.localeCompare(bv);
      return sort.direction === "desc" ? -cmp : cmp;
    });
    return arr;
  }, [allServices, search, filters, sort]);

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
      <InventoryPageHeader
        title={t("service.title")}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        addLabel={canEdit ? t("service.addService") : undefined}
        onAdd={canEdit ? openCreate : undefined}
      />

      {/* KPI indicators */}
      {!isLoading && allServices.length > 0 && <KpiSection services={allServices} t={t} />}

      <SearchBadge search={search} onClear={() => setSearch("")} />
      <ListingLabel label={t("service.listing") || "Services"} show={!isLoading && allServices.length > 0} />

      {/* Toolbar */}
      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          allServices.length > 0 ? (
            <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export") || "Export"} onClick={exportCSV} />
          ) : undefined
        }
      />

      <InventoryContent
        isLoading={isLoading}
        items={services}
        viewMode={viewMode}
        emptyIcon="box"
        emptyTitle={t("common.noResults")}
        emptyDescription={search || activeFilterCount > 0 ? t("host.emptyStateFilter") || "Try adjusting your filters" : t("service.emptyStateAdd") || "Add your first service"}
        emptyAction={canEdit && !search && activeFilterCount === 0 ? (
          <Button size="sm" onClick={openCreate}><span className="mr-1">+</span> {t("service.addService")}</Button>
        ) : undefined}
        renderCard={(svc) => <ServiceCard svc={svc} />}
        renderTable={(items) => <ServicesTableView services={items} t={t} />}
      />

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
        addColor="#a855f7"
      />
    </PageShell>
  );
}
