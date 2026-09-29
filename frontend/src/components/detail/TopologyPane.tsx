"use client";

import { useState, type ReactNode } from "react";
import EmptyState from "@/components/ui/EmptyState";
import Spinner from "@/components/ui/Spinner";
import ViewToggle from "@/components/ui/ViewToggle";
import TopologyGraph from "@/components/graph/TopologyGraph";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { GraphData } from "@/lib/types";

/**
 * Detail "Topologia": the asset's subgraph beside its RelationsCard; phones
 * switch between the two. The graph endpoint takes a moment, so it shows
 * loading — never "no connections" while the list beside it has some.
 */
export default function TopologyPane({ graph, loading, relations, hasRelations, t }: {
  graph: GraphData;
  loading: boolean;
  relations: ReactNode;
  hasRelations: boolean;
  t: (k: string) => string;
}) {
  const isMobile = useMediaQuery("(max-width: 767px)");
  const [mobileView, setMobileView] = useState<"list" | "graph">("list");

  if (!hasRelations && !loading && graph.nodes.length === 0) {
    return <EmptyState icon="topology" title={t("host.noTopology")} description={t("topology.noRelations")} compact />;
  }

  const graphBox = (
    <div className="rounded-[var(--radius-lg)] overflow-hidden border border-[var(--border-subtle)] bg-[var(--bg-surface)] h-full min-h-[300px] flex items-center justify-center">
      {loading ? (
        <span className="inline-flex items-center gap-2 text-sm text-[var(--text-muted)]"><Spinner />{t("common.loading")}</span>
      ) : graph.nodes.length > 0 ? (
        <TopologyGraph data={graph} className="w-full h-full" />
      ) : (
        <EmptyState icon="topology" title={t("host.noTopology")} description={t("topology.noRelations")} compact />
      )}
    </div>
  );

  if (isMobile) {
    return (
      <div className="animate-fade-in space-y-4">
        <ViewToggle
          value={mobileView}
          onChange={(v) => setMobileView(v as "list" | "graph")}
          options={[
            { key: "list", label: t("common.list"), icon: ICON_PATHS.viewTable },
            { key: "graph", label: t("common.graph"), icon: ICON_PATHS.bolt },
          ]}
        />
        {mobileView === "list" ? relations : <div className="h-[60vh]">{graphBox}</div>}
      </div>
    );
  }

  return (
    <div className="animate-fade-in grid grid-cols-[minmax(0,1fr)_340px] gap-5 h-[calc(100vh-16rem)]">
      {graphBox}
      <div className="overflow-y-auto min-h-0">{relations}</div>
    </div>
  );
}
