"use client";

import { useState, type ReactNode } from "react";
import Link from "next/link";
import SituacaoText from "@/components/ui/SituacaoText";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import SectionCard from "@/components/ui/SectionCard";
import Spinner from "@/components/ui/Spinner";
import ViewToggle from "@/components/ui/ViewToggle";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import TopologyGraph from "@/components/graph/TopologyGraph";
import type { GraphData } from "@/lib/types";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

const rowClass = "flex items-center gap-3 px-5 py-2.5 hover:bg-[var(--bg-elevated)] transition-colors";

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <h4 className="px-5 pt-4 pb-1 text-xs font-medium text-[var(--text-faint)]">{title}</h4>
      <div className="divide-y divide-[var(--border-subtle)]">{children}</div>
    </div>
  );
}

export default function TopologyTab({ data, filteredGraph, graphLoading, t }: {
  data: { orchestrator?: { type: string; version: string } | null; dns_records?: { id: number; domain: string; has_https: boolean; situacao: string }[]; services?: { id: number; nickname: string; technology_stack?: string }[]; projects?: { id: number; name: string; situacao?: string }[] };
  filteredGraph: GraphData;
  graphLoading: boolean;
  t: (k: string) => string;
}) {
  const isMobile = useMediaQuery("(max-width: 767px)");
  const [mobileView, setMobileView] = useState<"list" | "graph">("list");

  const dns = data.dns_records ?? [];
  const services = data.services ?? [];
  const projects = data.projects ?? [];
  const relationCount = dns.length + services.length + projects.length + (data.orchestrator ? 1 : 0);

  if (relationCount === 0 && !graphLoading && filteredGraph.nodes.length === 0) {
    return <EmptyState icon="topology" title={t("host.noTopology")} description={t("host.noTopologyDesc")} compact />;
  }

  // Same shell as the other tabs: one card, grouped flush rows (as in Operações).
  const relations = (
    <SectionCard as="h3" title={t("topology.relations")} count={relationCount} body="flush" empty={relationCount === 0 ? t("host.noTopologyDesc") : undefined}>
      {data.orchestrator && (
        <Group title={t("topology.orchestrator")}>
          <div className="flex items-center gap-3 px-5 py-2.5 text-sm">
            <span className="text-[var(--text-primary)] font-mono">{data.orchestrator.type}</span>
            <span className="ml-auto text-2xs text-[var(--text-muted)] font-mono">{data.orchestrator.version}</span>
          </div>
        </Group>
      )}
      {services.length > 0 && (
        <Group title={t("topology.services")}>
          {services.map((svc) => (
            <Link key={svc.id} href={`/services/${svc.id}`} className={rowClass}>
              <Icon path={ICON_PATHS.serverStack} className="w-3.5 h-3.5 shrink-0 text-[var(--accent)]" />
              <span className="text-sm text-[var(--text-primary)] truncate flex-1">{svc.nickname}</span>
              {svc.technology_stack && <Badge>{svc.technology_stack}</Badge>}
            </Link>
          ))}
        </Group>
      )}
      {dns.length > 0 && (
        <Group title={t("topology.dnsRecords")}>
          {dns.map((d) => (
            <Link key={d.id} href={`/dns/${d.id}`} className={rowClass}>
              <Icon path={ICON_PATHS.globeMeridian} className="w-3.5 h-3.5 shrink-0 text-[var(--success)]" />
              <span className="text-sm text-[var(--text-primary)] truncate flex-1 font-mono">{d.domain}</span>
              <span className={d.has_https ? "text-[var(--success)]" : "text-[var(--text-faint)]/40"} title={d.has_https ? t("topology.https") : t("topology.noHttps")}>
                <Icon path={ICON_PATHS.lock} className="w-3.5 h-3.5" />
              </span>
              <SituacaoText situacao={d.situacao} />
            </Link>
          ))}
        </Group>
      )}
      {projects.length > 0 && (
        <Group title={t("topology.projects")}>
          {projects.map((p) => (
            <Link key={p.id} href={`/projects/${p.id}`} className={rowClass}>
              <Icon path={ICON_PATHS.folder} className="w-3.5 h-3.5 shrink-0 text-[var(--warning)]" />
              <span className="text-sm text-[var(--text-primary)] truncate flex-1">{p.name}</span>
              <SituacaoText situacao={p.situacao} />
            </Link>
          ))}
        </Group>
      )}
    </SectionCard>
  );

  // The graph endpoint builds the whole inventory and takes a few seconds:
  // show it loading, never "no connections" while the list beside it has some.
  const graph = (
    <div className="rounded-[var(--radius-lg)] overflow-hidden border border-[var(--border-subtle)] bg-[var(--bg-surface)] h-full min-h-[300px] flex items-center justify-center">
      {graphLoading ? (
        <span className="inline-flex items-center gap-2 text-sm text-[var(--text-muted)]"><Spinner />{t("common.loading")}</span>
      ) : filteredGraph.nodes.length > 0 ? (
        <TopologyGraph data={filteredGraph} className="w-full h-full" />
      ) : (
        <EmptyState icon="topology" title={t("host.noTopology")} description={t("host.noTopologyDesc")} compact />
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
        {mobileView === "list" ? relations : <div className="h-[60vh]">{graph}</div>}
      </div>
    );
  }

  return (
    <div className="animate-fade-in grid grid-cols-[minmax(0,1fr)_340px] gap-5 h-[calc(100vh-16rem)]">
      {graph}
      <div className="overflow-y-auto min-h-0">{relations}</div>
    </div>
  );
}
