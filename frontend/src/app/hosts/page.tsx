"use client";

import { useState, useCallback, useRef, useMemo, useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { hostsAPI, sshAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Drawer from "@/components/ui/Drawer";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useExportCSV } from "@/hooks/useExportCSV";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import ListToolbar from "@/components/ui/ListToolbar";
import HostForm from "./HostForm";
import FilterDrawer, { emptyFilters } from "./FilterDrawer";
import HostCard from "./_components/HostCard";
import HostsFAB from "./_components/HostsFAB";
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
  const isMobile = useMediaQuery("(max-width: 639px)");

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

  const iconScan = "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z";
  const iconExport = "M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z";
  const svgProps = { className: "w-4 h-4", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", strokeWidth: 2 } as const;

  return (
    <PageShell>
      {/* Title + view toggle + add */}
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("host.title")}</h1>
        <div className="flex items-center gap-1.5">
          <div className="hidden sm:flex">
            <ViewToggle
              value={viewMode}
              onChange={(v) => setViewMode(v as "cards" | "table")}
              options={[
                { key: "cards", label: t("common.cardView"), icon: VIEW_ICONS.cards },
                { key: "table", label: t("common.tableView"), icon: VIEW_ICONS.table },
              ]}
            />
          </div>
          {canEdit && (
            <div className="hidden sm:block">
              <Button size="sm" onClick={() => setShowForm(true)}><span className="mr-1">+</span> {t("host.addHost")}</Button>
            </div>
          )}
        </div>
      </div>

      {!isLoading && hosts.length > 0 && <KpiSection hosts={hosts} t={t} />}

      {search && (
        <div className="flex items-center gap-2 mb-4">
          <span className="text-xs text-[var(--text-muted)]">{t("common.search")}:</span>
          <span className="text-xs text-[var(--text-primary)] font-medium">&ldquo;{search}&rdquo;</span>
          <button onClick={() => setSearch("")} className="text-xs text-[var(--text-faint)] hover:text-[var(--text-secondary)]">&times;</button>
        </div>
      )}

      {!isLoading && hosts.length > 0 && (
        <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("host.listing")}</h2>
      )}

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
              <button
                onClick={exportCSV}
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] transition-all"
                title={t("common.export") || "Export"}
              >
                <svg {...svgProps}><path strokeLinecap="round" strokeLinejoin="round" d={iconExport} /></svg>
                <span className="hidden md:inline">{t("common.export") || "Export"}</span>
              </button>
            )}
            {canEdit && scannableHosts.length > 0 && (
              <button
                onClick={() => setShowScanModal(true)}
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] transition-all"
                title={t("host.scanAll")}
              >
                <svg {...svgProps}><path strokeLinecap="round" strokeLinejoin="round" d={iconScan} /></svg>
                <span className="hidden md:inline">{t("host.scanAll")}</span>
              </button>
            )}
          </div>
        }
      />

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : hosts.length === 0 ? (
        <EmptyState
          icon="server"
          title={t("common.noResults")}
          description={search || activeFilterCount ? t("host.emptyStateFilter") : t("host.emptyStateAdd")}
          action={canEdit && !search && !activeFilterCount ? <Button size="sm" onClick={() => setShowForm(true)}>+ {t("host.addHost")}</Button> : undefined}
        />
      ) : viewMode === "cards" ? (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {sortedHosts.slice(0, visibleCount).map((host, i) => (
              <div key={host.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
                <HostCard host={host} />
              </div>
            ))}
          </div>
          {visibleCount < sortedHosts.length && <div ref={loadMoreRef} className="h-1" />}
        </>
      ) : (
        <HostsTableView hosts={sortedHosts} tablePage={tablePage} onPageChange={setTablePage} t={t} />
      )}

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

      <HostsFAB canEdit={canEdit} hasHosts={hosts.length > 0} hasScannableHosts={scannableHosts.length > 0}
        activeFilterCount={activeFilterCount} onAdd={() => setShowForm(true)} onFilter={() => setShowFilters(true)}
        onScan={() => setShowScanModal(true)} onExport={exportCSV} />
    </PageShell>
  );
}
