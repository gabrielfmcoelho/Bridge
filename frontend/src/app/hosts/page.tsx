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
import BatchDockerSetupModal from "./_components/BatchDockerSetupModal";
import BatchDockerLogsModal from "./_components/BatchDockerLogsModal";
import BatchSudoNopasswdModal from "./_components/BatchSudoNopasswdModal";
import BatchSetupKeyModal from "./_components/BatchSetupKeyModal";
import HostsTableView from "./_components/HostsTableView";
import type { Host, HostFilters, HostSortConfig } from "@/lib/types";

type ScanStatus = "pending" | "scanning" | "success" | "failed" | "skipped";
type ScanProgress = Record<string, { status: ScanStatus; error?: string; attempt?: number }>;

// activeMethodStatus returns the test status of the SSH method that would
// actually be used to connect to a host. Mirrors the picker in
// startBatchScan so bucket counts and the scan target list stay coherent.
function activeMethodStatus(h: Host): "success" | "failed" | "" {
  const useKey = h.has_key && h.has_password
    ? h.preferred_auth !== "password"
    : h.has_key;
  const status = useKey ? h.key_test_status : h.password_test_status;
  if (status === "success") return "success";
  if (status === "failed") return "failed";
  return "";
}

const SCAN_MAX_ATTEMPTS = 3; // 1 initial + 2 retries
// Linear backoff (not exponential): every retry waits the same short
// interval. Exponential made sense when a transient SSH dial failure
// could clear up over a few seconds, but in practice most repeat
// failures here are deterministic (auth, permissions, missing
// credential), so a long backoff just wastes one of N parallel scan
// slots on a host that will never succeed.
const SCAN_RETRY_BACKOFF_MS = [200, 200];

// Substrings that mark a fail-fast error: retrying won't change the
// outcome, so we burn the host's slot on attempt 1 instead of paying
// 3 × scan_time. Lowercased for case-insensitive matching.
const FAIL_FAST_ERROR_SUBSTRINGS = [
  "no password stored",
  "no key configured",
  "no key path configured",
  "host has both auth methods",
  "method must be",
  "permission denied",
  "publickey",
  "authentication failed",
  "auth failed",
  "incorrect password",
  "host has no password or key",
  "failed to decrypt password",
];

function isFailFastError(message: string): boolean {
  const lower = message.toLowerCase();
  return FAIL_FAST_ERROR_SUBSTRINGS.some((s) => lower.includes(s));
}

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
  const [showDockerModal, setShowDockerModal] = useState(false);
  const [showDockerLogsModal, setShowDockerLogsModal] = useState(false);
  const [showSudoModal, setShowSudoModal] = useState(false);
  const [showSetupKeyModal, setShowSetupKeyModal] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanProgress, setScanProgress] = useState<ScanProgress>({});
  const [scanConcurrency, setScanConcurrency] = useLocalStorage("hosts.scanConcurrency", 10);
  const [scanScope, setScanScope] = useLocalStorage<"all" | "failed" | "success" | "untested">("hosts.scanScope", "all");
  // "auto"     → respect each host's preferred_auth (legacy behaviour)
  // "password" → force password auth, skip hosts that have only a key
  // "key"      → force key auth, skip hosts that have only a password
  const [scanAuthMethod, setScanAuthMethod] = useLocalStorage<"auto" | "password" | "key">("hosts.scanAuthMethod", "auto");
  // abortRef gates new dispatch (worker pool stops claiming hosts).
  // abortControllerRef cancels in-flight HTTP requests so the operator
  // doesn't have to wait the default 120s timeout per stuck scan.
  const abortRef = useRef(false);
  const abortControllerRef = useRef<AbortController | null>(null);

  // Pinned search — restored from localStorage on mount, kept in sync via the pin button.
  const [pinnedSearch, setPinnedSearch] = useLocalStorage("hosts.pinnedSearch", "");
  useEffect(() => {
    if (pinnedSearch && !search) setSearch(pinnedSearch);
    // Mount-only restore; further changes to pinnedSearch should not overwrite live typing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
  const scannableHosts = useMemo(() => hosts.filter(h => h.has_key || h.has_password), [hosts]);
  // Bucketing uses the *active* SSH method (the one startBatchScan actually
  // dials with), not "any column failed". This matches what startBatchScan
  // picks at line 148 and ensures a host that succeeds via the active method
  // moves out of "failed" even if a stale status from the inactive method
  // still says 'failed'.
  const failedScannableHosts = useMemo(
    () => scannableHosts.filter(h => activeMethodStatus(h) === "failed"),
    [scannableHosts],
  );
  const successScannableHosts = useMemo(
    () => scannableHosts.filter(h => activeMethodStatus(h) === "success"),
    [scannableHosts],
  );
  const untestedScannableHosts = useMemo(
    () => scannableHosts.filter(h => activeMethodStatus(h) === ""),
    [scannableHosts],
  );
  // If the persisted scope has no candidates, fall back to "all" so the start button stays usable.
  const effectiveScope: "all" | "failed" | "success" | "untested" =
    scanScope === "failed" && failedScannableHosts.length > 0 ? "failed" :
    scanScope === "success" && successScannableHosts.length > 0 ? "success" :
    scanScope === "untested" && untestedScannableHosts.length > 0 ? "untested" :
    "all";
  const scopedHosts =
    effectiveScope === "failed" ? failedScannableHosts :
    effectiveScope === "success" ? successScannableHosts :
    effectiveScope === "untested" ? untestedScannableHosts :
    scannableHosts;
  // Method-based filter: "password" mode requires has_password, "key" mode
  // requires has_key. "auto" keeps every scoped host. Falls back to "auto"
  // when the picked method has zero candidates so the start button stays
  // usable instead of mysteriously refusing to fire.
  const passwordCapableHosts = scopedHosts.filter((h) => h.has_password);
  const keyCapableHosts = scopedHosts.filter((h) => h.has_key);
  const effectiveAuthMethod: "auto" | "password" | "key" =
    scanAuthMethod === "password" && passwordCapableHosts.length > 0 ? "password" :
    scanAuthMethod === "key" && keyCapableHosts.length > 0 ? "key" :
    "auto";
  const targetHosts =
    effectiveAuthMethod === "password" ? passwordCapableHosts :
    effectiveAuthMethod === "key" ? keyCapableHosts :
    scopedHosts;

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
    abortControllerRef.current = new AbortController();
    const signal = abortControllerRef.current.signal;
    setScanning(true);
    const initial: ScanProgress = {};
    targetHosts.forEach(h => { initial[h.oficial_slug] = { status: "pending" }; });
    setScanProgress(initial);

    // Sliding-window worker pool: scanConcurrency workers share a single
    // index cursor. Each worker grabs the next host atomically (cursor++ runs
    // synchronously between awaits in JS), processes it, then loops. Faster
    // hosts free up slots immediately instead of waiting for a batch peer.
    let cursor = 0;
    const workerCount = Math.min(Math.max(1, scanConcurrency), targetHosts.length);
    const workers = Array.from({ length: workerCount }, async () => {
      while (true) {
        if (abortRef.current) return;
        const i = cursor++;
        if (i >= targetHosts.length) return;
        const host = targetHosts[i];
        // When the operator picked an explicit method we honour it (the
        // target list was already filtered to hosts that support it).
        // Auto falls back to the per-host preferred_auth heuristic.
        const method: "password" | "key" =
          effectiveAuthMethod === "password" ? "password" :
          effectiveAuthMethod === "key" ? "key" :
          host.has_key && host.has_password
            ? (host.preferred_auth === "password" ? "password" : "key")
            : (host.has_key ? "key" : "password");

        let lastError = "Connection failed";
        let succeeded = false;
        let aborted = false;
        for (let attempt = 1; attempt <= SCAN_MAX_ATTEMPTS; attempt++) {
          if (abortRef.current) { aborted = true; break; }
          setScanProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "scanning", attempt } }));
          try {
            const res = await sshAPI.testConnection(host.oficial_slug, method, true, signal);
            if (res.success) {
              setScanProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "success", attempt } }));
              succeeded = true;
              break;
            }
            lastError = res.error || "Connection failed";
          } catch (err: unknown) {
            // Operator-initiated cancellation: surface as "skipped" rather
            // than failed so the batch summary doesn't conflate user stops
            // with real connection errors.
            if (err instanceof Error && err.name === "AbortError") {
              aborted = true;
              break;
            }
            lastError = err instanceof Error ? err.message : "Unknown error";
          }
          // Bail fast on errors that retrying won't change. Auth/permission
          // failures are deterministic — sitting in the backoff burns a
          // worker slot on a host that will never succeed.
          if (isFailFastError(lastError)) {
            break;
          }
          // Linear backoff before next retry (skip after final attempt).
          // The abort flag is also checked post-sleep so a stop press
          // mid-backoff lands within ~200ms instead of waiting 1.5s.
          if (attempt < SCAN_MAX_ATTEMPTS) {
            await new Promise(r => setTimeout(r, SCAN_RETRY_BACKOFF_MS[attempt - 1] ?? 200));
            if (abortRef.current) { aborted = true; break; }
          }
        }
        if (aborted) {
          setScanProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "skipped" } }));
          return;
        }
        if (!succeeded) {
          setScanProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "failed", error: lastError, attempt: SCAN_MAX_ATTEMPTS } }));
        }
      }
    });
    await Promise.all(workers);

    setScanning(false);
    abortControllerRef.current = null;
    queryClient.invalidateQueries({ queryKey: ["hosts"] });
    queryClient.invalidateQueries({ queryKey: ["dashboard"] });
  }, [scanning, targetHosts, scanConcurrency, effectiveAuthMethod, queryClient]);

  const stopBatchScan = useCallback(() => {
    abortRef.current = true;
    // Cancel in-flight HTTP requests so workers stuck waiting on a slow
    // SSH op resolve immediately instead of running to the 120s timeout.
    abortControllerRef.current?.abort();
    // Mark every host that hadn't finished yet as "skipped" so the modal
    // stops showing them as pending/scanning. Workers that were mid-flight
    // will also write "skipped" via the catch block, but doing it here
    // gives the operator instant visual feedback.
    setScanProgress(prev => {
      const next = { ...prev };
      for (const slug in next) {
        const status = next[slug]?.status;
        if (status === "pending" || status === "scanning") {
          next[slug] = { status: "skipped" };
        }
      }
      return next;
    });
  }, []);

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
        searchAdornment={
          <PinSearchButton
            search={search}
            pinnedSearch={pinnedSearch}
            onTogglePin={() => setPinnedSearch(pinnedSearch === search && search ? "" : search)}
            t={t}
          />
        }
        actions={
          <div className="flex items-center gap-1.5">
            {hosts.length > 0 && (
              <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export") || "Export"} onClick={exportCSV} hideLabel="md" />
            )}
            {canEdit && scannableHosts.length > 0 && (
              <ToolbarActionButton icon={ICON_PATHS.scan} label={t("host.scanAll")} onClick={() => setShowScanModal(true)} hideLabel="md" />
            )}
            {canEdit && scannableHosts.length > 0 && (
              <ToolbarActionButton icon={ICON_PATHS.cube} label={t("host.batchDocker")} onClick={() => setShowDockerModal(true)} hideLabel="md" />
            )}
            {canEdit && scannableHosts.length > 0 && (
              <ToolbarActionButton icon={ICON_PATHS.document} label={t("host.batchDockerLogs")} onClick={() => setShowDockerLogsModal(true)} hideLabel="md" />
            )}
            {canEdit && hosts.some(h => h.has_password) && (
              <ToolbarActionButton icon={ICON_PATHS.terminal} label={t("host.batchSudo")} onClick={() => setShowSudoModal(true)} hideLabel="md" />
            )}
            {canEdit && hosts.some(h => h.has_password) && (
              <ToolbarActionButton icon={ICON_PATHS.key} label={t("host.batchKey")} onClick={() => setShowSetupKeyModal(true)} hideLabel="md" />
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
        renderTable={(items) => <HostsTableView hosts={items} tablePage={tablePage} onPageChange={setTablePage} canEdit={canEdit} t={t} />}
        visibleCount={visibleCount}
        loadMoreRef={loadMoreRef}
        onLoadMore={() => setVisibleCount((c) => c + 24)}
        loadingMoreLabel={t("common.loadingMore")}
        loadMoreLabel={t("common.loadMore")}
      />

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={t("host.addHost")} subHeader={formSubHeader} footer={formFooter}>
        <HostForm onClose={() => setShowForm(false)} onFooterChange={setFormFooter} onSubHeaderChange={setFormSubHeader}
          onSuccess={() => { setShowForm(false); queryClient.invalidateQueries({ queryKey: ["hosts"] }); }} />
      </Drawer>

      <ResponsiveModal open={showScanModal} onClose={() => { if (!scanning) setShowScanModal(false); }} title={t("host.scanAll")}>
        <BatchScanModal scanning={scanning} scanProgress={scanProgress} scannableHosts={targetHosts}
          scannedCount={scannedCount} successCount={successCount} failedCount={failedCount}
          concurrency={scanConcurrency} onConcurrencyChange={setScanConcurrency}
          scope={effectiveScope} onScopeChange={setScanScope}
          authMethod={effectiveAuthMethod} onAuthMethodChange={setScanAuthMethod}
          passwordCapableCount={passwordCapableHosts.length}
          keyCapableCount={keyCapableHosts.length}
          allHostsCount={scannableHosts.length}
          failedHostsCount={failedScannableHosts.length}
          successHostsCount={successScannableHosts.length}
          untestedHostsCount={untestedScannableHosts.length}
          onStart={startBatchScan} onStop={stopBatchScan} onClose={() => setShowScanModal(false)}
          onViewFailed={() => { setFilters({ ...emptyFilters, scan_result: "failed" }); setShowScanModal(false); }}
          t={t} />
      </ResponsiveModal>

      <ResponsiveModal open={showDockerModal} onClose={() => setShowDockerModal(false)} title={t("host.batchDocker")}>
        <BatchDockerSetupModal hosts={hosts} onClose={() => setShowDockerModal(false)} t={t} />
      </ResponsiveModal>

      <ResponsiveModal open={showDockerLogsModal} onClose={() => setShowDockerLogsModal(false)} title={t("host.batchDockerLogs")}>
        <BatchDockerLogsModal hosts={hosts} onClose={() => setShowDockerLogsModal(false)} t={t} />
      </ResponsiveModal>

      <ResponsiveModal open={showSudoModal} onClose={() => setShowSudoModal(false)} title={t("host.batchSudo")}>
        <BatchSudoNopasswdModal hosts={hosts} onClose={() => setShowSudoModal(false)} t={t} />
      </ResponsiveModal>

      <ResponsiveModal open={showSetupKeyModal} onClose={() => setShowSetupKeyModal(false)} title={t("host.batchKey")}>
        <BatchSetupKeyModal hosts={hosts} onClose={() => setShowSetupKeyModal(false)} t={t} />
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
        extraActions={canEdit ? [
          ...(scannableHosts.length > 0 ? [{
            label: t("host.scanAll"),
            icon: ICON_PATHS.scan,
            color: "#8b5cf6",
            onClick: () => setShowScanModal(true),
          }] : []),
          ...(scannableHosts.length > 0 ? [{
            label: t("host.batchDocker"),
            icon: ICON_PATHS.cube,
            color: "#0ea5e9",
            onClick: () => setShowDockerModal(true),
          }] : []),
          ...(scannableHosts.length > 0 ? [{
            label: t("host.batchDockerLogs"),
            icon: ICON_PATHS.document,
            color: "#06b6d4",
            onClick: () => setShowDockerLogsModal(true),
          }] : []),
          ...(hosts.some(h => h.has_password) ? [{
            label: t("host.batchSudo"),
            icon: ICON_PATHS.terminal,
            color: "#f59e0b",
            onClick: () => setShowSudoModal(true),
          }] : []),
          ...(hosts.some(h => h.has_password) ? [{
            label: t("host.batchKey"),
            icon: ICON_PATHS.key,
            color: "#10b981",
            onClick: () => setShowSetupKeyModal(true),
          }] : []),
        ] : undefined}
      />
    </PageShell>
  );
}

function PinSearchButton({
  search,
  pinnedSearch,
  onTogglePin,
  t,
}: {
  search: string;
  pinnedSearch: string;
  onTogglePin: () => void;
  t: (key: string) => string;
}) {
  const isPinned = pinnedSearch !== "" && pinnedSearch === search;
  const canPin = search.trim() !== "" || isPinned;
  return (
    <button
      type="button"
      onClick={onTogglePin}
      disabled={!canPin}
      title={isPinned ? t("host.unpinSearch") : t("host.pinSearch")}
      aria-label={isPinned ? t("host.unpinSearch") : t("host.pinSearch")}
      className={`p-1 rounded transition-colors ${isPinned ? "text-[var(--accent)]" : "text-[var(--text-faint)] hover:text-[var(--text-secondary)]"} disabled:opacity-30 disabled:cursor-not-allowed`}
    >
      <svg className="w-4 h-4" fill={isPinned ? "currentColor" : "none"} viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z" />
      </svg>
    </button>
  );
}
