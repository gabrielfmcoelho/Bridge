"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import TabBar, { type Tab } from "@/components/ui/TabBar";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import Pagination from "@/components/ui/Pagination";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { SkeletonTable } from "@/components/ui/Skeleton";
import VulnTable from "@/components/vulns/VulnTable";
import VulnDrawer from "@/components/vulns/VulnDrawer";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useDebounce } from "@/hooks/useDebounce";
import { integrationsAPI, vulnAPI, wazuhAPI } from "@/lib/api";
import { useFlag } from "@/contexts/FlagContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import EtipiImportModal from "./EtipiImportModal";
import CveCatalog from "./CveCatalog";
import DependencyList, { type DependencyFlag, type DependencyKindFilter } from "./DependencyList";

const PER_PAGE = 25;
type View = "open" | "triaged" | "resolved" | "unmatched" | "catalog" | "dependencies";
type Severity = "" | "critical" | "high" | "medium" | "low";
type Source = "" | "etipi" | "wazuh" | "inventory";

const VIEW_STATUS: Record<View, string> = {
  open: "open",
  triaged: "accepted,false_positive",
  resolved: "resolved",
  unmatched: "",
  catalog: "",
  dependencies: "",
};

export default function VulnList() {
  const { t } = useLocale();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";

  const [view, setView] = useState<View>("open");
  const [severity, setSeverity] = useState<Severity>("");
  const [source, setSource] = useState<Source>("");
  const [depKind, setDepKind] = useState<DependencyKindFilter>("components");
  const [depFlag, setDepFlag] = useState<DependencyFlag>("");
  const [search, setSearch] = useState("");
  const q = useDebounce(search.trim(), 300);
  // The page belongs to one filter set: changing a filter is back to page 1.
  const filterKey = [view, severity, source, q].join("|");
  const [paged, setPaged] = useState({ key: filterKey, page: 1 });
  const page = paged.key === filterKey ? paged.page : 1;
  const setPage = (p: number) => setPaged({ key: filterKey, page: p });
  // ?id= opens a finding directly (the link a GLPI ticket carries back).
  const searchParams = useSearchParams();
  const [openId, setOpenId] = useState<number | null>(() => Number(searchParams.get("id")) || null);
  const [importOpen, setImportOpen] = useState(false);
  const flag = useFlag();
  const queryClient = useQueryClient();
  // Settings are admin-only, as is the sync.
  const { data: integrations } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get, enabled: isAdmin, retry: false, staleTime: 60_000 });
  const wazuhEnabled = integrations?.wazuh?.wazuh_enabled === "true";
  const depsEnabled = integrations?.dependencies?.deps_cve_enabled === "true";
  const evaluateDeps = useMutation({
    mutationFn: vulnAPI.evaluateDependencies,
    onSuccess: (r) => {
      queryClient.invalidateQueries({ queryKey: ["vulnerabilities"] });
      flag({
        appearance: r.errors.length ? "warning" : "success",
        title: t("vuln.deps.evaluated"),
        description: t("vuln.deps.evaluatedSummary", {
          new: String(r.findings.new), resolved: String(r.findings.resolved),
          osv: String(r.osv_queried), nvd: String(r.nvd_queried),
        }) +
          (r.nvd_deferred ? ` · ${t("vuln.deps.nvdDeferred", { count: String(r.nvd_deferred) })}` : "") +
          (r.errors.length ? ` · ${r.errors.slice(0, 3).join("; ")}` : ""),
      });
    },
    onError: (e: Error) => flag({ appearance: "error", title: t("vuln.deps.evaluateFailed"), description: e.message }),
  });
  const syncWazuh = useMutation({
    mutationFn: wazuhAPI.sync,
    onSuccess: (r) => {
      queryClient.invalidateQueries({ queryKey: ["vulnerabilities"] });
      flag({
        appearance: "success",
        title: t("vuln.wazuhSynced"),
        description: t("vuln.importSummary", { new: String(r.new), resolved: String(r.resolved), unmatched: String(r.unmatched) }) +
          (r.unmatched_targets.length ? ` · ${t("vuln.unmatchedAgents", { names: r.unmatched_targets.slice(0, 10).join(", ") })}` : ""),
      });
    },
    onError: (e: Error) => flag({ appearance: "error", title: t("vuln.wazuhSyncFailed"), description: e.message }),
  });

  const params: Record<string, string> = { page: String(page), per_page: String(PER_PAGE) };
  if (VIEW_STATUS[view]) params.status = VIEW_STATUS[view];
  if (view === "unmatched") params.unmatched = "true";
  if (severity) params.severity = severity;
  if (source) params.source = source;
  if (q) params.search = q;

  const isCatalog = view === "catalog";
  const isDeps = view === "dependencies";
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "list", params],
    queryFn: () => vulnAPI.list(params),
    enabled: !isCatalog && !isDeps,
  });
  const rows = data?.data ?? [];
  const total = data?.meta.total ?? 0;

  const tabs: Tab[] = [
    { key: "open", label: t("vuln.view.open") },
    { key: "triaged", label: t("vuln.view.triaged") },
    { key: "resolved", label: t("vuln.view.resolved") },
    ...(isAdmin ? [{ key: "unmatched", label: t("vuln.view.unmatched") }] : []),
    { key: "catalog", label: t("vuln.view.catalog") },
    { key: "dependencies", label: t("vuln.view.dependencies") },
  ];

  return (
    <PageShell>
      <PageHeader
        title={t("nav.vulnerabilities")}
        subtitle={t("vuln.subtitle")}
        actions={
          isAdmin ? (
            <div className="flex items-center gap-2">
              {depsEnabled && (
                <Button size="sm" variant="secondary" onClick={() => evaluateDeps.mutate()} loading={evaluateDeps.isPending}>{t("vuln.deps.evaluate")}</Button>
              )}
              {wazuhEnabled && (
                <Button size="sm" variant="secondary" onClick={() => syncWazuh.mutate()} loading={syncWazuh.isPending}>{t("vuln.syncWazuh")}</Button>
              )}
              <Button size="sm" onClick={() => setImportOpen(true)}>{t("vuln.importEtipi")}</Button>
            </div>
          ) : undefined
        }
        controlsKey="vulnerabilities"
        controls={
          <>
            <TabBar tabs={tabs} activeTab={view} onChange={(k) => setView(k as View)} />
            <ListToolbar
              search={search}
              onSearchChange={setSearch}
              searchPlaceholder={isCatalog ? t("vuln.cveAnalysis.searchPlaceholder") : isDeps ? t("vuln.deps.searchPlaceholder") : t("vuln.searchPlaceholder")}
              actions={
                isCatalog ? undefined : isDeps ? <>
                  <ToolbarSelect<DependencyKindFilter>
                    name={t("vuln.deps.kindLabel")}
                    icon={ICON_PATHS.filter}
                    value={depKind}
                    onChange={setDepKind}
                    options={(["components", "packages", "all"] as const).map((k) => ({ value: k, label: t(`vuln.deps.kindFilter.${k}`) }))}
                  />
                  <ToolbarSelect<DependencyFlag>
                    name={t("vuln.deps.flag")}
                    icon={ICON_PATHS.filter}
                    value={depFlag}
                    onChange={setDepFlag}
                    options={[
                      { value: "", label: t("vuln.deps.flagAny") },
                      { value: "cves", label: t("vuln.deps.flagCves") },
                      { value: "eol", label: t("vuln.deps.flagEol") },
                    ]}
                  />
                </> : <>
                  <ToolbarSelect<Severity>
                    name={t("vuln.severity")}
                    icon={ICON_PATHS.filter}
                    value={severity}
                    onChange={setSeverity}
                    options={[
                      { value: "", label: t("vuln.anySeverity") },
                      ...(["critical", "high", "medium", "low"] as const).map((s) => ({ value: s, label: t(`vuln.severityLevel.${s}`) })),
                    ]}
                  />
                  <ToolbarSelect<Source>
                    name={t("vuln.source")}
                    icon={ICON_PATHS.filter}
                    value={source}
                    onChange={setSource}
                    options={[
                      { value: "", label: t("vuln.anySource") },
                      { value: "etipi", label: "ETIPI" },
                      { value: "wazuh", label: "Wazuh" },
                      { value: "inventory", label: t("vuln.sourceInventory") },
                    ]}
                  />
                </>
              }
            />
          </>
        }
      />

      {isCatalog ? (
        <CveCatalog search={q} />
      ) : isDeps ? (
        <DependencyList search={q} kind={depKind} flag={depFlag} onOpenFinding={setOpenId} />
      ) : isLoading ? (
        <SkeletonTable rows={8} />
      ) : isError ? (
        <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>
      ) : rows.length === 0 ? (
        <EmptyState icon="search" title={t(`vuln.empty.${view}`)} description={t("vuln.emptyHint")} />
      ) : (
        <div className="space-y-3">
          <VulnTable rows={rows} onOpen={setOpenId} />
          <Pagination page={page} totalPages={Math.max(1, Math.ceil(total / PER_PAGE))} total={total} perPage={PER_PAGE} onChange={setPage} />
        </div>
      )}

      <VulnDrawer id={openId} onClose={() => setOpenId(null)} />
      {isAdmin && <EtipiImportModal open={importOpen} onClose={() => setImportOpen(false)} />}
    </PageShell>
  );
}
