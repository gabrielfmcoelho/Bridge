"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
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
  const fallbackColor = dns.situacao === "active" ? "#10b981" : dns.situacao === "maintenance" ? "#f59e0b" : "#6b7280";
  const linkedHostsCount = dns.host_ids?.length || 0;
  const mainResp = dns.main_responsavel_name || dns.responsavel || "-";

  return (
    <Link href={`/dns/${dns.id}`}>
      <Card className="h-full border-l-[3px] flex flex-col overflow-hidden" style={{ borderLeftColor: situacaoColor || fallbackColor }} clickIndicator="link">
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
          <CardIndicator icon={ICON_PATHS.lock} count={dns.has_https ? 1 : 0} color="emerald" title={dns.has_https ? "HTTPS" : "No HTTPS"} hideCount />
          <CardIndicator icon={ICON_PATHS.server} count={linkedHostsCount} color="cyan" title={`${linkedHostsCount} host(s)`} />
          <CardIndicator icon={ICON_PATHS.gear} count={0} color="amber" title="0 services" />
          <CardIndicator icon={ICON_PATHS.folder} count={0} color="violet" title="0 projects" />
          <CardIndicatorSeparator />
          <CardIndicator icon={ICON_PATHS.alert} count={0} color="amber" title="0 alerts" />
          <CardIndicator icon={ICON_PATHS.clipboard} count={0} color="purple" title="0 issues" />
          <CardIndicator icon={ICON_PATHS.document} count={0} color="orange" title="0 chamados" />
        </div>
      </Card>
    </Link>
  );
}
