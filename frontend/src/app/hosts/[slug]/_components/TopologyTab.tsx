"use client";

import { useState } from "react";
import Link from "next/link";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import Field from "@/components/ui/Field";
import SectionHeading from "@/components/ui/SectionHeading";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import TopologyGraph from "@/components/graph/TopologyGraph";
import type { GraphData } from "@/lib/types";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function TopologyTab({ data, filteredGraph, t }: {
  data: { orchestrator?: { type: string; version: string } | null; dns_records?: { id: number; domain: string; has_https: boolean; situacao: string }[]; services?: { id: number; nickname: string; technology_stack?: string }[]; projects?: { id: number; name: string; situacao?: string }[] };
  filteredGraph: GraphData;
  t: (k: string) => string;
}) {
  const isMobile = useMediaQuery("(max-width: 767px)");
  const [mobileView, setMobileView] = useState<"list" | "graph">("list");

  const hasConnections = filteredGraph.nodes.length > 0 || data.orchestrator || (data.dns_records && data.dns_records.length > 0) || (data.services && data.services.length > 0) || (data.projects && data.projects.length > 0);

  if (!hasConnections) {
    return (
      <EmptyState
        icon="topology"
        title={t("host.noTopology") || "No connections yet"}
        description={t("host.noTopologyDesc") || "Link this host to DNS records, services, or projects to see its topology."}
      />
    );
  }

  const hasListItems = !!data.orchestrator || (data.dns_records && data.dns_records.length > 0) || (data.services && data.services.length > 0) || (data.projects && data.projects.length > 0);

  const connectionList = (
    <div className="space-y-5">
      {!hasListItems && (
        <EmptyState
          icon="topology"
          title={t("host.noTopology") || "No connections yet"}
          description={t("host.noTopologyDesc") || "Link this host to DNS records, services, or projects to see its topology."}
          compact
        />
      )}

      {/* Orchestrator */}
      {data.orchestrator && (
        <>
          <SectionHeading>{t("topology.orchestrator")}</SectionHeading>
          <Card hover={false} className="!p-3">
            <div className="grid grid-cols-2 gap-3 text-xs">
              <div>
                <span className="text-[var(--text-faint)] block mb-0.5">{t("topology.type")}</span>
                <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{data.orchestrator.type}</span>
              </div>
              <div>
                <span className="text-[var(--text-faint)] block mb-0.5">{t("topology.version")}</span>
                <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{data.orchestrator.version}</span>
              </div>
            </div>
          </Card>
        </>
      )}

      {/* DNS Records */}
      {data.dns_records && data.dns_records.length > 0 && (
        <>
          <SectionHeading>{t("topology.dnsRecords")}</SectionHeading>
          <div className="grid grid-cols-1 gap-2">
            {data.dns_records.map((dns) => (
              <Card key={dns.id} hover={false} className="!p-3">
                <div className="flex items-center gap-2 mb-2">
                  <div className="w-8 h-8 rounded-full flex items-center justify-center shrink-0 bg-emerald-500/15 text-emerald-400">
                    <Icon path={ICON_PATHS.globeMeridian} />
                  </div>
                  <span className="text-sm font-medium text-[var(--text-primary)] truncate flex-1" style={{ fontFamily: "var(--font-mono)" }}>{dns.domain}</span>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <span className={`${dns.has_https ? "text-emerald-400" : "text-[var(--text-faint)]/30"}`} title={dns.has_https ? t("topology.https") : t("topology.noHttps")}>
                      <Icon path={ICON_PATHS.lock} className="w-3.5 h-3.5" />
                    </span>
                    <Badge variant="situacao" situacao={dns.situacao} compact>{dns.situacao}</Badge>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </>
      )}

      {/* Services */}
      {data.services && data.services.length > 0 && (
        <>
          <SectionHeading>{t("topology.services")}</SectionHeading>
          <div className="grid grid-cols-1 gap-2">
            {data.services.map((svc) => (
              <Link key={svc.id} href={`/services/${svc.id}`} className="block">
                <Card clickIndicator="link" className="!p-3 !pb-7">
                  <div className="flex items-center gap-2">
                    <div className="w-8 h-8 rounded-full flex items-center justify-center shrink-0 bg-purple-500/15 text-purple-400">
                      <Icon path={ICON_PATHS.serverStack} />
                    </div>
                    <span className="text-sm font-medium text-[var(--text-primary)] truncate flex-1">{svc.nickname}</span>
                    {svc.technology_stack && <Badge>{svc.technology_stack}</Badge>}
                  </div>
                </Card>
              </Link>
            ))}
          </div>
        </>
      )}

      {/* Projects */}
      {data.projects && data.projects.length > 0 && (
        <>
          <SectionHeading>{t("topology.projects") || "Projects"}</SectionHeading>
          <div className="grid grid-cols-1 gap-2">
            {data.projects.map((proj) => (
              <Link key={proj.id} href={`/projects/${proj.id}`} className="block">
                <Card clickIndicator="link" className="!p-3 !pb-7">
                  <div className="flex items-center gap-2">
                    <div className="w-8 h-8 rounded-full flex items-center justify-center shrink-0 bg-amber-500/15 text-amber-400">
                      <Icon path={ICON_PATHS.folder} />
                    </div>
                    <span className="text-sm font-medium text-[var(--text-primary)] truncate flex-1">{proj.name}</span>
                    {proj.situacao && <Badge variant="situacao" situacao={proj.situacao} compact>{proj.situacao}</Badge>}
                  </div>
                </Card>
              </Link>
            ))}
          </div>
        </>
      )}
    </div>
  );

  if (isMobile) {
    return (
      <div className="animate-fade-in space-y-4">
        {/* Mobile toggle: list / graph */}
        <div className="flex gap-1 p-1 rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-subtle)]">
          {(["list", "graph"] as const).map((v) => (
            <button
              key={v}
              onClick={() => setMobileView(v)}
              className={`flex-1 px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] transition duration-150 capitalize ${
                mobileView === v
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              {v === "list" ? t("common.list") || "List" : t("common.graph") || "Graph"}
            </button>
          ))}
        </div>
        {mobileView === "list" ? connectionList : (
          filteredGraph.nodes.length > 0 ? (
            <div className="rounded-[var(--radius-md)] overflow-hidden border border-[var(--border-subtle)]" style={{ height: "60vh" }}>
              <TopologyGraph data={filteredGraph} className="w-full h-full" />
            </div>
          ) : (
            <EmptyState icon="topology" title={t("host.noTopology") || "No connections yet"} description={t("host.noTopologyDesc") || "Link this host to DNS records, services, or projects to see its topology."} />
          )
        )}
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-4 lg:gap-5 h-[60vh] lg:h-[calc(100vh-16rem)]">
        {filteredGraph.nodes.length > 0 ? (
          <div className="rounded-[var(--radius-lg)] overflow-hidden border border-[var(--border-subtle)] h-full min-h-[300px]">
            <TopologyGraph data={filteredGraph} className="w-full h-full" />
          </div>
        ) : (
          <EmptyState icon="topology" title={t("host.noTopology") || "No connections yet"} compact />
        )}
        <div className="overflow-y-auto pr-1 max-h-[40vh] lg:max-h-none">
          {connectionList}
        </div>
      </div>
    </div>
  );
}
