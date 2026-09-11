"use client";

import Link from "next/link";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import TopologyGraph from "@/components/graph/TopologyGraph";
import type { GraphData, Service, Host, DNSRecord } from "@/lib/types";
import SectionHeading from "@/components/ui/SectionHeading";
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
          <Card
            hover={false}
            className="overflow-hidden flex flex-col h-[50vh] lg:h-[calc(100vh-18rem)]"
          >
            <h2
              className="text-sm font-semibold text-[var(--text-secondary)] mb-3 shrink-0 font-display"
            >
              {t("nav.topology")}
            </h2>
            <div className="flex-1 rounded-[var(--radius-md)] overflow-hidden border border-[var(--border-subtle)]">
              <TopologyGraph data={filteredGraph} className="w-full h-full" />
            </div>
          </Card>
        ) : (
          <div className="flex items-center justify-center text-sm text-[var(--text-faint)]">
            {t("service.noTopologyData")}
          </div>
        )}

        {/* Connection lists -- right */}
        <div className="space-y-4">
          {dependsOnServices.length > 0 && (
            <Card hover={false}>
              <SectionHeading variant="section">
                {t("service.dependencies")}
              </SectionHeading>
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
            </Card>
          )}

          {dependentServices.length > 0 && (
            <Card hover={false}>
              <SectionHeading variant="section">
                {t("service.dependents")}
              </SectionHeading>
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
            </Card>
          )}

          {linkedHosts.length > 0 && (
            <Card hover={false}>
              <SectionHeading variant="section">
                {t("service.linkedHosts")}
              </SectionHeading>
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
            </Card>
          )}

          {linkedDns.length > 0 && (
            <Card hover={false}>
              <SectionHeading variant="section">
                {t("service.linkedDns")}
              </SectionHeading>
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
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
