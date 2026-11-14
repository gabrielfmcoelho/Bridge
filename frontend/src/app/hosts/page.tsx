"use client";

import { useState, useRef, useMemo, useEffect } from "react";
import { useQuery, useQueryClient, useMutation, keepPreviousData } from "@tanstack/react-query";
import { hostsAPI, scanAPI, proxmoxAPI, coolifyAPI } from "@/lib/api";
import { useScanRun } from "@/hooks/useScanRun";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { useAuth } from "@/contexts/AuthContext";
import { useOpenOnParam } from "@/hooks/useOpenOnParam";
import { usePageTab } from "@/hooks/usePageTab";
import HostsOSView from "./_components/HostsOSView";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Drawer from "@/components/ui/Drawer";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useDebounce } from "@/hooks/useDebounce";
import { useExportCSV } from "@/hooks/useExportCSV";
import ListToolbar from "@/components/ui/ListToolbar";
import TrashButton from "@/components/inventory/TrashDrawer";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import SearchBadge from "@/components/ui/SearchBadge";
import SectionHeading from "@/components/ui/SectionHeading";
import PageHeader from "@/components/ui/PageHeader";
import InventoryContent from "@/components/inventory/InventoryContent";
import InventoryOverview from "@/components/inventory/InventoryOverview";
import HostForm from "./HostForm";
import FilterDrawer, { emptyFilters } from "./FilterDrawer";
import HostCard from "./_components/HostCard";
import InventoryFAB from "@/components/inventory/InventoryFAB";
import KpiSection from "./_components/KpiSection";
import HostsDashboard from "./_components/HostsDashboard";
import BatchScanModal from "./_components/BatchScanModal";
import BatchDockerSetupModal from "./_components/BatchDockerSetupModal";
import BatchDockerLogsModal from "./_components/BatchDockerLogsModal";
import BatchSudoNopasswdModal from "./_components/BatchSudoNopasswdModal";
import BatchSetupKeyModal from "./_components/BatchSetupKeyModal";
import HostsTableView from "./_components/HostsTableView";
import OSUpdateModal, { type UpdateHost } from "./_components/OSUpdateModal";
import { RunDrawer } from "./_components/OSUpdateRuns";
import OSRestoreModal from "./_components/OSRestoreModal";
import { useLatestBackups } from "./_components/OSBadges";
import { osLabel } from "./_components/hostInsights";
import GroupByMenu from "@/components/inventory/GroupByMenu";
import Icon from "@/components/ui/Icon";
import DropdownMenu, { DropdownMenuGroup, DropdownMenuItem } from "@/components/ui/DropdownMenu";
import { useRelationGroups } from "@/hooks/useRelationGroups";
import type { GroupEntity } from "@/lib/grouping";
import type { Host, HostFilters, HostSortConfig, OSUpdateResult } from "@/lib/types";

// activeMethodStatus returns the test status of the SSH method that would
// actually be used to connect to a host. Mirrors the server's pick
// (scanMethod) so bucket counts and the scan target list stay coherent.
// The server runs at most this many scans of a run at once.
const SCAN_MAX_CONCURRENCY = 10;

function activeMethodStatus(h: Host): "success" | "failed" | "" {
  const useKey = h.has_key && h.has_password
    ? h.preferred_auth !== "password"
    : h.has_key;
  const status = useKey ? h.key_test_status : h.password_test_status;
  if (status === "success") return "success";
  if (status === "failed") return "failed";
  return "";
}

export default function HostsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const flag = useFlag();

  // UI state
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<HostFilters>(emptyFilters);
  const [showForm, setShowForm] = useState(false);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [pageTab, selectTab] = usePageTab(["os"] as const);
  const [tablePage, setTablePage] = useState(1);
  const [visibleCount, setVisibleCount] = useState(24);
  const loadMoreRef = useRef<HTMLDivElement>(null);

  // Scan state
  const [showScanModal, setShowScanModal] = useState(false);
  const [showDockerModal, setShowDockerModal] = useState(false);
  const [showDockerLogsModal, setShowDockerLogsModal] = useState(false);
  const [showSudoModal, setShowSudoModal] = useState(false);
  const [showSetupKeyModal, setShowSetupKeyModal] = useState(false);
  // Maintenance over the listed hosts: OS update or backup-only (admin).
  const [maintenance, setMaintenance] = useState<"security" | "backup" | null>(null);
  const [openRun, setOpenRun] = useState<number | null>(null);
  const [restoring, setRestoring] = useState<OSUpdateResult | null>(null);
  const [repairing, setRepairing] = useState<UpdateHost | null>(null);
  const [scanConcurrency, setScanConcurrency] = useLocalStorage("hosts.scanConcurrency", 10);
  const [scanScope, setScanScope] = useLocalStorage<"all" | "failed" | "success" | "untested">("hosts.scanScope", "all");
  // "auto"     → respect each host's preferred_auth (legacy behaviour)
  // "password" → force password auth, skip hosts that have only a key
  // "key"      → force key auth, skip hosts that have only a password
  const [scanAuthMethod, setScanAuthMethod] = useLocalStorage<"auto" | "password" | "key">("hosts.scanAuthMethod", "auto");
  // The batch scan runs on the server: this tab only starts, follows and
  // cancels it. Reopened, the page picks up my scan still running.
  const [scanRunId, setScanRunId] = useState<number | null>(null);

  // Pinned search — restored from localStorage on mount, kept in sync via the pin button.
  const [pinnedSearch, setPinnedSearch] = useLocalStorage("hosts.pinnedSearch", "");
  useEffect(() => {
    if (pinnedSearch && !search) setSearch(pinnedSearch);
    // Mount-only restore; further changes to pinnedSearch should not overwrite live typing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [sort, setSort] = useLocalStorage<HostSortConfig>("hosts_sort", { field: "nickname", direction: "asc" });
  const [groupBy, setGroupBy] = useLocalStorage<GroupEntity | "">("hosts_groupBy", "");

  // Typing asks the server once the user pauses, not once per key.
  const searchQ = useDebounce(search.trim(), 250);
  useEffect(() => { setVisibleCount(24); setTablePage(1); }, [searchQ, filters, sort]);
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
    queryKey: ["hosts", searchQ, filters, sort],
    // Keep the current cards on screen while the next search loads.
    placeholderData: keepPreviousData,
    queryFn: () => {
      const params: Record<string, string> = {};
      if (searchQ) params.search = searchQ;
      Object.entries(filters).forEach(([k, v]) => { if (v) params[k] = v; });
      // Server-authoritative sort (incl. scan metrics resolved against the
      // latest scan's numeric columns — P3). sort.field maps 1:1 to the backend
      // sort_by keys (nickname/situacao/resource_cpu/ram/disk/containers_count).
      params.sort_by = sort.field;
      params.sort_dir = sort.direction;
      return hostsAPI.list(params);
    },
  });

  // Server returns the list already sorted; render in that order. The full list
  // also feeds the batch-scan worker pool + scannable/failed/success counts
  // below, so hosts always loads it (card view + scan need the whole set).
  const sortedHosts = hosts;
  const grouping = useRelationGroups("host", groupBy, sortedHosts);

  // Table view drives REAL server-side pagination: a separate query fetches one
  // page (filtered+sorted+limited) so column-sort + paging are server-authoritative.
  // Only enabled in table mode; card/scan keep using the full list above.
  const TABLE_PER_PAGE = 20;
  const tableQuery = useQuery({
    queryKey: ["hosts-table", searchQ, filters, sort, tablePage],
    placeholderData: keepPreviousData,
    // Grouped, the table shows every (filtered) row per group from the full list.
    enabled: viewMode === "table" && !groupBy,
    queryFn: () => {
      const params: Record<string, string> = {};
      if (searchQ) params.search = searchQ;
      Object.entries(filters).forEach(([k, v]) => { if (v) params[k] = v; });
      params.sort_by = sort.field;
      params.sort_dir = sort.direction;
      params.page = String(tablePage);
      params.per_page = String(TABLE_PER_PAGE);
      return hostsAPI.listPaginated(params);
    },
  });
  const tableHosts = tableQuery.data?.data ?? [];
  const tableTotal = tableQuery.data?.meta.total ?? 0;

  const canEdit = user?.role === "admin" || user?.role === "editor";
  const backups = useLatestBackups(user?.role === "admin");
  useOpenOnParam("new", () => setShowForm(true), canEdit);
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

  const activeScans = useQuery({
    queryKey: ["scan-runs", "active"],
    queryFn: () => scanAPI.list({ active: true }),
    enabled: canEdit,
  });
  const scanRun = useScanRun(scanRunId ?? activeScans.data?.[0]?.id ?? null).data;
  const scanning = scanRun?.status === "running";
  // A run that just ended changed hosts' test status and scan data.
  const lastScanStatus = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (lastScanStatus.current === "running" && scanRun && scanRun.status !== "running") {
      for (const key of ["hosts", "hosts-table", "dashboard", "scan-runs"]) queryClient.invalidateQueries({ queryKey: [key] });
    }
    lastScanStatus.current = scanRun?.status;
  }, [scanRun, queryClient]);

  const startScan = useMutation({
    mutationFn: () => scanAPI.start({
      host_ids: targetHosts.map((h) => h.id),
      concurrency: Math.min(scanConcurrency, SCAN_MAX_CONCURRENCY),
      ...(effectiveAuthMethod === "auto" ? {} : { method: effectiveAuthMethod }),
    }),
    onSuccess: (run) => {
      queryClient.setQueryData(["scan-run", run.id], run);
      setScanRunId(run.id);
    },
    onError: (err) => flag({ appearance: "error", title: t("host.scanAll"), description: err.message }),
  });
  const cancelScan = useMutation({
    mutationFn: (id: number) => scanAPI.cancel(id),
    onSuccess: (run) => queryClient.setQueryData(["scan-run", run.id], run),
    onError: (err) => flag({ appearance: "error", title: t("host.stopScan"), description: err.message }),
  });

  // Outcome as a flag, not a banner: the sync runs in the background and its
  // result shouldn't push the list down.
  const proxmoxSync = useMutation({
    mutationFn: proxmoxAPI.sync,
    onMutate: () => flag({ appearance: "info", title: t("host.syncProxmoxRunning") }),
    onSuccess: (d) => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
      queryClient.invalidateQueries({ queryKey: ["hosts-table"] });
      // One line per server. A failed server or reasons for missing IPs are
      // actionable (URL, agent, token privilege), so a warning, which stays
      // until dismissed.
      const lines = d.servers.flatMap((s) => [
        `${s.name} — ${s.error
          ? t("host.proxmoxSyncFailed", { error: s.error })
          : t("host.proxmoxSyncDone", {
            found: String(s.found), created: String(s.created), updated: String(s.updated),
            deactivated: String(s.deactivated), no_ip: String(s.no_ip),
          })}`,
        ...(s.no_ip_reasons ?? []).map((r) => `  • ${r}`),
      ]);
      const warn = d.servers.some((s) => s.error || s.no_ip_reasons?.length);
      flag({ appearance: warn ? "warning" : "success", title: t("host.syncProxmox"), description: lines.join("\n") });
    },
    onError: (err) => flag({ appearance: "error", title: t("host.syncProxmox"), description: err.message }),
  });

  // Coolify inventory: stamps resources on the scanned containers, creates the
  // unseen ones offline, folds redeploy copies, then links DNS → service.
  const coolifySync = useMutation({
    mutationFn: coolifyAPI.sync,
    onMutate: () => flag({ appearance: "info", title: t("host.syncCoolifyRunning") }),
    onSuccess: (d) => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
      queryClient.invalidateQueries({ queryKey: ["hosts-table"] });
      queryClient.invalidateQueries({ queryKey: ["services"] });
      queryClient.invalidateQueries({ queryKey: ["graph"] });
      flag({
        appearance: d.unmatched.length ? "warning" : "success",
        title: t("host.syncCoolify"),
        description: [
          t("host.coolifySyncDone", {
            resources: String(d.resources), matched: String(d.matched), created: String(d.created),
            merged: String(d.merged_duplicates), projects: String(d.projects_set),
            dns: String(d.dns.service_links_added ?? 0), apis: String(d.apis_linked),
          }),
          ...(d.unmatched.length ? [t("host.coolifyUnmatched", { names: d.unmatched.join(", ") })] : []),
        ].join("\n"),
      });
    },
    onError: (err) => flag({ appearance: "error", title: t("host.syncCoolify"), description: err.message }),
  });

  const scanDone = (scanRun?.results ?? []).filter((x) => x.status !== "pending" && x.status !== "running").length;


  // Operations over the whole fleet: rare, and editor/admin only, so they sit
  // behind one menu instead of crowding the list toolbar. The phone FAB gets
  // the same list.
  const bulkActions = [
    ...(canEdit && scannableHosts.length > 0 ? [
      { label: scanning ? t("host.scanRunningAction", { done: String(scanDone), total: String(scanRun?.results?.length ?? 0) }) : t("host.scanAll"),
        icon: ICON_PATHS.scan, color: "var(--accent)", onClick: () => setShowScanModal(true) },
      { label: t("host.batchDocker"), icon: ICON_PATHS.cube, color: "var(--info)", onClick: () => setShowDockerModal(true) },
      { label: t("host.batchDockerLogs"), icon: ICON_PATHS.document, color: "var(--cyan)", onClick: () => setShowDockerLogsModal(true) },
    ] : []),
    ...(canEdit && hosts.some((h) => h.has_password) ? [
      { label: t("host.batchSudo"), icon: ICON_PATHS.terminal, color: "var(--warning)", onClick: () => setShowSudoModal(true) },
      { label: t("host.batchKey"), icon: ICON_PATHS.key, color: "var(--success)", onClick: () => setShowSetupKeyModal(true) },
    ] : []),
    ...(user?.role === "admin" && hosts.length > 0 ? [
      { label: t("host.bulkOSUpdate"), icon: ICON_PATHS.refresh, color: "var(--warning)", onClick: () => setMaintenance("security"), group: "maintenance" },
      { label: t("host.bulkBackup"), icon: ICON_PATHS.exportDoc, color: "var(--success)", onClick: () => setMaintenance("backup"), group: "maintenance" },
    ] : []),
    ...(user?.role === "admin" ? [
      { label: t("host.syncProxmox"), icon: ICON_PATHS.refresh, color: "var(--info)", onClick: () => proxmoxSync.mutate(), disabled: proxmoxSync.isPending, group: "sync" },
      { label: t("host.syncCoolify"), icon: ICON_PATHS.refresh, color: "var(--accent)", onClick: () => coolifySync.mutate(), disabled: coolifySync.isPending, group: "sync" },
    ] : []),
  ];

  return (
    <PageShell>
      <PageHeader
        title={t("host.title")}
        description={t("host.pageDescription")}
        addLabel={canEdit ? t("host.addHost") : undefined}
        onAdd={canEdit ? () => setShowForm(true) : undefined}
        hideAddOnPhone
        controlsKey="hosts"
        controlsBadge={activeFilterCount}
        tabs={{
          idBase: "hosts",
          label: t("host.title"),
          active: pageTab,
          onChange: selectTab,
          items: [
            { key: "overview", label: t("inventory.view.overview"), icon: ICON_PATHS.viewCards },
            { key: "dashboard", label: t("inventory.view.dashboard"), icon: ICON_PATHS.layoutGrid },
            { key: "os", label: t("host.osView.tab"), icon: ICON_PATHS.server },
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
                  <GroupByMenu options={["service", "dns", "project", "contact", "entidade"]} value={groupBy} onChange={setGroupBy} />
                )}
                {bulkActions.length > 0 && (
                  <DropdownMenu trigger={<ToolbarActionButton icon={ICON_PATHS.bolt} label={t("host.bulkActions")} hideLabel="md" />}>
                    {(["ops", "maintenance", "sync"] as const).map((g) => {
                      const items = bulkActions.filter((a) => ("group" in a ? a.group : "ops") === g);
                      return items.length > 0 && (
                        <DropdownMenuGroup key={g} title={t({ ops: "host.bulkGroupOps", maintenance: "host.bulkGroupMaintenance", sync: "host.bulkGroupSync" }[g])}>
                          {items.map((a) => (
                            <DropdownMenuItem
                              key={a.label}
                              onClick={a.onClick}
                              disabled={"disabled" in a ? a.disabled : false}
                              className="whitespace-nowrap"
                              elemBefore={<Icon path={a.icon} className="w-4 h-4" style={{ color: a.color }} />}
                            >
                              {a.label}
                            </DropdownMenuItem>
                          ))}
                        </DropdownMenuGroup>
                      );
                    })}
                  </DropdownMenu>
                )}
                {hosts.length > 0 && (
                  <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export")} onClick={exportCSV} hideLabel="md" />
                )}
                <TrashButton title={t("trash.title", { module: t("nav.hosts") })} source={{
                  key: ["hosts-trash"], fetch: hostsAPI.trash, restore: hostsAPI.restore, invalidate: [["hosts"], ["hosts-table"]],
                  label: (h) => h.nickname || h.oficial_slug, meta: (h) => h.hostname || h.oficial_slug,
                }} />
              </div>
            }
          />
        }
      >
        {pageTab === "overview" ? (
          <InventoryOverview kpis={!isLoading && <KpiSection layout="list" hosts={hosts} filters={filters} onFiltersChange={setFilters} sort={sort} onSortChange={setSort} />}>
              <SearchBadge search={search} onClear={() => setSearch("")} />
              {!isLoading && hosts.length > 0 && <SectionHeading>{t("host.listing")}</SectionHeading>}
              <InventoryContent
                columns={3}
                isLoading={grouping.isLoading || (viewMode === "table" && !groupBy ? tableQuery.isLoading : isLoading)}
                items={viewMode === "table" && !groupBy ? tableHosts : sortedHosts}
                groups={grouping.groups}
                viewMode={viewMode}
                emptyIcon="server"
                emptyTitle={t("common.noResults")}
                emptyDescription={search || activeFilterCount ? t("host.emptyStateFilter") : t("host.emptyStateAdd")}
                emptyAction={canEdit && !search && !activeFilterCount ? <Button size="sm" onClick={() => setShowForm(true)}>+ {t("host.addHost")}</Button> : undefined}
                renderCard={(host) => <HostCard host={host} />}
                renderTable={(items) => <HostsTableView hosts={items} total={groupBy ? undefined : tableTotal} tablePage={tablePage} onPageChange={setTablePage} sort={sort} onSortChange={setSort} canEdit={canEdit} backups={backups} t={t} />}
                visibleCount={visibleCount}
                loadMoreRef={loadMoreRef}
                onLoadMore={() => setVisibleCount((c) => c + 24)}
                loadingMoreLabel={t("common.loadingMore")}
                loadMoreLabel={t("common.loadMore")}
              />
          </InventoryOverview>
        ) : pageTab === "os" ? (
          <HostsOSView hosts={hosts} isAdmin={user?.role === "admin"} />
        ) : (
          <HostsDashboard
            hosts={hosts}
            filters={filters}
            sort={sort}
            onSortChange={setSort}
            onApplyFilter={(f) => { setFilters({ ...filters, ...f }); selectTab("overview"); }}
          />
        )}
      </PageHeader>

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={t("host.addHost")} subHeader={formSubHeader} footer={formFooter}>
        <HostForm onClose={() => setShowForm(false)} onFooterChange={setFormFooter} onSubHeaderChange={setFormSubHeader}
          onSuccess={() => { setShowForm(false); queryClient.invalidateQueries({ queryKey: ["hosts"] }); }} />
      </Drawer>

      <ResponsiveModal open={showScanModal} onClose={() => setShowScanModal(false)} title={t("host.scanAll")}>
        <BatchScanModal run={scanRun} starting={startScan.isPending} scannableHosts={targetHosts}
          concurrency={Math.min(scanConcurrency, SCAN_MAX_CONCURRENCY)} onConcurrencyChange={setScanConcurrency}
          scope={effectiveScope} onScopeChange={setScanScope}
          authMethod={effectiveAuthMethod} onAuthMethodChange={setScanAuthMethod}
          passwordCapableCount={passwordCapableHosts.length}
          keyCapableCount={keyCapableHosts.length}
          allHostsCount={scannableHosts.length}
          failedHostsCount={failedScannableHosts.length}
          successHostsCount={successScannableHosts.length}
          untestedHostsCount={untestedScannableHosts.length}
          onStart={() => startScan.mutate()} onStop={() => scanRun && cancelScan.mutate(scanRun.id)} stopping={cancelScan.isPending}
          onClose={() => setShowScanModal(false)}
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

      {maintenance && (
        <OSUpdateModal preset={maintenance} onClose={() => setMaintenance(null)}
          target={{ by: "hosts", hosts: hosts.map((h) => ({ id: h.id, nickname: h.nickname, slug: h.oficial_slug, os: osLabel(h) })) }}
          onStarted={(run) => { setMaintenance(null); setOpenRun(run.id); }} />
      )}
      {/* The run started here (the history lives in the OS tab and on each host). */}
      {openRun !== null && <RunDrawer id={openRun} onClose={() => setOpenRun(null)} onRestore={(x) => { setOpenRun(null); setRestoring(x); }}
        onRepair={(h) => { setOpenRun(null); setRepairing(h); }} />}
      {restoring && <OSRestoreModal backup={restoring} onClose={() => setRestoring(null)}
        onStarted={(r) => { setRestoring(null); setOpenRun(r.id); }} />}
      {repairing && <OSUpdateModal target={{ by: "hosts", hosts: [repairing] }} preset="repair" onClose={() => setRepairing(null)}
        onStarted={(run) => { setRepairing(null); setOpenRun(run.id); }} />}

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
        extraActions={bulkActions.length > 0 ? bulkActions.map(({ label, icon, color, onClick }) => ({ label, icon, color, onClick })) : undefined}
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
