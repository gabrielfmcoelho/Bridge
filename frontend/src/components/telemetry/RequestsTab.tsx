"use client";

import { useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  telemetryAPI,
  type TelemetryFilters,
  type TelemetryGroupBy,
  type TelemetryKind,
  type TelemetryRange,
  type TelemetryRequests,
} from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import StatCard from "@/components/ui/StatCard";
import PillButton from "@/components/ui/PillButton";
import StatusAlert from "@/components/ui/StatusAlert";
import EmptyState from "@/components/ui/EmptyState";
import SectionHeading from "@/components/ui/SectionHeading";
import NativeSelect from "@/components/ui/NativeSelect";
import Tag from "@/components/ui/Tag";
import Icon from "@/components/ui/Icon";
import LineChart, { CHART_COLORS, CHART_OTHER, type LineSeries } from "@/components/ui/LineChart";
import { Skeleton } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import { ICON_PATHS } from "@/lib/icon-paths";
import TraceDrawer from "./TraceDrawer";

const RANGES: TelemetryRange[] = ["1h", "24h", "7d", "30d"];
// The groups that make sense per kind: callers' keys only exist behind the
// gateway (APIs); a host splits by its services.
const GROUPS: Record<TelemetryKind, TelemetryGroupBy[]> = {
  api: ["route", "key", "status"],
  service: ["route", "status"],
  host: ["service", "route", "status"],
};
const OTHER = "__outros__";

type Metric = "count" | "p95";

const ms = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(1)} s` : `${Math.round(v)} ms`);

/** Whether the SigNoz integration is on: pages show their Requisições tab only then. */
export function useTelemetryEnabled(): boolean {
  const { data } = useQuery({ queryKey: ["telemetry-status"], queryFn: telemetryAPI.status, retry: false, staleTime: 5 * 60_000 });
  return data?.enabled === true;
}

/**
 * KPIs, requests over time and the top groups of one telemetry result. Data
 * only — the share page renders it from the redeemed payload, without auth.
 * onPick (optional) turns the top rows into filters.
 */
export function RequestsPanel({
  data,
  groupBy,
  groupControls,
  chartControls,
  onPick,
}: {
  data: Pick<TelemetryRequests, "summary" | "series" | "top" | "series_by_key">;
  groupBy: TelemetryGroupBy;
  groupControls?: ReactNode;
  chartControls?: ReactNode;
  onPick?: (key: string) => void;
}) {
  const { t, locale } = useLocale();
  const [metric, setMetric] = useState<Metric>("count");
  const num = (n: number) => n.toLocaleString(locale);
  const s = data.summary;

  // Rows for the chart: one per slot; one column per series (the total, or each key).
  const { rows, series } = useMemo(() => {
    const value = (p: { count: number; p95: number }) => (metric === "count" ? p.count : Math.round(p.p95));
    if (data.series_by_key?.length) {
      let slot = 0;
      const series: LineSeries[] = data.series_by_key.map((k) => ({
        key: k.key,
        label: k.key === OTHER ? t("telemetry.others") : k.label || k.key || t("telemetry.unknown"),
        // Fixed slot per position in the server's order (top key first); "outros" stays neutral.
        color: k.key === OTHER ? CHART_OTHER : CHART_COLORS[slot++] ?? CHART_OTHER,
      }));
      const rows = (data.series_by_key[0]?.points ?? []).map((p, i) => {
        const row: Record<string, number> = { t: Date.parse(p.t) };
        for (const k of data.series_by_key!) row[k.key] = value(k.points[i] ?? { count: 0, p95: 0 });
        return row;
      });
      return { rows, series };
    }
    return {
      rows: (data.series ?? []).map((p) => ({ t: Date.parse(p.t), total: value(p) })),
      series: [{ key: "total", label: metric === "count" ? t("telemetry.requests") : "p95", color: CHART_COLORS[0] }],
    };
  }, [data, metric, t]);

  if (!s || s.count === 0) {
    return <EmptyState icon="search" title={t("telemetry.emptyTitle")} description={t("telemetry.emptyHint")} />;
  }
  const errPct = (100 * s.errors) / s.count;
  // Ticks by hour within two days, by date beyond.
  const span = rows.length > 1 ? rows[rows.length - 1].t - rows[0].t : 0;
  const fmtX = (v: number) =>
    new Date(v).toLocaleString(locale, span > 2 * 86_400_000 ? { day: "2-digit", month: "2-digit" } : { hour: "2-digit", minute: "2-digit" });
  const fmtTip = (v: number) => new Date(v).toLocaleString(locale, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
        <StatCard label={t("telemetry.requests")} value={num(s.count)} icon={ICON_PATHS.bolt} color="accent" />
        <StatCard
          label={t("telemetry.errors")}
          value={`${errPct.toFixed(errPct < 10 ? 1 : 0)}%`}
          hint={num(s.errors)}
          icon={ICON_PATHS.alert}
          color={errPct >= 5 ? "danger" : errPct >= 1 ? "warning" : "success"}
        />
        <StatCard label="p50" value={ms(s.p50)} icon={ICON_PATHS.clock} color="info" />
        <StatCard label="p95" value={ms(s.p95)} icon={ICON_PATHS.clock} color="info" />
        <StatCard label="p99" value={ms(s.p99)} icon={ICON_PATHS.clock} color="info" />
      </div>

      <Card>
        <SectionHeading
          variant="label"
          as="h3"
          actions={
            <div className="flex flex-wrap items-center gap-1.5">
              {chartControls}
              {(["count", "p95"] as Metric[]).map((m) => (
                <PillButton key={m} size="sm" active={metric === m} onClick={() => setMetric(m)}>
                  {t(`telemetry.metric.${m}`)}
                </PillButton>
              ))}
            </div>
          }
        >
          {t("telemetry.overTime")}
        </SectionHeading>
        <div className="mt-3">
          <LineChart
            data={rows}
            xKey="t"
            series={series}
            formatX={fmtX}
            formatTooltipX={fmtTip}
            formatY={metric === "count" ? (v) => num(v) : (v) => ms(v)}
            ariaLabel={t("telemetry.overTime")}
          />
        </div>
      </Card>

      <Card>
        <SectionHeading variant="label" as="h3" actions={groupControls}>
          {t("telemetry.top")}
        </SectionHeading>
        <div className="overflow-x-auto">
          <table className={`${tableClasses.compact.table} mt-2`}>
            <thead>
              <tr className={tableClasses.compact.headRow}>
                <th className={tableClasses.compact.th}>{t(`telemetry.groupBy.${groupBy}`)}</th>
                <th className={`${tableClasses.compact.th} text-right`}>{t("telemetry.requests")}</th>
                <th className={`${tableClasses.compact.th} text-right`}>5xx</th>
                <th className={`${tableClasses.compact.th} text-right`}>p95</th>
                <th className={`${tableClasses.compact.th} text-right`}>p99</th>
              </tr>
            </thead>
            <tbody>
              {(data.top ?? []).map((row) => {
                const name = row.label ? (
                  <>
                    <span className="font-sans text-sm text-[var(--text-primary)]">{row.label}</span>{" "}
                    <span className="text-[var(--text-muted)]">{row.key}</span>
                  </>
                ) : (
                  row.key || t("telemetry.unknown")
                );
                return (
                  <tr key={row.key} className={tableClasses.compact.row}>
                    <td className={`${tableClasses.compact.td} font-mono text-xs break-all`}>
                      {onPick ? (
                        <button type="button" onClick={() => onPick(row.key)} className="text-left hover:underline" title={t("telemetry.filters.pick")}>
                          {name}
                        </button>
                      ) : (
                        name
                      )}
                    </td>
                    <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{num(row.count)}</td>
                    <td className={`${tableClasses.compact.td} text-right tabular-nums ${row.errors ? "text-[var(--danger)]" : ""}`}>{num(row.errors)}</td>
                    <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{ms(row.p95)}</td>
                    <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{ms(row.p99)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
}

/** Request count, errors and latency percentiles of one API, service or host, read from SigNoz. */
export default function RequestsTab({ kind, id }: { kind: TelemetryKind; id: string | number }) {
  const { t, locale } = useLocale();
  const [range, setRange] = useState<TelemetryRange>("24h");
  const [groupBy, setGroupBy] = useState<TelemetryGroupBy>(GROUPS[kind][0]);
  const [filters, setFilters] = useState<TelemetryFilters>({});
  const [byKey, setByKey] = useState(false);
  const [trace, setTrace] = useState<string | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ["telemetry", kind, id, range, groupBy, filters, byKey],
    queryFn: () => telemetryAPI.requests(kind, id, range, groupBy, filters, byKey && kind === "api"),
    retry: false,
    staleTime: 60_000,
  });
  const { data: recent } = useQuery({
    queryKey: ["telemetry-spans", kind, id, range, filters],
    queryFn: () => telemetryAPI.spans(kind, id, range, filters),
    enabled: !!data?.available,
    retry: false,
    staleTime: 60_000,
  });

  const keyLabel = (k: string) => data?.facets?.keys?.find((o) => o.key === k)?.label || k || t("telemetry.unknown");
  const setFilter = (f: keyof TelemetryFilters, v?: string) => setFilters((cur) => ({ ...cur, [f]: v || undefined }));
  // A top row becomes a filter on the dimension it groups by (a host's service rows don't filter).
  const pick = groupBy === "service" ? undefined : (k: string) => setFilter(groupBy === "route" ? "route" : groupBy === "key" ? "key" : "status", k);
  const active = (Object.entries(filters) as [keyof TelemetryFilters, string | undefined][]).filter(([, v]) => v);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        {RANGES.map((r) => (
          <PillButton key={r} active={range === r} onClick={() => setRange(r)}>
            {t(`telemetry.range.${r}`)}
          </PillButton>
        ))}
        {data?.signoz_url && (
          <a
            href={data.signoz_url}
            target="_blank"
            rel="noreferrer"
            className="ml-auto inline-flex items-center gap-1 text-xs text-[var(--accent)] hover:underline"
          >
            {t("telemetry.openSignoz")}
            <Icon path={ICON_PATHS.externalLink} className="w-3.5 h-3.5" />
          </a>
        )}
      </div>

      {data?.available && data.facets && (
        <div className="flex flex-wrap items-end gap-3">
          <div className="w-full sm:w-72">
            <NativeSelect label={t("telemetry.filters.route")} value={filters.route ?? ""} onChange={(e) => setFilter("route", e.target.value)}>
              <option value="">{t("telemetry.filters.all")}</option>
              {[...new Set([...(filters.route ? [filters.route] : []), ...data.facets.routes])].map((r) => (
                <option key={r} value={r}>
                  {r || t("telemetry.unknown")}
                </option>
              ))}
            </NativeSelect>
          </div>
          {kind === "api" && (
            <div className="w-full sm:w-72">
              <NativeSelect label={t("telemetry.filters.key")} value={filters.key ?? ""} onChange={(e) => setFilter("key", e.target.value)}>
                <option value="">{t("telemetry.filters.all")}</option>
                {(data.facets.keys ?? [])
                  .filter((o) => o.key)
                  .map((o) => (
                    <option key={o.key} value={o.key}>
                      {o.label ? `${o.label} (${o.key})` : o.key}
                    </option>
                  ))}
              </NativeSelect>
            </div>
          )}
          {active.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5 pb-2">
              {active.map(([f, v]) => (
                <Tag key={f} onRemove={() => setFilter(f)} removeLabel={t("telemetry.filters.remove")}>
                  {`${t(`telemetry.filters.${f}`)}: ${f === "key" ? keyLabel(v!) : v}`}
                </Tag>
              ))}
            </div>
          )}
        </div>
      )}

      {isLoading && <Skeleton className="h-64 w-full rounded-[var(--radius-md)]" />}
      {error && <StatusAlert variant="error">{t("telemetry.loadError")}</StatusAlert>}
      {data && !data.available && <StatusAlert variant="info">{t(`telemetry.reason.${data.reason}`)}</StatusAlert>}
      {data?.available && (
        <RequestsPanel
          data={data}
          groupBy={groupBy}
          onPick={pick}
          chartControls={
            kind === "api" ? (
              <PillButton size="sm" active={byKey} onClick={() => setByKey(!byKey)}>
                {t("telemetry.byKey")}
              </PillButton>
            ) : undefined
          }
          groupControls={
            <div className="flex gap-1.5">
              {GROUPS[kind].map((g) => (
                <PillButton key={g} size="sm" active={groupBy === g} onClick={() => setGroupBy(g)}>
                  {t(`telemetry.groupBy.${g}`)}
                </PillButton>
              ))}
            </div>
          }
        />
      )}

      {data?.available && !!recent?.spans?.length && (
        <Card>
          <SectionHeading variant="label" as="h3" hint={t("telemetry.recent.hint")}>
            {t("telemetry.recent.title")}
          </SectionHeading>
          <div className="overflow-x-auto">
            <table className={`${tableClasses.compact.table} mt-2`}>
              <thead>
                <tr className={tableClasses.compact.headRow}>
                  <th className={tableClasses.compact.th}>{t("telemetry.recent.time")}</th>
                  <th className={tableClasses.compact.th}>{t("telemetry.recent.request")}</th>
                  <th className={tableClasses.compact.th}>{t("telemetry.recent.status")}</th>
                  {kind === "api" && <th className={tableClasses.compact.th}>{t("telemetry.groupBy.key")}</th>}
                  <th className={`${tableClasses.compact.th} text-right`}>{t("telemetry.recent.duration")}</th>
                </tr>
              </thead>
              <tbody>
                {recent.spans.map((sp) => {
                  const code = parseInt(sp.status) || 0;
                  return (
                    <tr
                      key={sp.span_id}
                      className={`${tableClasses.compact.row} cursor-pointer`}
                      onClick={() => setTrace(sp.trace_id)}
                      tabIndex={0}
                      onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && setTrace(sp.trace_id)}
                      aria-label={t("telemetry.recent.open")}
                    >
                      <td className={`${tableClasses.compact.td} whitespace-nowrap text-xs text-[var(--text-secondary)]`}>
                        {new Date(sp.time).toLocaleString(locale, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" })}
                      </td>
                      <td className={`${tableClasses.compact.td} font-mono text-xs break-all`}>
                        <span className="text-[var(--text-muted)]">{sp.method}</span> {sp.route || sp.path}
                      </td>
                      <td
                        className={`${tableClasses.compact.td} font-mono text-xs ${
                          code >= 500 ? "text-[var(--danger)]" : code >= 400 ? "text-[var(--warning)]" : "text-[var(--text-secondary)]"
                        }`}
                      >
                        {sp.status || "–"}
                      </td>
                      {kind === "api" && (
                        <td className={`${tableClasses.compact.td} text-xs truncate max-w-[14rem]`} title={sp.client}>
                          {sp.label || sp.client || t("telemetry.unknown")}
                        </td>
                      )}
                      <td className={`${tableClasses.compact.td} text-right tabular-nums text-xs`}>{ms(sp.duration_ms)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      <TraceDrawer kind={kind} id={id} traceId={trace} onClose={() => setTrace(null)} />
    </div>
  );
}
