"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { situacaoAccent } from "@/lib/constants";
import SituacaoText from "@/components/ui/SituacaoText";
import { useHostNames } from "@/hooks/useHostNames";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import { certState, certTone } from "@/lib/dnsCert";
import { certLabel } from "./CertBadge";
import type { DNSRecord } from "@/lib/types";

export default function DnsCard({ dns }: { dns: DNSRecord }) {
  const { t } = useLocale();
  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const situacaoColor = situacoes.find((s) => s.value === dns.situacao)?.color;
  const linkedHostsCount = dns.host_ids?.length || 0;
  const mainResp = dns.main_responsavel_name || dns.responsavel || "";
  // Subtitle: where the domain points — its first linked host, "+N" for more.
  const hostNames = useHostNames();
  const firstHost = dns.host_ids?.[0] != null ? hostNames.get(dns.host_ids[0]) : undefined;
  const linkedHost = firstHost ? `${firstHost}${linkedHostsCount > 1 ? ` +${linkedHostsCount - 1}` : ""}` : undefined;
  const cert = certState(dns);
  const scanned = cert !== "none" && cert !== "unscanned";

  return (
    <Link href={`/dns/${dns.id}`} className="block h-full">
      <Card accent={situacaoAccent(dns.situacao, situacaoColor)} className="h-full flex flex-col overflow-hidden">
        {/* Fixed anatomy: every slot renders, "–"/0/dimmed when empty. */}
        <CardHeader
          title={dns.domain}
          subtitle={linkedHost}
          subtitleFont="display"
          status={<SituacaoText situacao={dns.situacao} />}
          description={dns.observacoes}
        />

        <CardMetadataGrid
          items={[
            { label: t("dns.responsavel"), value: mainResp },
            { label: t("host.entity"), value: dns.main_entidade || "" },
            { label: t("dns.certificate"), value: certLabel(dns, t) },
            { label: "HTTPS", value: dns.has_https ? t("common.yes") : t("common.no") },
          ]}
        />

        <CardTagsSection tags={dns.tags} />

        {/* Bottom indicators — all icons always visible (faint when 0), like hosts */}
        <div className="flex items-center gap-3 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4">
          <CardIndicator
            icon={ICON_PATHS.lock}
            count={dns.has_https || scanned ? 1 : 0}
            color={scanned ? certTone(cert) : "success"}
            title={scanned ? certLabel(dns, t) : dns.has_https ? t("topology.https") : t("topology.noHttps")}
            hideCount
          />
          <CardIndicator icon={ICON_PATHS.server} count={linkedHostsCount} color="cyan" title={t("dns.hostCount", { count: String(linkedHostsCount) })} />
          {/* The DNS list doesn't send these counts yet: shown dimmed, never a fake 0. */}
          <CardIndicator icon={ICON_PATHS.gear} disabled color="warning" title={t("host.services")} />
          <CardIndicator icon={ICON_PATHS.folder} disabled color="accent" title={t("host.linkedProjects")} />
          <CardIndicatorSeparator />
          <CardIndicator icon={ICON_PATHS.alert} disabled color="danger" title={t("host.alerts")} />
          <CardIndicator icon={ICON_PATHS.clipboard} disabled color="accent" title={t("issue.title")} />
          <CardIndicator icon={ICON_PATHS.document} disabled color="warning" title={t("nav.chamados")} />
        </div>
      </Card>
    </Link>
  );
}
