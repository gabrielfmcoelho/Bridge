"use client";

import Link from "next/link";
import Badge from "@/components/ui/Badge";
import TopologyGraph from "@/components/graph/TopologyGraph";
import type { GraphData, Service, Host, DNSRecord } from "@/lib/types";
import SectionCard from "@/components/ui/SectionCard";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface TopologyTabProps {
  filteredGraph: GraphData;
  dependsOnServices: Service[];
  dependentServices: Service[];
  linkedHosts: Host[];
  linkedDns: DNSRecord[];
  t: (key: string) => string;
}

export default function ServiceTopologyTab({
  filteredGraph,
  dependsOnServices,
  dependentServices,
  linkedHosts,
  linkedDns,
  t,
}: TopologyTabProps) {
  const hasConnections =
    filteredGraph.nodes.length > 0 ||
    dependsOnServices.length > 0 ||
    dependentServices.length > 0 ||
    linkedHosts.length > 0 ||
    linkedDns.length > 0;

  if (!hasConnections) {
    return (
      <p className="text-sm text-[var(--text-faint)] text-center py-8">
        {t("service.noConnections")}
      </p>
    );
  }

  return (
    <div className="animate-fade-in">
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-5">
        {/* Graph -- left */}
        {filteredGraph.nodes.length > 0 ? (
          <SectionCard title={t("nav.topology")} body="flush">
            <div className="h-[calc(50vh-3.5rem)] lg:h-[calc(100vh-21.5rem)]">
              <TopologyGraph data={filteredGraph} className="w-full h-full" />
            </div>
          </SectionCard>
        ) : (
          <div className="flex items-center justify-center text-sm text-[var(--text-faint)]">
            {t("service.noTopologyData")}
          </div>
        )}

        {/* Connection lists -- right */}
        <div className="space-y-4">
          {dependsOnServices.length > 0 && (
            <SectionCard title={t("service.dependencies")}>
              <div className="space-y-1">
                {dependsOnServices.map((dep) => (
                  <Link
                    key={dep.id}
                    href={`/services/${dep.id}`}
                    className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
                  >
                    <span className="flex-1 truncate">{dep.nickname}</span>
                    {dep.technology_stack && <Badge>{dep.technology_stack}</Badge>}
                  </Link>
                ))}
              </div>
            </SectionCard>
          )}

          {dependentServices.length > 0 && (
            <SectionCard title={t("service.dependents")}>
              <div className="space-y-1">
                {dependentServices.map((dep) => (
                  <Link
                    key={dep.id}
                    href={`/services/${dep.id}`}
                    className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
                  >
                    <span className="flex-1 truncate">{dep.nickname}</span>
                    {dep.technology_stack && <Badge>{dep.technology_stack}</Badge>}
                  </Link>
                ))}
              </div>
            </SectionCard>
          )}

          {linkedHosts.length > 0 && (
            <SectionCard title={t("service.linkedHosts")}>
              <div className="space-y-1">
                {linkedHosts.map((host) => (
                  <Link
                    key={host.id}
                    href={`/hosts/${host.oficial_slug}`}
                    className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
                  >
                    <span className="flex-1 truncate font-mono">{host.oficial_slug}</span>
                  </Link>
                ))}
              </div>
            </SectionCard>
          )}

          {linkedDns.length > 0 && (
            <SectionCard title={t("service.linkedDns")}>
              <div className="space-y-1">
                {linkedDns.map((dns) => (
                  <Link
                    key={dns.id}
                    href={`/dns/${dns.id}`}
                    className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
                  >
                    <span className="flex-1 truncate font-mono">{dns.domain}</span>
                    {dns.has_https && (
                      <Badge color="emerald">
                        <Icon path={ICON_PATHS.lock} className="w-3 h-3" />
                      </Badge>
                    )}
                  </Link>
                ))}
              </div>
            </SectionCard>
          )}
        </div>
      </div>
    </div>
  );
}
