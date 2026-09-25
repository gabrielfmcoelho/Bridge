"use client";

import Link from "next/link";
import Badge from "@/components/ui/Badge";
import SituacaoText from "@/components/ui/SituacaoText";
import TopologyGraph from "@/components/graph/TopologyGraph";
import type { GraphData, Host, Service } from "@/lib/types";
import SectionCard from "@/components/ui/SectionCard";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

interface TopologyTabProps {
  filteredGraph: GraphData;
  linkedHosts: Host[];
  linkedServices: Service[];
}

export default function DnsTopologyTab({ filteredGraph, linkedHosts, linkedServices }: TopologyTabProps) {
  const { t } = useLocale();
  if (filteredGraph.nodes.length === 0 && linkedHosts.length === 0 && linkedServices.length === 0) {
    return (
      <p className="text-sm text-[var(--text-faint)] text-center py-8">
        {t("dns.noConnections")}
      </p>
    );
  }

  return (
    <div className="animate-fade-in">
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-5">
        {/* Graph */}
        {filteredGraph.nodes.length > 0 ? (
          <SectionCard title={t("host.tabTopology")} body="flush">
            <div className="h-[calc(50vh-3.5rem)] lg:h-[calc(100vh-21.5rem)]">
              <TopologyGraph data={filteredGraph} className="w-full h-full" />
            </div>
          </SectionCard>
        ) : (
          <div className="flex items-center justify-center text-sm text-[var(--text-faint)] py-12">
            {t("dns.noTopologyData")}
          </div>
        )}

        {/* Connection lists */}
        <div className="space-y-4">
          {linkedHosts.length > 0 && (
            <SectionCard title={t("dns.linkedHosts")}>
              <div className="space-y-2">
                {linkedHosts.map((h) => (
                  <Link
                    key={h.id}
                    href={`/hosts/${h.oficial_slug}`}
                    className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
                  >
                    <Icon path={ICON_PATHS.server} className="w-3.5 h-3.5 shrink-0 text-[var(--cyan)]" />
                    <span className="truncate">{h.nickname || h.oficial_slug}</span>
                    {h.situacao && <SituacaoText situacao={h.situacao} />}
                  </Link>
                ))}
              </div>
            </SectionCard>
          )}

          {linkedServices.length > 0 && (
            <SectionCard title={t("dns.linkedServices")}>
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
            </SectionCard>
          )}
        </div>
      </div>
    </div>
  );
}
