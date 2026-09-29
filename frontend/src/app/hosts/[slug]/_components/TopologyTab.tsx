"use client";

import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { dnsGroup, projectsGroup, servicesGroup, type RelationGroup } from "@/components/detail/RelationsCard";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { GraphData } from "@/lib/types";

export default function TopologyTab({ data, filteredGraph, graphLoading, t }: {
  data: { orchestrator?: { type: string; version: string } | null; dns_records?: { id: number; domain: string; has_https: boolean; situacao: string }[]; services?: { id: number; nickname: string; technology_stack?: string }[]; projects?: { id: number; name: string; situacao?: string }[] };
  filteredGraph: GraphData;
  graphLoading: boolean;
  t: (k: string) => string;
}) {
  const orchestrator: RelationGroup = {
    title: t("topology.orchestrator"),
    rows: data.orchestrator ? [{
      key: "orch", icon: ICON_PATHS.cube, tone: "var(--text-muted)", label: data.orchestrator.type, mono: true,
      trailing: <span className="text-2xs text-[var(--text-muted)] font-mono">{data.orchestrator.version}</span>,
    }] : [],
  };
  const groups = [orchestrator, servicesGroup(data.services ?? [], t), dnsGroup(data.dns_records ?? [], t), projectsGroup(data.projects ?? [], t)];
  return (
    <TopologyPane
      graph={filteredGraph}
      loading={graphLoading}
      hasRelations={groups.some((g) => g.rows.length > 0)}
      relations={<RelationsCard groups={groups} t={t} />}
      t={t}
    />
  );
}
