"use client";

import SituacaoText from "@/components/ui/SituacaoText";
import { CardIndicator } from "@/components/inventory";

import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { hostsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Panel from "@/components/ui/Panel";
import Button from "@/components/ui/Button";
import CopyButton from "@/components/ui/CopyButton";
import Field from "@/components/ui/Field";
import Tag from "@/components/ui/Tag";
import Icon from "@/components/ui/Icon";
import SectionHeading from "@/components/ui/SectionHeading";
import { Skeleton } from "@/components/ui/Skeleton";
import { ICON_PATHS } from "@/lib/icon-paths";
import { formatSize } from "@/lib/units";
import { getTimeAgo, hasPermissionDeniedMessage } from "@/lib/utils";
import type { Host } from "@/lib/types";
import { closeQuickLook, useIsQuickLookRenderer, useQuickLook } from "./quickLookStore";

/** Drop one anywhere a quick look can be opened; only the elected one renders.
 *  The Panel stays mounted (closed) from the start, so every open is a
 *  transition: it slides in and takes focus. */
export function QuickLookOutlet() {
  const { t } = useLocale();
  const router = useRouter();
  const isRenderer = useIsQuickLookRenderer();
  const { host, open } = useQuickLook();
  if (!isRenderer) return null;
  return (
    <Panel
      open={open && !!host}
      onClose={closeQuickLook}
      title={host?.nickname ?? ""}
      subtitle={host && <span className="font-mono">{host.oficial_slug}</span>}
      footer={
        host && (
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => router.push(`/hosts/${host.oficial_slug}`)}>
              {t("host.openPage")}
              <Icon path={ICON_PATHS.arrowRight} />
            </Button>
            {host.hostname && <CopyButton value={host.hostname} label={t("host.copyHostname")} icon />}
          </div>
        )
      }
    >
      {host && <HostQuickLook host={host} open={open} />}
    </Panel>
  );
}

/** A scan value worth showing: present and not a captured shell error. */
const clean = (v?: string) => (v && v.trim() && !hasPermissionDeniedMessage(v) ? formatSize(v) : "");

/** The panel body for one host. */
export default function HostQuickLook({ host, open }: { host: Host; open: boolean }) {
  const { t, locale } = useLocale();
  const slug = host.oficial_slug;
  // Same key and call as HostDetail, so opening the page afterwards is instant.
  const { data, isLoading } = useQuery({
    queryKey: ["host", slug],
    queryFn: () => hostsAPI.get(slug),
    enabled: open,
  });

  // The list row paints the header at once; the detail fills in what it lacks.
  const h = data?.host ?? host;
  const tags = data?.tags ?? host.tags ?? [];
  const mainResp = (data?.responsaveis ?? host.responsaveis)?.find((r) => r.is_main)?.name || host.main_responsavel_name;
  const sr = host.scan_resources;
  const cpu = clean(sr?.cpu) || h.recurso_cpu;
  const ram = clean(sr?.ram) || h.recurso_ram;
  const disk = clean(sr?.storage) || h.recurso_armazenamento;

  const counts = [
    { icon: ICON_PATHS.globe, label: "DNS", n: data?.dns_records?.length ?? host.dns_count },
    { icon: ICON_PATHS.gear, label: t("host.services"), n: data?.services?.length ?? host.services_count },
    { icon: ICON_PATHS.folder, label: t("nav.projects"), n: data?.projects?.length ?? host.projects_count },
    { icon: ICON_PATHS.container, label: t("host.containers"), n: host.containers_count },
    { icon: ICON_PATHS.alert, label: t("host.alerts"), n: host.alerts?.length },
    { icon: ICON_PATHS.document, label: t("nav.chamados"), n: data?.chamados?.length ?? host.chamados_count },
    { icon: ICON_PATHS.clipboard, label: t("nav.issues"), n: host.issues_count },
  ].filter((c) => (c.n ?? 0) > 0);

  return (
    <div className="space-y-5 text-sm">
      <div className="flex flex-wrap items-center gap-3">
        <SituacaoText situacao={h.situacao} />
        <CardIndicator
          icon={ICON_PATHS.moon}
          count={host.idle ? 1 : 0}
          color="info"
          hideCount
          disabled={!host.has_scan}
          title={host.idle ? `${t("host.idle")}: ${(host.idle_reasons ?? []).join("; ") || t("host.idleNoWorkloads")}` : `${t("host.idle")}: ${t("host.idleNotFlaggedShort")}`}
        />
      </div>

      <div className="grid grid-cols-2 gap-x-4 gap-y-3">
        {h.hostname && <Field className="col-span-2" label={t("host.hostname")} value={h.hostname} mono />}
        {h.hospedagem && <Field label={t("host.hospedagem")} value={h.hospedagem} />}
        {host.main_entidade && <Field label={t("host.entity")} value={host.main_entidade} />}
        {mainResp && <Field className="col-span-2" label={t("dns.responsavel")} value={mainResp} />}
      </div>

      {(cpu || ram || disk) && (
        <section className="space-y-2">
          <SectionHeading as="h3">{t("host.resources")}</SectionHeading>
          <div className="grid grid-cols-3 gap-3">
            {cpu && <Field label="CPU" value={cpu} mono />}
            {ram && <Field label="RAM" value={ram} mono />}
            {disk && <Field label={t("vm.disk")} value={disk} mono />}
          </div>
        </section>
      )}

      <section className="space-y-2">
        <SectionHeading as="h3">{t("host.quickLookLinked")}</SectionHeading>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-32" />
          </div>
        ) : counts.length > 0 ? (
          <ul className="grid grid-cols-2 gap-x-4 gap-y-1.5">
            {counts.map((c) => (
              <li key={c.label} className="flex items-center gap-2 text-[var(--text-secondary)]">
                <Icon path={c.icon} className="w-3.5 h-3.5 text-[var(--text-muted)]" />
                <span className="truncate">{c.label}</span>
                <span className="ml-auto font-mono tabular-nums text-[var(--text-primary)]">{c.n}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-[var(--text-muted)]">{t("host.quickLookNoLinks")}</p>
        )}
      </section>

      {tags.length > 0 && (
        <section className="space-y-2">
          <SectionHeading as="h3">{t("common.tags")}</SectionHeading>
          <div className="flex flex-wrap gap-1.5">
            {tags.map((tag) => <Tag key={tag}>{tag}</Tag>)}
          </div>
        </section>
      )}

      <p className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
        <Icon path={ICON_PATHS.clock} className="w-3.5 h-3.5" />
        {host.last_scan_at ? (
          <>
            {t("host.quickLookLastScan")}{" "}
            <time dateTime={host.last_scan_at} className="font-mono">{getTimeAgo(host.last_scan_at, locale)}</time>
          </>
        ) : (
          t("host.kpi.noScan")
        )}
      </p>
    </div>
  );
}
