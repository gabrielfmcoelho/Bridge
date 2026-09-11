"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { situacaoAccent } from "@/lib/constants";
import Badge from "@/components/ui/Badge";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { DNSRecord } from "@/lib/types";

export default function DnsCard({ dns }: { dns: DNSRecord }) {
  const { t } = useLocale();
  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const situacaoColor = situacoes.find((s) => s.value === dns.situacao)?.color;
  const linkedHostsCount = dns.host_ids?.length || 0;
  const mainResp = dns.main_responsavel_name || dns.responsavel || "-";

  return (
    <Link href={`/dns/${dns.id}`}>
      <Card accent={situacaoAccent(dns.situacao, situacaoColor)} className="h-full flex flex-col overflow-hidden" clickIndicator="link">
        <CardHeader
          title={dns.domain}
          description={dns.observacoes || t("common.noDescription")}
          badge={
            <Badge variant="situacao" situacao={dns.situacao} compact>
              {dns.situacao}
            </Badge>
          }
        />

        <CardMetadataGrid
          items={[
            { label: t("dns.responsavel"), value: mainResp },
            { label: t("host.entity") || "Entidade", value: "-" },
          ]}
        />

        <CardTagsSection tags={dns.tags} />

        <div className="flex-1 min-h-3" />

        {/* Bottom indicators — all icons always visible (faint when 0), like hosts */}
        <div className="flex items-center gap-3 pt-3 border-t border-[var(--border-subtle)]">
          <CardIndicator icon={ICON_PATHS.lock} count={dns.has_https ? 1 : 0} color="emerald" title={dns.has_https ? t("topology.https") : t("topology.noHttps")} hideCount />
          <CardIndicator icon={ICON_PATHS.server} count={linkedHostsCount} color="cyan" title={t("dns.hostCount", { count: String(linkedHostsCount) })} />
          <CardIndicator icon={ICON_PATHS.gear} count={0} color="amber" title={`0 ${t("host.services").toLowerCase()}`} />
          <CardIndicator icon={ICON_PATHS.folder} count={0} color="violet" title={`0 ${t("host.linkedProjects").toLowerCase()}`} />
          <CardIndicatorSeparator />
          <CardIndicator icon={ICON_PATHS.alert} count={0} color="amber" title={`0 ${t("host.alerts").toLowerCase()}`} />
          <CardIndicator icon={ICON_PATHS.clipboard} count={0} color="purple" title={`0 ${t("issue.title").toLowerCase()}`} />
          <CardIndicator icon={ICON_PATHS.document} count={0} color="orange" title="0 chamados" />
        </div>
      </Card>
    </Link>
  );
}
