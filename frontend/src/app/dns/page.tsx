"use client";

import { useState, useMemo, useCallback, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { dnsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useExportCSV } from "@/hooks/useExportCSV";
import { useInventoryFilters } from "@/hooks/useInventoryFilters";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import SearchBadge from "@/components/ui/SearchBadge";
import ListingLabel from "@/components/ui/ListingLabel";
import InventoryPageHeader from "@/components/inventory/InventoryPageHeader";
import InventoryContent from "@/components/inventory/InventoryContent";
import DnsCard from "./_components/DnsCard";
import DnsTableView from "./_components/DnsTableView";
import KpiSection from "./_components/KpiSection";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import DnsForm from "./DnsForm";
import DnsFilterDrawer, { emptyFilters, type DNSFilters } from "./FilterDrawer";
import type { DNSRecord } from "@/lib/types";

export default function DNSPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const { search, setSearch, filters, setFilters, viewMode, setViewMode, sort, setSort, activeFilterCount } =
    useInventoryFilters<DNSFilters>({ storageKey: "dns", emptyFilters, defaultSort: { field: "domain", direction: "asc" } });

  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<DNSRecord | null>(null);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [tablePage, setTablePage] = useState(1);

  const canEdit = user?.role === "admin" || user?.role === "editor";

  const { data: allRecords = [], isLoading } = useQuery({
    queryKey: ["dns"],
    queryFn: dnsAPI.list,
  });

  // Table view drives REAL server-side filtering+sort+pagination (one page at a
  // time). Enabled only in table mode; KPIs/export/cards keep using allRecords.
  const tableQuery = useQuery({
    queryKey: ["dns-table", search, filters, sort, tablePage],
    enabled: viewMode === "table",
    queryFn: () => {
      const params: Record<string, string> = {};
      if (search) params.search = search;
      if (filters.situacao) params.situacao = filters.situacao;
      if (filters.tag) params.tag = filters.tag;
      if (filters.responsavel) params.responsavel = filters.responsavel;
      if (filters.has_https) params.has_https = filters.has_https;
      params.sort_by = sort.field;
      params.sort_dir = sort.direction;
      params.page = String(tablePage);
      params.per_page = "20";
      return dnsAPI.listPaginated(params);
    },
  });
  const tableRecords = tableQuery.data?.data ?? [];
  const tableTotal = tableQuery.data?.meta.total ?? 0;

  useEffect(() => { setTablePage(1); }, [search, filters, sort]);

  const filteredAndSorted = useMemo(() => {
    let result = [...allRecords];
    if (search) {
      const s = search.toLowerCase();
      result = result.filter(d => d.domain.toLowerCase().includes(s) || d.responsavel?.toLowerCase().includes(s));
    }
    if (filters.situacao) result = result.filter(d => d.situacao === filters.situacao);
    if (filters.tag) result = result.filter(d => d.tags?.includes(filters.tag));
    if (filters.responsavel) result = result.filter(d => d.responsavel === filters.responsavel);
    if (filters.has_https === "yes") result = result.filter(d => d.has_https);
    else if (filters.has_https === "no") result = result.filter(d => !d.has_https);

    result.sort((a, b) => {
      let cmp = 0;
      switch (sort.field) {
        case "situacao": cmp = a.situacao.localeCompare(b.situacao); break;
        case "responsavel": cmp = (a.responsavel || "").localeCompare(b.responsavel || ""); break;
        default: cmp = a.domain.localeCompare(b.domain);
      }
      return sort.direction === "desc" ? -cmp : cmp;
    });
    return result;
  }, [allRecords, search, filters, sort]);

  const deleteMutation = useMutation({
    mutationFn: (id: number) => dnsAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["dns"] }),
  });

  const exportCSV = useExportCSV(
    allRecords,
    [
      { key: "domain", header: "domain" },
      { key: "has_https", header: "has_https", transform: (d) => d.has_https ? "yes" : "no" },
      { key: "situacao", header: "situacao" },
      { key: "responsavel", header: "responsavel" },
      { key: "observacoes", header: "observacoes" },
      { key: "tags", header: "tags", transform: (d) => ((d as unknown as DNSRecord).tags || []).join("; ") },
    ],
    "dns_export",
  );

  const openCreate = useCallback(() => { setEditing(null); setShowForm(true); }, []);

  return (
    <PageShell>
      <InventoryPageHeader
        title={t("dns.title")}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        addLabel={canEdit ? t("dns.addDns") : undefined}
        onAdd={canEdit ? openCreate : undefined}
      />

      {!isLoading && allRecords.length > 0 && <KpiSection records={allRecords} t={t} />}

      <SearchBadge search={search} onClear={() => setSearch("")} />
      <ListingLabel label={t("dns.listing") || "DNS Records"} show={!isLoading && allRecords.length > 0} />

      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          allRecords.length > 0 ? (
            <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export") || "Export"} onClick={exportCSV} />
          ) : undefined
        }
      />

      <InventoryContent
        isLoading={viewMode === "table" ? tableQuery.isLoading : isLoading}
        items={viewMode === "table" ? tableRecords : filteredAndSorted}
        viewMode={viewMode}
        emptyIcon="globe"
        emptyTitle={t("common.noResults")}
        emptyDescription={search || activeFilterCount ? t("host.emptyStateFilter") || "Try adjusting your filters" : t("dns.emptyStateAdd") || "Add your first DNS record"}
        emptyAction={canEdit && !search && !activeFilterCount ? <Button size="sm" onClick={openCreate}>+ {t("dns.addDns")}</Button> : undefined}
        renderCard={(dns) => <DnsCard dns={dns} />}
        renderTable={(items) => <DnsTableView records={items} total={tableTotal} tablePage={tablePage} onPageChange={setTablePage} sort={sort} onSortChange={setSort} t={t} />}
      />

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={editing ? t("common.edit") : t("dns.addDns")} subHeader={formSubHeader}>
        <DnsForm
          initial={editing}
          onSubHeaderChange={setFormSubHeader}
          onSuccess={() => { setShowForm(false); setEditing(null); queryClient.invalidateQueries({ queryKey: ["dns"] }); }}
        />
      </Drawer>

      <DnsFilterDrawer
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
        hasItems={allRecords.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openCreate}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
        addLabel={t("dns.addDns")}
        addColor="#06b6d4"
      />
    </PageShell>
  );
}
