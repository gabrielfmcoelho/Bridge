"use client";

import { Suspense, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { useLocale } from "@/contexts/LocaleContext";
import PageShell from "@/components/layout/PageShell";
import TabBar from "@/components/ui/TabBar";
import { Skeleton } from "@/components/ui/Skeleton";
import EmptyState from "@/components/ui/EmptyState";

import { fetchLineage } from "@/lib/lineage/loader";
import { buildIndexes } from "@/lib/lineage/indexes";
import OverviewPanel from "@/components/lineage/OverviewPanel";
import IssuesPanel from "@/components/lineage/IssuesPanel";
import LineageGraph from "@/components/lineage/LineageGraph";
import TablesPanel from "@/components/lineage/TablesPanel";

type ViewKey = "overview" | "graph" | "tables" | "issues";

export default function LineagePage() {
  return (
    <Suspense fallback={<PageShell><Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" /></PageShell>}>
      <LineagePageInner />
    </Suspense>
  );
}

function LineagePageInner() {
  const { t } = useLocale();
  const sp = useSearchParams();
  const urlView = (sp.get("view") as ViewKey | null) ?? "overview";
  const issueKind = sp.get("kind");

  const [view, setView] = useState<ViewKey>(urlView);
  const [focus, setFocus] = useState<string | null>(sp.get("focus"));

  const { data, isLoading, error } = useQuery({
    queryKey: ["lineage"],
    queryFn: ({ signal }) => fetchLineage(signal),
    staleTime: Infinity,
  });

  const indexes = useMemo(() => (data ? buildIndexes(data) : null), [data]);

  const tabs = useMemo(() => {
    const issueCount =
      (data?.warnings?.length ?? 0) +
      (data?.coverage?.gaps?.orphan_tasks?.length ?? 0) +
      (data?.coverage?.gaps?.isolated_models?.length ?? 0) +
      (data?.coverage?.gaps?.unused_sources?.length ?? 0) +
      (data?.coverage?.gaps?.unused_macros?.length ?? 0);
    return [
      { key: "overview", label: t("atlas.lineage.tabs.overview") },
      { key: "graph",    label: t("atlas.lineage.tabs.graph") },
      { key: "tables",   label: t("atlas.lineage.tabs.tables") },
      { key: "issues",   label: t("atlas.lineage.tabs.issues"), badge: issueCount },
    ];
  }, [data, t]);

  function selectTab(key: string) {
    const v = key as ViewKey;
    setView(v);
    updateUrl(sp, { view: v === "overview" ? undefined : v });
  }

  return (
    <PageShell fullBleed={view === "graph"}>
      <div className={view === "graph" ? "flex flex-col h-full" : ""}>
        <div className={view === "graph" ? "p-4 pb-2 border-b border-[var(--border-subtle)]" : ""}>
          <div className="flex items-center justify-between mb-3">
            <div>
              <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>
                {t("atlas.lineage.title")}
              </h1>
              {data?.generated_at && (
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  {t("atlas.lineage.generatedAt")}: {new Date(data.generated_at).toLocaleString()}
                </p>
              )}
            </div>
          </div>
          <TabBar tabs={tabs} activeTab={view} onChange={selectTab} />
        </div>

        <div className={view === "graph" ? "flex-1 min-h-0" : "mt-4"}>
          {isLoading ? (
            <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
          ) : error ? (
            <EmptyState
              icon="server"
              title={t("common.error")}
              description={error instanceof Error ? error.message : String(error)}
            />
          ) : !indexes ? (
            <EmptyState
              icon="server"
              title={t("common.noResults")}
              description={t("atlas.lineage.notGenerated")}
            />
          ) : view === "overview" ? (
            <OverviewPanel indexes={indexes} onNavigate={(v, params) => goTo(setView, setFocus, sp, v, params)} />
          ) : view === "graph" ? (
            <LineageGraph indexes={indexes} focusId={focus} />
          ) : view === "tables" ? (
            <TablesPanel
              indexes={indexes}
              onOpenInGraph={(id) => goTo(setView, setFocus, sp, "graph", { focus: id })}
            />
          ) : (
            <IssuesPanel indexes={indexes} filterKind={issueKind} onNavigate={(v, params) => goTo(setView, setFocus, sp, v, params)} />
          )}
        </div>
      </div>
    </PageShell>
  );
}

function goTo(
  setView: (v: ViewKey) => void,
  setFocus: (id: string | null) => void,
  sp: ReadonlyURLSearchParams,
  view: ViewKey,
  extra: Record<string, string | undefined> = {},
) {
  setView(view);
  if (Object.prototype.hasOwnProperty.call(extra, "focus")) {
    setFocus(extra.focus ?? null);
  }
  updateUrl(sp, { ...extra, view: view === "overview" ? undefined : view });
}

/**
 * Update the URL without triggering a Next.js navigation / RSC fetch.
 * The page state is fully client-side, so we just want a shareable URL.
 */
function updateUrl(sp: ReadonlyURLSearchParams, patch: Record<string, string | undefined>) {
  if (typeof window === "undefined") return;
  const params = new URLSearchParams(sp.toString());
  for (const [k, v] of Object.entries(patch)) {
    if (v === undefined || v === null || v === "") params.delete(k);
    else params.set(k, v);
  }
  const qs = params.toString();
  const url = `/atlas/lineage${qs ? `?${qs}` : ""}`;
  window.history.replaceState(null, "", url);
}

type ReadonlyURLSearchParams = ReturnType<typeof useSearchParams>;
