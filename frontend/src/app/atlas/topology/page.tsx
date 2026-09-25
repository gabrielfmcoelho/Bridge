"use client";

import { useQuery } from "@tanstack/react-query";
import { graphAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import TopologyGraph from "@/components/graph/TopologyGraph";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";

const legendItems = [
  { type: "host", color: "bg-[var(--cyan)]", label: "Hosts" },
  { type: "service", color: "bg-[var(--accent)]", label: "Services" },
  { type: "dns", color: "bg-[var(--success)]", label: "DNS" },
  { type: "project", color: "bg-[var(--warning)]", label: "Projects" },
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
      <PageHeader
        title={t("topology.title")}
        indicators={nodes.length > 0 ? legendItems.map((item) => {
          const count = nodes.filter(n => n.type === item.type).length;
          if (count === 0) return null;
          return (
            <div key={item.type} className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] animate-fade-in">
              <span className={`w-2 h-2 rounded-full ${item.color}`} />
              {item.label} ({count})
            </div>
          );
        }) : undefined}
      />

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
          description={t("topology.emptyDescription")}
        />
      )}
    </PageShell>
  );
}
