"use client";

import { useQuery } from "@tanstack/react-query";
import { graphAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import PageShell from "@/components/layout/PageShell";
import TopologyGraph from "@/components/graph/TopologyGraph";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";

const legendItems = [
  { type: "host", color: "bg-cyan-500", label: "Hosts" },
  { type: "service", color: "bg-purple-500", label: "Services" },
  { type: "dns", color: "bg-emerald-500", label: "DNS" },
  { type: "project", color: "bg-amber-500", label: "Projects" },
];

export default function TopologyPage() {
  const { t } = useLocale();
  const { data, isLoading } = useQuery({
    queryKey: ["graph"],
    queryFn: graphAPI.get,
  });

  const nodes = Array.isArray(data?.nodes) ? data.nodes : [];

  return (
    <PageShell>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("topology.title")}</h1>
      </div>

      {/* Legend bar */}
      {nodes.length > 0 && (
        <div className="flex gap-2 mb-4 animate-fade-in">
          {legendItems.map((item) => {
            const count = nodes.filter(n => n.type === item.type).length;
            if (count === 0) return null;
            return (
              <div key={item.type} className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)]">
                <span className={`w-2 h-2 rounded-full ${item.color}`} />
                {item.label} ({count})
              </div>
            );
          })}
        </div>
      )}

      {isLoading ? (
        <Skeleton className="w-full h-[calc(100vh-14rem)] rounded-[var(--radius-lg)]" />
      ) : nodes.length > 0 ? (
        <div className="animate-fade-in">
          <TopologyGraph data={data!} />
        </div>
      ) : (
        <EmptyState
          icon="server"
          title={t("common.noResults")}
          description="Add hosts and services to see the topology"
        />
      )}
    </PageShell>
  );
}
