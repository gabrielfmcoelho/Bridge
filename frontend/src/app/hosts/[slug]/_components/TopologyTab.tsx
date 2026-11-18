"use client";

import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { appsGroup, dnsGroup, projectsGroup, routesGroups, servicesGroup, type RelationGroup } from "@/components/detail/RelationsCard";
import AppDepLinksButton from "@/components/appdeps/AppDepLinksManager";
import { useAppDepLinks } from "@/hooks/useAppDepLinks";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { GraphData, WebRoute } from "@/lib/types";

export default function TopologyTab({ data, hostId, canEdit, filteredGraph, graphLoading, t }: {
  hostId: number;
  canEdit: boolean;
  data: { orchestrator?: { type: string; version: string } | null; dns_records?: { id: number; domain: string; has_https: boolean; situacao: string }[]; services?: { id: number; nickname: string; technology_stack?: string }[]; projects?: { id: number; name: string; situacao?: string }[]; web_routes?: { outbound: WebRoute[] | null; inbound: WebRoute[] | null } };
  filteredGraph: GraphData;
  graphLoading: boolean;
  t: (k: string) => string;
}) {
  const { links: apps } = useAppDepLinks({ entity_type: "host", entity_id: hostId });
  const orchestrator: RelationGroup = {
    title: t("topology.orchestrator"),
    rows: data.orchestrator ? [{
      key: "orch", icon: ICON_PATHS.cube, tone: "var(--text-muted)", label: data.orchestrator.type, mono: true,
      trailing: <span className="text-2xs text-[var(--text-muted)] font-mono">{data.orchestrator.version}</span>,
    }] : [],
  };
  const groups = [orchestrator, servicesGroup(data.services ?? [], t), dnsGroup(data.dns_records ?? [], t),
    ...routesGroups(data.web_routes?.outbound ?? [], "outbound", t), ...routesGroups(data.web_routes?.inbound ?? [], "inbound", t),
    projectsGroup(data.projects ?? [], t), appsGroup(apps, t)];
  return (
    <TopologyPane
      graph={filteredGraph}
      loading={graphLoading}
      hasRelations={groups.some((g) => g.rows.length > 0)}
      relations={<RelationsCard groups={groups} t={t} controls={canEdit ? <AppDepLinksButton entity={{ type: "host", id: hostId }} /> : undefined} />}
      t={t}
    />
  );
}
