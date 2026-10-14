"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { sentinelAPI, type SentinelMetricsRange, type SentinelTrafficRange } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import PillButton from "@/components/ui/PillButton";
import StatusAlert from "@/components/ui/StatusAlert";
import EmptyState from "@/components/ui/EmptyState";
import SectionHeading from "@/components/ui/SectionHeading";
import NativeSelect from "@/components/ui/NativeSelect";
import LineChart, { CHART_COLORS } from "@/components/ui/LineChart";
import { Skeleton } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import { RequestsPanel } from "@/components/telemetry/RequestsTab";
import { pointsToRows, trafficToPanel } from "@/lib/sentinel";
import { SentinelReasonAlert } from "./HostSentinelTab";

const TRAFFIC_RANGES: SentinelTrafficRange[] = ["24h", "7d", "30d"];
const DIMENSIONS = ["status", "country", "browser", "os", "device", "method", "referer", "bot"];

/** Coolify Sentinel on a service: its traffic through Traefik and its container's CPU / memory. */
export default function ServiceSentinelTab({ serviceId, containerPorts }: { serviceId: number; containerPorts?: string }) {
  const { t, locale } = useLocale();
  const [range, setRange] = useState<SentinelTrafficRange>("24h");
  const [dim, setDim] = useState("status");
  const resRange: SentinelMetricsRange = range === "24h" ? "24h" : "7d"; // metrics keep 7 days

  const traffic = useQuery({ queryKey: ["sentinel-traffic", serviceId, range], queryFn: () => sentinelAPI.serviceTraffic(serviceId, range) });
  const hasKeys = !!traffic.data?.data?.keys.length;
  const breakdown = useQuery({
    queryKey: ["sentinel-breakdown", serviceId, dim, range],
    queryFn: () => sentinelAPI.serviceBreakdown(serviceId, dim, range),
    enabled: hasKeys,
  });
  const resources = useQuery({ queryKey: ["sentinel-resources", serviceId, resRange], queryFn: () => sentinelAPI.serviceResources(serviceId, resRange) });

  const num = (n: number) => n.toLocaleString(locale);
  const fmtX = (v: number) => new Date(v).toLocaleString(locale, resRange === "7d" ? { day: "2-digit", month: "2-digit" } : { hour: "2-digit", minute: "2-digit" });
  const fmtTip = (v: number) => new Date(v).toLocaleString(locale, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
  const rangePills = (
    <div className="flex gap-1.5">
      {TRAFFIC_RANGES.map((r) => (
        <PillButton key={r} size="sm" active={range === r} onClick={() => setRange(r)}>{t(`sentinel.range.${r}`)}</PillButton>
      ))}
    </div>
  );
  const tr = traffic.data?.available ? traffic.data.data : undefined;
  const res = resources.data?.available ? resources.data.data : undefined;
  const totalDim = (breakdown.data?.data ?? []).reduce((s, r) => s + r.requests, 0);

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between gap-3">
        <SectionHeading variant="label" as="h3">{t("sentinel.traffic")}</SectionHeading>
        {rangePills}
      </div>

      {traffic.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : traffic.error ? (
        <StatusAlert variant="error">{t("sentinel.loadError")}</StatusAlert>
      ) : traffic.data && !traffic.data.available ? (
        <SentinelReasonAlert reason={traffic.data.reason} detail={traffic.data.detail} />
      ) : tr && tr.keys.length === 0 ? (
        <EmptyState compact icon="globe" title={t("sentinel.noTrafficTitle")} description={t("sentinel.noTrafficHint")} />
      ) : tr ? (
        <>
          {containerPorts?.includes("->") && <StatusAlert variant="info">{t("sentinel.bypassHint", { ports: containerPorts })}</StatusAlert>}
          <RequestsPanel data={trafficToPanel(tr)} groupBy="route" usersLabel={t("sentinel.visitors")} hideRowP99 />
          <Card>
            <SectionHeading
              variant="label"
              as="h3"
              actions={
                <NativeSelect value={dim} onChange={(e) => setDim(e.target.value)} aria-label={t("sentinel.breakdownBy")}>
                  {DIMENSIONS.map((d) => <option key={d} value={d}>{t(`sentinel.dim.${d}`)}</option>)}
                </NativeSelect>
              }
            >
              {t("sentinel.breakdown")}
            </SectionHeading>
            {breakdown.isLoading ? (
              <Skeleton className="h-24 w-full mt-3" />
            ) : (
              <table className={`${tableClasses.compact.table} mt-2`}>
                <tbody>
                  {(breakdown.data?.data ?? []).map((r) => (
                    <tr key={r.value} className={tableClasses.compact.row}>
                      <td className={`${tableClasses.compact.td} font-mono text-xs break-all`}>{r.value || t("telemetry.unknown")}</td>
                      <td className={`${tableClasses.compact.td} w-1/2`}>
                        <div className="h-1.5 rounded-full bg-[var(--accent)]/70" style={{ width: `${totalDim ? (100 * r.requests) / totalDim : 0}%` }} />
                      </td>
                      <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{num(r.requests)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Card>
        </>
      ) : null}

      <Card>
        <SectionHeading variant="label" as="h3" actions={res?.container && <span className="font-mono text-xs text-[var(--text-muted)] break-all">{res.container}</span>}>
          {t("sentinel.containerUsage")}
        </SectionHeading>
        {resources.isLoading ? (
          <Skeleton className="h-44 w-full mt-3" />
        ) : resources.data && !resources.data.available ? (
          <div className="mt-3"><SentinelReasonAlert reason={resources.data.reason} detail={resources.data.detail} /></div>
        ) : res && !res.container ? (
          <p className="mt-3 text-sm text-[var(--text-muted)]">{t("sentinel.noContainer")}</p>
        ) : res ? (
          <div className="mt-3">
            <LineChart
              data={pointsToRows({ cpu: res.cpu, memory: res.memory })}
              xKey="t"
              series={[
                { key: "cpu", label: t("sentinel.cpuCore"), color: CHART_COLORS[0] },
                { key: "memory", label: t("sentinel.memory"), color: CHART_COLORS[1] },
              ]}
              formatX={fmtX}
              formatTooltipX={fmtTip}
              formatY={(v) => `${Math.round(v)}%`}
              ariaLabel={t("sentinel.containerUsage")}
            />
            <p className="mt-2 text-xs text-[var(--text-muted)]">{t("sentinel.cpuCoreHint")}</p>
          </div>
        ) : null}
      </Card>
    </div>
  );
}
