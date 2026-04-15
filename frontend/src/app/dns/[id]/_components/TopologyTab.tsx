"use client";

import Link from "next/link";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import TopologyGraph from "@/components/graph/TopologyGraph";
import type { GraphData, Host, Service } from "@/lib/types";

interface TopologyTabProps {
  filteredGraph: GraphData;
  linkedHosts: Host[];
  linkedServices: Service[];
}

export default function DnsTopologyTab({ filteredGraph, linkedHosts, linkedServices }: TopologyTabProps) {
  if (filteredGraph.nodes.length === 0 && linkedHosts.length === 0 && linkedServices.length === 0) {
    return (
      <p className="text-sm text-[var(--text-faint)] text-center py-8">
        No connections found for this DNS record.
      </p>
    );
  }

  return (
    <div className="animate-fade-in">
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-5">
        {/* Graph */}
        {filteredGraph.nodes.length > 0 ? (
          <Card hover={false} className="overflow-hidden flex flex-col" style={{ height: "calc(100vh - 18rem)" }}>
            <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-3 shrink-0" style={{ fontFamily: "var(--font-display)" }}>
              Topology
            </h2>
            <div className="flex-1 rounded-[var(--radius-md)] overflow-hidden border border-[var(--border-subtle)]">
              <TopologyGraph data={filteredGraph} className="w-full h-full" />
            </div>
          </Card>
        ) : (
          <div className="flex items-center justify-center text-sm text-[var(--text-faint)] py-12">
            No topology data
          </div>
        )}

        {/* Connection lists */}
        <div className="space-y-4">
          {linkedHosts.length > 0 && (
            <Card hover={false}>
              <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-3" style={{ fontFamily: "var(--font-display)" }}>
                Linked Hosts
              </h2>
              <div className="space-y-2">
                {linkedHosts.map((h) => (
                  <Link
                    key={h.id}
                    href={`/hosts/${h.oficial_slug}`}
                    className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
                  >
                    <svg className="w-3.5 h-3.5 shrink-0 text-cyan-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                    </svg>
                    <span className="truncate">{h.nickname || h.oficial_slug}</span>
                    {h.situacao && <Badge variant="situacao" situacao={h.situacao} dot>{h.situacao}</Badge>}
                  </Link>
                ))}
              </div>
            </Card>
          )}

          {linkedServices.length > 0 && (
            <Card hover={false}>
              <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-3" style={{ fontFamily: "var(--font-display)" }}>
                Linked Services
              </h2>
              <div className="space-y-2">
                {linkedServices.map((svc) => (
                  <Link
                    key={svc.id}
                    href={`/services/${svc.id}`}
                    className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
                  >
                    <span className="truncate">{svc.nickname}</span>
                    {svc.technology_stack && <Badge>{svc.technology_stack}</Badge>}
                  </Link>
                ))}
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
