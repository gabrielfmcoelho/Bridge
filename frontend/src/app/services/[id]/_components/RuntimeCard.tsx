"use client";

import Link from "next/link";
import SectionCard from "@/components/ui/SectionCard";
import Field from "@/components/ui/Field";
import { StatusText } from "@/components/ui/SituacaoText";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import type { Host, Service } from "@/lib/types";

/**
 * The observed side of a service: where and how the scan last saw it running.
 * Manual services have no runtime the scan follows — the card says so.
 */
export default function RuntimeCard({ service, hosts }: { service: Service; hosts: Host[] }) {
  const { t, locale, formatDateTime } = useLocale();
  const scanOwned = service.source !== "manual" && !!service.discovery_kind;
  const online = service.container_status === "online";
  return (
    <SectionCard
      as="h3"
      title={t("service.runtimeTitle")}
      description={scanOwned ? t(service.discovery_kind === "container" ? "service.runtimeContainer" : "service.runtimeHost") : undefined}
      empty={scanOwned ? undefined : t("service.runtimeManual")}
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
          {service.container_status && (
            <StatusText color={online ? "var(--success)" : "var(--text-muted)"} on={online} label={t(`service.status.${service.container_status}`)} />
          )}
          {service.last_seen_at && (
            <span className="text-xs text-[var(--text-muted)]" title={formatDateTime(service.last_seen_at)}>
              {t("service.seenAgo", { ago: getTimeAgo(service.last_seen_at, locale) })}
            </span>
          )}
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="sm:col-span-2">
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">{t("nav.hosts")}</span>
            {hosts.length > 0 ? (
              <div className="flex flex-wrap gap-x-3 gap-y-1">
                {hosts.map((h) => (
                  <Link key={h.id} href={`/hosts/${h.oficial_slug}`} className="text-sm text-[var(--accent)] hover:underline">{h.nickname}</Link>
                ))}
              </div>
            ) : <p className="text-sm text-[var(--text-muted)]">–</p>}
          </div>
          {service.discovery_kind === "container" ? (
            <>
              <Field label={t("service.containerName")} value={service.container_name} mono />
              <Field label={t("service.containerId")} value={service.container_id} mono />
              <Field className="sm:col-span-2" label={t("service.containerImage")} value={service.container_image} mono />
              <Field className="sm:col-span-2" label={t("service.containerPorts")} value={service.container_ports} mono />
            </>
          ) : (
            <Field className="sm:col-span-2" label={t("service.catalogName")} value={service.discovery_key} mono />
          )}
          <Field label={t("service.discoveredAt")} value={service.discovered_at ? formatDateTime(service.discovered_at) : ""} />
          <Field label={t("service.lastSeen")} value={service.last_seen_at ? formatDateTime(service.last_seen_at) : ""} />
        </div>
      </div>
    </SectionCard>
  );
}
