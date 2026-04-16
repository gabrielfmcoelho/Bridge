"use client";

import { useState, useCallback, useRef, useMemo, useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { hostsAPI, sshAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Drawer from "@/components/ui/Drawer";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useExportCSV } from "@/hooks/useExportCSV";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import SearchBadge from "@/components/ui/SearchBadge";
import ListingLabel from "@/components/ui/ListingLabel";
import InventoryPageHeader from "@/components/inventory/InventoryPageHeader";
import InventoryContent from "@/components/inventory/InventoryContent";
import HostForm from "./HostForm";
import FilterDrawer, { emptyFilters } from "./FilterDrawer";
import HostCard from "./_components/HostCard";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import KpiSection from "./_components/KpiSection";
import BatchScanModal from "./_components/BatchScanModal";
import HostsTableView from "./_components/HostsTableView";
import type { Host, HostFilters, HostSortConfig } from "@/lib/types";

type ScanStatus = "pending" | "scanning" | "success" | "failed" | "skipped";
type ScanProgress = Record<string, { status: ScanStatus; error?: string }>;

export default function HostsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  // UI state
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<HostFilters>(emptyFilters);
  const [showForm, setShowForm] = useState(false);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [tablePage, setTablePage] = useState(1);
  const [visibleCount, setVisibleCount] = useState(24);
  const loadMoreRef = useRef<HTMLDivElement>(null);

  // Scan state
  const [showScanModal, setShowScanModal] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanProgress, setScanProgress] = useState<ScanProgress>({});
  const abortRef = useRef(false);

  const [sort, setSort] = useLocalStorage<HostSortConfig>("hosts_sort", { field: "nickname", direction: "asc" });

  useEffect(() => { setVisibleCount(24); setTablePage(1); }, [search, filters]);
  useEffect(() => {
    const el = loadMoreRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) setVisibleCount((c) => c + 24);
    }, { rootMargin: "200px" });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const { data: hosts = [], isLoading } = useQuery({
    queryKey: ["hosts", search, filters],
    queryFn: () => {
      const params: Record<string, string> = {};
      if (search) params.search = search;
      Object.entries(filters).forEach(([k, v]) => { if (v) params[k] = v; });
      return hostsAPI.list(params);
    },
  });

  const sortedHosts = useMemo(() => {
    const arr = [...hosts];
    const parsePct = (s?: string) => parseInt((s || "0").replace("%", "")) || 0;
    arr.sort((a, b) => {
      let cmp = 0;
      switch (sort.field) {
        case "containers_count": cmp = (a.containers_count || 0) - (b.containers_count || 0); break;
        case "resource_cpu": cmp = parsePct(a.scan_resources?.cpu_usage) - parsePct(b.scan_resources?.cpu_usage); break;
        case "resource_ram": cmp = parsePct(a.scan_resources?.ram_percent) - parsePct(b.scan_resources?.ram_percent); break;
        case "resource_disk": cmp = parsePct(a.scan_resources?.disk_percent) - parsePct(b.scan_resources?.disk_percent); break;
        case "situacao": cmp = a.situacao.localeCompare(b.situacao); break;
        default: cmp = a.nickname.localeCompare(b.nickname);
      }
      return sort.direction === "desc" ? -cmp : cmp;
    });
    return arr;
  }, [hosts, sort]);

  const canEdit = user?.role === "admin" || user?.role === "editor";
  const activeFilterCount = Object.values(filters).filter(Boolean).length;
  const scannableHosts = hosts.filter(h => h.has_key || h.has_password);

  const exportCSV = useExportCSV(
    hosts,
    [
      { key: "nickname", header: "nickname" },
      { key: "oficial_slug", header: "oficial_slug" },
      { key: "hostname", header: "hostname" },
      { key: "hospedagem", header: "hospedagem" },
      { key: "tipo_maquina", header: "tipo_maquina" },
      { key: "user", header: "user" },
      { key: "situacao", header: "situacao" },
      { key: "description", header: "description" },
      { key: "setor_responsavel", header: "setor_responsavel" },
      { key: "responsavel_interno", header: "responsavel_interno" },
      { key: "contato_responsavel_interno", header: "contato_responsavel_interno" },
      { key: "acesso_empresa_externa", header: "acesso_empresa_externa" },
      { key: "empresa_responsavel", header: "empresa_responsavel" },
      { key: "responsavel_externo", header: "responsavel_externo" },
      { key: "contato_responsavel_externo", header: "contato_responsavel_externo" },
      { key: "observacoes", header: "observacoes" },
      { key: "tags", header: "tags", transform: (h: Host) => (h.tags || []).join("; ") },
    ],
    "hosts_export",
  );

  const startBatchScan = useCallback(async () => {
    if (scanning) return;
    abortRef.current = false;
    setScanning(true);
    const initial: ScanProgress = {};
    scannableHosts.forEach(h => { initial[h.oficial_slug] = { status: "pending" }; });
    setScanProgress(initial);
    for (const host of scannableHosts) {
      if (abortRef.current) break;
      setScanProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "scanning" } }));
      const method: "password" | "key" = host.has_key && host.has_password
        ? (host.preferred_auth === "password" ? "password" : "key") : (host.has_key ? "key" : "password");
      try {
        const res = await sshAPI.testConnection(host.oficial_slug, method, true);
        setScanProgress(prev => ({ ...prev, [host.oficial_slug]: res.success ? { status: "success" } : { status: "failed", error: res.error || "Connection failed" } }));
      } catch (err: unknown) {
        setScanProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "failed", error: err instanceof Error ? err.message : "Unknown error" } }));
      }
    }
    setScanning(false);
    queryClient.invalidateQueries({ queryKey: ["hosts"] });
    queryClient.invalidateQueries({ queryKey: ["dashboard"] });
  }, [scanning, scannableHosts, queryClient]);

  const stopBatchScan = useCallback(() => { abortRef.current = true; }, []);

  const scanCounts = Object.values(scanProgress);
  const scannedCount = scanCounts.filter(s => s.status === "success" || s.status === "failed").length;
  const successCount = scanCounts.filter(s => s.status === "success").length;
  const failedCount = scanCounts.filter(s => s.status === "failed").length;


  return (
    <PageShell>
      <InventoryPageHeader
        title={t("host.title")}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        addLabel={canEdit ? t("host.addHost") : undefined}
        onAdd={canEdit ? () => setShowForm(true) : undefined}
      />

      {!isLoading && hosts.length > 0 && <KpiSection hosts={hosts} t={t} />}

      <SearchBadge search={search} onClear={() => setSearch("")} />
      <ListingLabel label={t("host.listing")} show={!isLoading && hosts.length > 0} />

      {/* Toolbar — search, filters, export, scan */}
      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          <div className="flex items-center gap-1.5">
            {hosts.length > 0 && (
              <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export") || "Export"} onClick={exportCSV} hideLabel="md" />
            )}
            {canEdit && scannableHosts.length > 0 && (
              <ToolbarActionButton icon={ICON_PATHS.scan} label={t("host.scanAll")} onClick={() => setShowScanModal(true)} hideLabel="md" />
            )}
          </div>
        }
      />

      <InventoryContent
        isLoading={isLoading}
        items={sortedHosts}
        viewMode={viewMode}
        emptyIcon="server"
        emptyTitle={t("common.noResults")}
        emptyDescription={search || activeFilterCount ? t("host.emptyStateFilter") : t("host.emptyStateAdd")}
        emptyAction={canEdit && !search && !activeFilterCount ? <Button size="sm" onClick={() => setShowForm(true)}>+ {t("host.addHost")}</Button> : undefined}
        renderCard={(host) => <HostCard host={host} />}
        renderTable={(items) => <HostsTableView hosts={items} tablePage={tablePage} onPageChange={setTablePage} t={t} />}
        visibleCount={visibleCount}
        loadMoreRef={loadMoreRef}
      />

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={t("host.addHost")} subHeader={formSubHeader} footer={formFooter}>
        <HostForm onClose={() => setShowForm(false)} onFooterChange={setFormFooter} onSubHeaderChange={setFormSubHeader}
          onSuccess={() => { setShowForm(false); queryClient.invalidateQueries({ queryKey: ["hosts"] }); }} />
      </Drawer>

      <ResponsiveModal open={showScanModal} onClose={() => { if (!scanning) setShowScanModal(false); }} title={t("host.scanAll")}>
        <BatchScanModal scanning={scanning} scanProgress={scanProgress} scannableHosts={scannableHosts}
          scannedCount={scannedCount} successCount={successCount} failedCount={failedCount}
          onStart={startBatchScan} onStop={stopBatchScan} onClose={() => setShowScanModal(false)} t={t} />
      </ResponsiveModal>

      <FilterDrawer open={showFilters} onClose={() => setShowFilters(false)} filters={filters} onFiltersChange={setFilters}
        sort={sort} onSortChange={setSort} search={search} onSearchChange={setSearch} />

      <InventoryFAB
        canEdit={canEdit}
        hasItems={hosts.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={() => setShowForm(true)}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
        addLabel={t("host.addHost")}
        extraActions={canEdit && scannableHosts.length > 0 ? [{
          label: t("host.scanAll"),
          icon: ICON_PATHS.scan,
          color: "#8b5cf6",
          onClick: () => setShowScanModal(true),
        }] : undefined}
      />
    </PageShell>
  );
}
