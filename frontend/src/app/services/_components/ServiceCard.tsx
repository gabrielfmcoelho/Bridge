"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { StatusText } from "@/components/ui/SituacaoText";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorGrid } from "@/components/inventory";
import { useHostNames } from "@/hooks/useHostNames";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";
import { serviceTitle, originKey } from "@/lib/serviceDisplay";
import type { Service } from "@/lib/types";

/** Scan-written descriptions ("Auto-discovered from container x") repeat the
 *  origin; they don't earn the description line. */
const GENERATED = /^Auto-discovered /;

export default function ServiceCard({ svc }: { svc: Service }) {
  const { t, locale } = useLocale();
  const hostNames = useHostNames();
  const { title, mono, id } = serviceTitle(svc);
  const hosts = svc.host_ids ?? [];
  const firstHost = hosts[0] != null ? hostNames.get(hosts[0]) : undefined;
  const where = firstHost ? `${firstHost}${hosts.length > 1 ? ` +${hosts.length - 1}` : ""}` : undefined;
  const online = svc.container_status === "online";
  // Stripe = runtime state, as situação is on hosts; external dependencies keep amber.
  const accent = svc.is_external_dependency ? "warning" : online ? "success" : svc.container_status === "offline" ? "danger" : "muted";
  const description = svc.description && !GENERATED.test(svc.description) ? svc.description : id;

  return (
    <Link href={`/services/${svc.id}`} className="block h-full">
      <Card accent={accent} className="h-full flex flex-col overflow-hidden">
        {/* Fixed anatomy: every slot renders, "–" when empty. */}
        <CardHeader
          titleFont={mono ? "mono" : "display"}
          title={title}
          subtitle={where}
          subtitleFont="display"
          status={
            svc.container_status ? (
              <span className="inline-flex items-center gap-2 min-w-0">
                <StatusText
                  color={online ? "var(--success)" : "var(--text-muted)"}
                  on={online}
                  label={t(`service.status.${svc.container_status}`)}
                />
                {svc.last_seen_at && (
                  <span className="text-2xs text-[var(--text-muted)] truncate" title={svc.last_seen_at}>
                    {t("service.seenAgo", { ago: getTimeAgo(svc.last_seen_at, locale) })}
                  </span>
                )}
              </span>
            ) : undefined
          }
          description={description}
        />

        <CardMetadataGrid
          items={[
            { label: t("service.category"), value: svc.service_kind ? t(`service.kind.${svc.service_kind}`) : "" },
            { label: t("service.originTitle"), value: t(originKey(svc)) },
            { label: svc.port ? t("service.port") : t("service.version"), value: svc.port || svc.version || "", mono: true },
            { label: t("dns.responsavel"), value: svc.main_responsavel_name || "" },
          ]}
        />

        <CardTagsSection tags={svc.tags} />

        {/* Links on one fixed row; external is a flag, not a count. */}
        <CardIndicatorGrid>
          <CardIndicator icon={ICON_PATHS.server} count={hosts.length} color="info" title={t("service.hostsCount", { count: String(hosts.length) })} />
          <CardIndicator icon={ICON_PATHS.globe} count={svc.dns_ids?.length || 0} color="success" title={t("service.dnsCount", { count: String(svc.dns_ids?.length || 0) })} />
          <CardIndicator icon={ICON_PATHS.link} count={svc.depends_on_ids?.length || 0} color="warning" title={t("service.depsCount", { count: String(svc.depends_on_ids?.length || 0) })} />
          <CardIndicator icon={ICON_PATHS.folder} count={svc.project_id ? 1 : 0} color="accent" hideCount title={svc.project_id ? t("service.inProject") : t("service.noProject")} />
          <CardIndicator icon={ICON_PATHS.clipboard} count={svc.issues_count || 0} color="accent" title={`${svc.issues_count || 0} ${t("nav.issues").toLowerCase()}`} />
          <CardIndicator icon={ICON_PATHS.alert} count={svc.is_external_dependency ? 1 : 0} color="warning" hideCount title={svc.is_external_dependency ? t("service.isExternalDependency") : t("service.notExternalDependency")} />
        </CardIndicatorGrid>
      </Card>
    </Link>
  );
}
