"use client";

import { useState, useMemo, useCallback, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { dnsAPI, coolifyAPI } from "@/lib/api";
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
import StatusAlert from "@/components/ui/StatusAlert";
import SearchBadge from "@/components/ui/SearchBadge";
import SectionHeading from "@/components/ui/SectionHeading";
import InventoryPageHeader from "@/components/inventory/InventoryPageHeader";
import InventoryContent from "@/components/inventory/InventoryContent";
import DnsCard from "./_components/DnsCard";
import DnsTableView from "./_components/DnsTableView";
import KpiSection from "./_components/KpiSection";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import DnsForm from "./DnsForm";
import DnsFilterDrawer, { emptyFilters, type DNSFilters } from "./FilterDrawer";
import { matchesCertFilter, compareCertExpiry } from "@/lib/dnsCert";
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
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [tablePage, setTablePage] = useState(1);

  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";

  const { data: allRecords = [], isLoading } = useQuery({
    queryKey: ["dns"],
    queryFn: dnsAPI.list,
  });

  // Table view drives REAL server-side filtering+sort+pagination (one page at a
  // time). Enabled only in table mode; KPIs use allRecords; cards/export use filteredAndSorted.
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
      if (filters.cert) params.cert = filters.cert;
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
    if (filters.cert) result = result.filter(d => matchesCertFilter(d, filters.cert));

    result.sort((a, b) => {
      let cmp = 0;
      switch (sort.field) {
        case "situacao": cmp = a.situacao.localeCompare(b.situacao); break;
        case "responsavel": cmp = (a.responsavel || "").localeCompare(b.responsavel || ""); break;
        case "cert_expires_at": return compareCertExpiry(a, b, sort.direction); // nulls last both ways
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

  const scanMutation = useMutation({
    mutationFn: dnsAPI.scanCerts,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dns"] });
      queryClient.invalidateQueries({ queryKey: ["dns-table"] });
    },
  });

  const syncMutation = useMutation({
    mutationFn: coolifyAPI.syncDNS,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dns"] });
      queryClient.invalidateQueries({ queryKey: ["dns-table"] });
    },
  });

  const exportCSV = useExportCSV(
    filteredAndSorted,
    [
      { key: "domain", header: "domain" },
      { key: "has_https", header: "has_https", transform: (d) => d.has_https ? "yes" : "no" },
      { key: "cert_expires_at", header: "cert_expires_at" },
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
      {!isLoading && allRecords.length > 0 && <SectionHeading>{t("dns.listing")}</SectionHeading>}

      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          allRecords.length > 0 || isAdmin ? (
            <>
              {canEdit && allRecords.length > 0 && (
                <ToolbarActionButton icon={ICON_PATHS.scan} label={t("dns.scanCerts")} onClick={() => scanMutation.mutate()} disabled={scanMutation.isPending} />
              )}
              {isAdmin && (
                <ToolbarActionButton icon={ICON_PATHS.refresh} label={t("dns.syncCoolify")} onClick={() => syncMutation.mutate()} disabled={syncMutation.isPending} />
              )}
              {allRecords.length > 0 && <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export")} onClick={exportCSV} />}
            </>
          ) : undefined
        }
      />

      {scanMutation.isPending && <StatusAlert variant="loading" className="mb-4">{t("dns.scanCertsRunning")}</StatusAlert>}
      {scanMutation.isSuccess && (
        <StatusAlert variant="success" className="mb-4">
          {t("dns.certScanDone", { scanned: String(scanMutation.data.scanned), ok: String(scanMutation.data.ok), failed: String(scanMutation.data.failed) })}
        </StatusAlert>
      )}
      {scanMutation.isError && <StatusAlert variant="error" className="mb-4">{scanMutation.error.message}</StatusAlert>}
      {syncMutation.isPending && <StatusAlert variant="loading" className="mb-4">{t("dns.syncCoolifyRunning")}</StatusAlert>}
      {syncMutation.isSuccess && (
        <StatusAlert variant="success" className="mb-4">
          {t("dns.coolifySyncDone", { found: String(syncMutation.data.found), created: String(syncMutation.data.created), links_added: String(syncMutation.data.links_added), no_host: String(syncMutation.data.no_host) })}
        </StatusAlert>
      )}
      {syncMutation.isError && <StatusAlert variant="error" className="mb-4">{syncMutation.error.message}</StatusAlert>}

      <InventoryContent
        isLoading={viewMode === "table" ? tableQuery.isLoading : isLoading}
        items={viewMode === "table" ? tableRecords : filteredAndSorted}
        viewMode={viewMode}
        emptyIcon="globe"
        emptyTitle={t("common.noResults")}
        emptyDescription={search || activeFilterCount ? t("host.emptyStateFilter") : t("dns.emptyStateAdd")}
        emptyAction={canEdit && !search && !activeFilterCount ? <Button size="sm" onClick={openCreate}>+ {t("dns.addDns")}</Button> : undefined}
        renderCard={(dns) => <DnsCard dns={dns} />}
        renderTable={(items) => <DnsTableView records={items} total={tableTotal} tablePage={tablePage} onPageChange={setTablePage} sort={sort} onSortChange={setSort} t={t} />}
      />

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={editing ? t("common.edit") : t("dns.addDns")} subHeader={formSubHeader} footer={formFooter}>
        <DnsForm
          initial={editing}
          onSubHeaderChange={setFormSubHeader}
          onFooterChange={setFormFooter}
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
