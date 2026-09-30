"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { situacaoAccent } from "@/lib/constants";
import { useSituacao } from "@/hooks/useSituacao";
import SituacaoText from "@/components/ui/SituacaoText";
import { useHostNames } from "@/hooks/useHostNames";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorGrid, CardMeter } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import { certState, certTone, certDaysLeft, certValidityPct } from "@/lib/dnsCert";
import { certLabel } from "./CertBadge";
import type { DNSRecord } from "@/lib/types";

export default function DnsCard({ dns }: { dns: DNSRecord }) {
  const { t, formatDate } = useLocale();
  const { roleOf, colorOf } = useSituacao();
  const situacaoColor = colorOf(dns.situacao);
  const linkedHostsCount = dns.host_ids?.length || 0;
  const mainResp = dns.main_responsavel_name || "";
  // Subtitle: where the domain points — its first linked host, "+N" for more.
  const hostNames = useHostNames();
  const firstHost = dns.host_ids?.[0] != null ? hostNames.get(dns.host_ids[0]) : undefined;
  const linkedHost = firstHost ? `${firstHost}${linkedHostsCount > 1 ? ` +${linkedHostsCount - 1}` : ""}` : undefined;
  const cert = certState(dns);
  const scanned = cert !== "none" && cert !== "unscanned";
  const tone = certTone(cert);
  const daysLeft = dns.cert_expires_at ? certDaysLeft(dns.cert_expires_at) : null;
  const validity = certValidityPct(dns);

  return (
    <Link href={`/dns/${dns.id}`} className="block h-full">
      <Card accent={situacaoAccent(roleOf(dns.situacao), situacaoColor)} className="h-full flex flex-col overflow-hidden">
        {/* Fixed anatomy: every slot renders, "–"/0/dimmed when empty. */}
        <CardHeader
          title={dns.domain}
          subtitle={linkedHost}
          titleFont="mono"
          subtitleFont="display"
          status={<SituacaoText situacao={dns.situacao} />}
          description={dns.observacoes}
        />

        <CardMetadataGrid
          items={[
            { label: t("dns.responsavel"), value: mainResp },
            { label: t("host.entity"), value: dns.main_entidade || "" },
            { label: t("dns.certificate"), value: certLabel(dns, t) },
            { label: t("dns.certIssuer"), value: dns.cert_issuer || "" },
          ]}
        />

        <CardTagsSection tags={dns.tags} />

        {/* The domain's own block, as CPU/RAM/disk is the host's: how much of the
            certificate's validity is left. Empty track + "–" until scanned. */}
        <div className="mt-3 pt-3 pb-1 border-t border-[var(--border-subtle)]">
          <CardMeter
            label={t("dns.certValidity")}
            pct={validity}
            reading={daysLeft !== null ? t("dns.daysLeft", { days: String(Math.max(daysLeft, 0)) }) : undefined}
            tone={tone === "danger" ? "danger" : tone === "warning" ? "warning" : "success"}
            caption={dns.cert_expires_at ? formatDate(dns.cert_expires_at) : undefined}
          />
        </div>

        {/* Only what a DNS record has: alerts and chamados are host-only. */}
        <CardIndicatorGrid>
          <CardIndicator
            icon={ICON_PATHS.lock}
            count={dns.has_https || scanned ? 1 : 0}
            color={scanned ? certTone(cert) : "success"}
            title={scanned ? certLabel(dns, t) : dns.has_https ? t("topology.https") : t("topology.noHttps")}
            hideCount
          />
          <CardIndicator icon={ICON_PATHS.server} count={linkedHostsCount} color="info" title={t("dns.hostCount", { count: String(linkedHostsCount) })} />
          <CardIndicator icon={ICON_PATHS.gear} count={dns.services_count || 0} color="warning" title={`${dns.services_count || 0} ${t("host.services").toLowerCase()}`} />
          <CardIndicator icon={ICON_PATHS.folder} count={dns.projects_count || 0} color="accent" title={`${dns.projects_count || 0} ${t("nav.projects").toLowerCase()}`} />
          <CardIndicator icon={ICON_PATHS.clipboard} count={dns.issues_count || 0} color="accent" title={`${dns.issues_count || 0} ${t("nav.issues").toLowerCase()}`} />
        </CardIndicatorGrid>
      </Card>
    </Link>
  );
}
