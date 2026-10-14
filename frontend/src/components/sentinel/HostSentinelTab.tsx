"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { sentinelAPI, type SentinelMetricsRange } from "@/lib/api";
import type { SentinelHostView } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import StatCard from "@/components/ui/StatCard";
import Badge from "@/components/ui/Badge";
import StatusDot from "@/components/ui/StatusDot";
import PillButton from "@/components/ui/PillButton";
import StatusAlert from "@/components/ui/StatusAlert";
import SectionHeading from "@/components/ui/SectionHeading";
import LineChart, { CHART_COLORS } from "@/components/ui/LineChart";
import { Skeleton } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import { ICON_PATHS } from "@/lib/icon-paths";
import { pointsToRows, pct } from "@/lib/sentinel";

const RANGES: SentinelMetricsRange[] = ["1h", "24h", "7d"];
const LEVEL_DOT = { critical: "danger", warning: "warning", info: "info" } as const;

/** Why Sentinel can't be read, as a callout (reason + raw detail). */
export function SentinelReasonAlert({ reason, detail }: { reason?: string; detail?: string }) {
  const { t } = useLocale();
  const variant = reason === "metrics_off" || reason === "traffic_off" || reason === "disabled" ? "info" : "warning";
  return (
    <StatusAlert variant={variant}>
      {t(`sentinel.reason.${reason ?? "error"}`)}
      {detail && <span className="block mt-1 font-mono text-xs text-[var(--text-muted)] break-all">{detail}</span>}
    </StatusAlert>
  );
}

/** Coolify Sentinel on a host: live status, the last sync, its alerts, history charts and live containers. */
export default function HostSentinelTab({ slug }: { slug: string }) {
  const { t, locale } = useLocale();
  const [range, setRange] = useState<SentinelMetricsRange>("24h");

  const status = useQuery({ queryKey: ["sentinel-host", slug], queryFn: () => sentinelAPI.hostStatus(slug) });
  const live = status.data?.status;
  const metricsOn = !!live?.available && live.metrics;
  const metrics = useQuery({
    queryKey: ["sentinel-host-metrics", slug, range],
    queryFn: () => sentinelAPI.hostMetrics(slug, range),
    enabled: metricsOn,
  });
  const containers = useQuery({
    queryKey: ["sentinel-host-containers", slug],
    queryFn: () => sentinelAPI.hostContainers(slug),
    enabled: metricsOn,
  });

  if (status.isLoading) return <Skeleton className="h-40 w-full" />;
  if (status.error) return <StatusAlert variant="error">{t("sentinel.loadError")}</StatusAlert>;
  const view = status.data as SentinelHostView;

  const when = (iso?: string | null) =>
    iso ? new Date(iso).toLocaleString(locale, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" }) : "–";
  const m = metrics.data?.available ? metrics.data.data : undefined;
  const span = range === "7d";
  const fmtX = (v: number) => new Date(v).toLocaleString(locale, span ? { day: "2-digit", month: "2-digit" } : { hour: "2-digit", minute: "2-digit" });
  const fmtTip = (v: number) => new Date(v).toLocaleString(locale, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
  const snap = view.snapshot;

  return (
    <div className="space-y-5 animate-fade-in">
      <Card>
        <SectionHeading variant="label" as="h3" actions={live?.version && <span className="text-xs text-[var(--text-muted)] font-mono">Sentinel {live.version}</span>}>
          {t("sentinel.title")}
        </SectionHeading>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {live?.available ? (
            <>
              <Badge color={live.metrics ? "success" : "default"} dot>{t(live.metrics ? "sentinel.metricsOn" : "sentinel.metricsOff")}</Badge>
              <Badge color={live.traffic ? "success" : "default"} dot>{t(live.traffic ? "sentinel.trafficOn" : "sentinel.trafficOff")}</Badge>
              <Badge color={live.push && live.push.consecutive_failures === 0 ? "success" : "warning"} dot>
                {t(live.push && live.push.consecutive_failures === 0 ? "sentinel.pushOk" : "sentinel.pushFailing")}
              </Badge>
            </>
          ) : (
            <div className="w-full"><SentinelReasonAlert reason={live?.reason} detail={live?.detail} /></div>
          )}
        </div>
        <p className="mt-3 text-xs text-[var(--text-muted)]">
          {snap ? t("sentinel.lastSync", { when: when(snap.synced_at) }) : t("sentinel.neverSynced")}
        </p>
      </Card>

      {snap && snap.state === "ok" && snap.metrics_enabled && (
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
          <StatCard label={t("sentinel.cpuAvg")} value={pct(snap.cpu_avg_24h)} icon={ICON_PATHS.bolt} color="accent" />
          <StatCard label={t("sentinel.cpuP95")} value={pct(snap.cpu_p95_24h)} icon={ICON_PATHS.bolt} color="accent" />
          <StatCard label={t("sentinel.memP95")} value={pct(snap.mem_p95_24h)} icon={ICON_PATHS.serverStack} color="cyan" />
          <StatCard label={t("sentinel.disk")} value={pct(snap.disk_pct)} icon={ICON_PATHS.archive} color="info" />
          <StatCard label={t("sentinel.load")} value={snap.load1 == null ? "–" : snap.load1.toFixed(2)} icon={ICON_PATHS.clock} color="info" />
        </div>
      )}

      {view.alerts.length > 0 && (
        <Card>
          <SectionHeading variant="label" as="h3">{t("sentinel.alerts")}</SectionHeading>
          <ul className="mt-2 space-y-1.5">
            {view.alerts.map((a, i) => (
              <li key={`${a.type}-${i}`} className="flex items-start gap-2 text-sm text-[var(--text-secondary)]">
                <StatusDot color={LEVEL_DOT[a.level as keyof typeof LEVEL_DOT] ?? "muted"} size="sm" />
                <span>{a.message}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}

      {metricsOn && (
        <Card>
          <SectionHeading
            variant="label"
            as="h3"
            actions={
              <div className="flex gap-1.5">
                {RANGES.map((r) => (
                  <PillButton key={r} size="sm" active={range === r} onClick={() => setRange(r)}>{t(`sentinel.range.${r}`)}</PillButton>
                ))}
              </div>
            }
          >
            {t("sentinel.usage")}
          </SectionHeading>
          {metrics.isLoading ? (
            <Skeleton className="h-44 w-full mt-3" />
          ) : metrics.data && !metrics.data.available ? (
            <div className="mt-3"><SentinelReasonAlert reason={metrics.data.reason} detail={metrics.data.detail} /></div>
          ) : m ? (
            <div className="mt-3 space-y-5">
              <LineChart
                data={pointsToRows({ cpu: m.cpu, memory: m.memory })}
                xKey="t"
                series={[
                  { key: "cpu", label: t("sentinel.cpu"), color: CHART_COLORS[0] },
                  { key: "memory", label: t("sentinel.memory"), color: CHART_COLORS[1] },
                ]}
                formatX={fmtX}
                formatTooltipX={fmtTip}
                formatY={(v) => `${Math.round(v)}%`}
                ariaLabel={t("sentinel.usage")}
              />
              <LineChart
                data={pointsToRows({ load: m.load })}
                xKey="t"
                series={[{ key: "load", label: t("sentinel.load"), color: CHART_COLORS[2] }]}
                formatX={fmtX}
                formatTooltipX={fmtTip}
                formatY={(v) => v.toFixed(1)}
                height={120}
                ariaLabel={t("sentinel.load")}
              />
            </div>
          ) : null}
        </Card>
      )}

      {metricsOn && (
        <Card>
          <SectionHeading variant="label" as="h3">{t("sentinel.containers")}</SectionHeading>
          {containers.isLoading ? (
            <Skeleton className="h-24 w-full mt-3" />
          ) : containers.data && !containers.data.available ? (
            <div className="mt-3"><SentinelReasonAlert reason={containers.data.reason} detail={containers.data.detail} /></div>
          ) : (
            <div className="overflow-x-auto">
              <table className={`${tableClasses.compact.table} mt-2`}>
                <thead>
                  <tr className={tableClasses.compact.headRow}>
                    <th className={tableClasses.compact.th}>{t("sentinel.container")}</th>
                    <th className={tableClasses.compact.th}>{t("sentinel.state")}</th>
                    <th className={`${tableClasses.compact.th} text-right`}>{t("sentinel.restarts")}</th>
                    <th className={`${tableClasses.compact.th} text-right`} title={t("sentinel.cpuCoreHint")}>{t("sentinel.cpuCore")}</th>
                    <th className={`${tableClasses.compact.th} text-right`}>{t("sentinel.memory")}</th>
                  </tr>
                </thead>
                <tbody>
                  {(containers.data?.data ?? [])
                    .slice()
                    .sort((a, b) => (b.cpu_pct ?? 0) - (a.cpu_pct ?? 0))
                    .map((c) => (
                      <tr key={c.name} className={tableClasses.compact.row}>
                        <td className={`${tableClasses.compact.td} font-mono text-xs break-all`}>
                          {c.service_id ? (
                            <Link href={`/services/${c.service_id}`} className="hover:underline" title={c.name}>
                              <span className="font-sans text-sm text-[var(--text-primary)]">{c.service_name}</span>
                            </Link>
                          ) : (
                            c.name
                          )}
                        </td>
                        <td className={tableClasses.compact.td}>
                          <span className="inline-flex items-center gap-1.5 text-xs">
                            <StatusDot color={c.state !== "running" ? "muted" : c.health === "unhealthy" ? "danger" : "success"} size="xs" />
                            {c.state}{c.health && c.health !== "unknown" ? ` · ${c.health}` : ""}
                          </span>
                        </td>
                        <td className={`${tableClasses.compact.td} text-right tabular-nums ${c.restart_count ? "text-[var(--warning)]" : ""}`}>{c.restart_count}</td>
                        <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{pct(c.cpu_pct, 1)}</td>
                        <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{pct(c.mem_pct, 1)}</td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}
    </div>
  );
}
