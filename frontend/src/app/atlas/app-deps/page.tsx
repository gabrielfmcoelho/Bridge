"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { appDepsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { usePageTab } from "@/hooks/usePageTab";
import { useAppDepLinks } from "@/hooks/useAppDepLinks";
import { useExportCSV } from "@/hooks/useExportCSV";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";
import {
  buildLandscapeGraph, CRITICALITY_ORDER, criticalityKey, indexByCi, linkCounts, linkRows, reportRows, rowMatches, type AppDepLinkRow,
} from "@/lib/appDeps";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import SectionCard from "@/components/ui/SectionCard";
import { Skeleton } from "@/components/ui/Skeleton";
import AppDepsGraph from "@/components/graph/AppDepsGraph";
import AppDepsReport from "./_components/AppDepsReport";
import SyncButton from "./_components/SyncButton";

const TABS = ["map"] as const;

export default function AppDepsPage() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const canSync = user?.role === "admin" || user?.role === "editor";
  const [tab, selectTab] = usePageTab(TABS);
  const [search, setSearch] = useState("");
  const [criticality, setCriticality] = useState("all");
  const [linked, setLinked] = useState<"all" | "noHost" | "none">("all");

  const { data, isLoading, error } = useQuery({ queryKey: ["app-deps"], queryFn: appDepsAPI.list });
  const sources = useMemo(() => data ?? [], [data]);
  const index = useMemo(() => indexByCi(sources), [sources]);
  const allRows = useMemo(() => reportRows(index), [index]);
  const { links: assetLinks } = useAppDepLinks({}, sources.length > 0);
  const counts = useMemo(() => linkCounts(assetLinks), [assetLinks]);
  const rows = useMemo(
    () => allRows.filter((r) => {
      const c = counts.get(r.ci);
      const linkOk = linked === "all" || (linked === "noHost" ? !c?.host : !c);
      return linkOk && (criticality === "all" || r.criticality === criticality) && rowMatches(r, index.byCi.get(r.ci)?.catalog, search);
    }),
    [allRows, index, criticality, search, counts, linked],
  );
  const landscape = useMemo(() => buildLandscapeGraph(index), [index]);
  const lastSync = useMemo(
    () => sources.reduce<string | undefined>((m, s) => (!m || s.synced_at > m ? s.synced_at : m), undefined),
    [sources],
  );

  // One CSV line per declared link, over the CIs the filters show.
  const csvRows = useMemo<AppDepLinkRow[]>(
    () => rows.flatMap((r) => { const e = index.byCi.get(r.ci); return e ? linkRows(e.catalog, index) : []; }),
    [rows, index],
  );
  const exportCSV = useExportCSV(csvRows, [
    { key: "ci", header: "ci" },
    { key: "direction", header: "direction" },
    { key: "other", header: "other_ci" },
    { key: "component", header: "component" },
    { key: "localComponent", header: "local_component" },
    { key: "technology", header: "technology" },
    { key: "endpoint", header: "endpoint" },
    { key: "criticality", header: "criticality" },
    { key: "via", header: "via" },
    { key: "description", header: "description" },
  ], "app-dependencies");

  const critOptions = [
    { value: "all", label: t("appDeps.filter.allCriticalities") },
    ...CRITICALITY_ORDER.map((c) => ({ value: c as string, label: t(criticalityKey(c)!) })),
  ];
  const isMap = tab === "map";
  const empty = !isLoading && !error && sources.length === 0;

  return (
    <PageShell>
      <PageHeader
        title={t("appDeps.title")}
        description={t("appDeps.description")}
        indicators={sources.length > 0 ? (
          <span className="text-xs text-[var(--text-muted)]">
            {t("appDeps.indicator", { count: String(index.byCi.size) })}
            {lastSync && ` · ${t("appDeps.syncedAgo", { ago: getTimeAgo(lastSync, locale) })}`}
          </span>
        ) : undefined}
        actions={canSync ? <SyncButton /> : undefined}
        controlsKey="app-deps"
        controls={!isMap && sources.length > 0 ? (
          <ListToolbar
            search={search}
            onSearchChange={setSearch}
            searchPlaceholder={t("appDeps.searchPlaceholder")}
            actions={
              <>
                <ToolbarSelect name={t("appDeps.filter.criticality")} icon={ICON_PATHS.filter} value={criticality} options={critOptions} onChange={setCriticality} />
                <ToolbarSelect name={t("appDeps.filter.links")} icon={ICON_PATHS.link} value={linked} onChange={setLinked} options={[
                  { value: "all" as const, label: t("appDeps.filter.linksAll") },
                  { value: "noHost" as const, label: t("appDeps.filter.linksNoHost") },
                  { value: "none" as const, label: t("appDeps.filter.linksNone") },
                ]} />
                <ToolbarActionButton icon={ICON_PATHS.exportDoc} label={t("common.export")} onClick={exportCSV} hideLabel="md" />
              </>
            }
          />
        ) : undefined}
        tabs={{
          idBase: "app-deps",
          label: t("appDeps.title"),
          active: isMap ? "map" : "overview",
          onChange: selectTab,
          items: [
            { key: "overview", label: t("appDeps.tabReport"), icon: ICON_PATHS.viewTable },
            { key: "map", label: t("appDeps.tabMap"), icon: ICON_PATHS.branch },
          ],
        }}
      >
        {error ? (
          <StatusAlert variant="error">{(error as Error).message || t("appDeps.loadError")}</StatusAlert>
        ) : isLoading ? (
          <Skeleton className="w-full h-[50vh] rounded-[var(--radius-lg)]" />
        ) : empty ? (
          <EmptyState
            icon="topology"
            title={t("appDeps.empty")}
            description={t("appDeps.emptyHint")}
            action={canSync ? <SyncButton /> : undefined}
            secondaryAction={user?.role === "admin" ? (
              <Link href="/settings/integrations" className="text-xs font-medium text-[var(--accent)] hover:underline">{t("appDeps.openSettings")}</Link>
            ) : undefined}
          />
        ) : (
          <div className="space-y-4">
            {index.failed.length > 0 && (
              <SectionCard title={t("appDeps.failed.title")} description={t("appDeps.failed.description")} count={index.failed.length} body="flush" collapsible defaultOpen={false}>
                <ul className="divide-y divide-[var(--border-subtle)]">
                  {index.failed.map((s) => (
                    <li key={s.gitlab_project_id} className="px-5 py-2.5 text-xs">
                      <a href={s.file_url || s.web_url} target="_blank" rel="noopener noreferrer" className="font-mono text-[var(--accent)] hover:underline">{s.project_path}</a>
                      <p className="font-mono text-[var(--danger)] mt-0.5 break-words">{s.parse_error}</p>
                    </li>
                  ))}
                </ul>
              </SectionCard>
            )}
            {index.duplicates.size > 0 && (
              <StatusAlert variant="warning">
                {t("appDeps.duplicates", {
                  list: [...index.duplicates.entries()].map(([ci, more]) => `${ci} (${[index.byCi.get(ci)!.source, ...more].map((s) => s.project_path).join(", ")})`).join("; "),
                })}
              </StatusAlert>
            )}
            {isMap ? (
              index.byCi.size > 0 ? <AppDepsGraph graph={landscape} /> : <EmptyState compact icon="topology" title={t("appDeps.noParsed")} />
            ) : rows.length > 0 ? (
              <AppDepsReport rows={rows} index={index} counts={counts} />
            ) : (
              <EmptyState compact icon="search" title={t("common.noResults")} description={t("appDeps.noMatch")} />
            )}
          </div>
        )}
      </PageHeader>
    </PageShell>
  );
}
