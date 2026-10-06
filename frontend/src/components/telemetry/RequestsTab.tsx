"use client";

import { useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  telemetryAPI,
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
import Icon from "@/components/ui/Icon";
import { Skeleton } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import { ICON_PATHS } from "@/lib/icon-paths";

const RANGES: TelemetryRange[] = ["1h", "24h", "7d", "30d"];
// The groups that make sense per kind: callers' keys only exist behind the
// gateway (APIs); a host splits by its services.
const GROUPS: Record<TelemetryKind, TelemetryGroupBy[]> = {
  api: ["route", "key", "status"],
  service: ["route", "status"],
  host: ["service", "route", "status"],
};

const ms = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(1)} s` : `${Math.round(v)} ms`);

/** Whether the SigNoz integration is on: pages show their Requisições tab only then. */
export function useTelemetryEnabled(): boolean {
  const { data } = useQuery({ queryKey: ["telemetry-status"], queryFn: telemetryAPI.status, retry: false, staleTime: 5 * 60_000 });
  return data?.enabled === true;
}

/**
 * KPIs, requests over time and the top groups of one telemetry result. Data
 * only — the share page renders it from the redeemed payload, without auth.
 */
export function RequestsPanel({
  data,
  groupBy,
  groupControls,
}: {
  data: Pick<TelemetryRequests, "summary" | "series" | "top">;
  groupBy: TelemetryGroupBy;
  groupControls?: ReactNode;
}) {
  const { t, locale } = useLocale();
  const num = (n: number) => n.toLocaleString(locale);
  const s = data.summary;
  if (!s || s.count === 0) {
    return <EmptyState icon="search" title={t("telemetry.emptyTitle")} description={t("telemetry.emptyHint")} />;
  }
  const series = data.series ?? [];
  const peak = Math.max(1, ...series.map((b) => b.count));
  const errPct = (100 * s.errors) / s.count;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
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
        <SectionHeading variant="label" as="h3">{t("telemetry.overTime")}</SectionHeading>
        {/* One bar per bucket; the danger slice at its foot is the 5xx share. */}
        <div className="mt-3 flex items-end gap-px h-28" role="img" aria-label={t("telemetry.overTime")}>
          {series.map((b) => (
            <div
              key={b.t}
              className="flex-1 flex flex-col justify-end rounded-t-sm overflow-hidden bg-[var(--accent)]/60 min-h-px"
              style={{ height: `${(100 * b.count) / peak}%` }}
              title={`${new Date(b.t).toLocaleString(locale)} · ${num(b.count)} · p95 ${ms(b.p95)}${b.errors ? ` · ${num(b.errors)} 5xx` : ""}`}
            >
              {b.errors > 0 && <div className="bg-[var(--danger)]" style={{ height: `${(100 * b.errors) / b.count}%` }} />}
            </div>
          ))}
        </div>
      </Card>

      <Card>
        <SectionHeading variant="label" as="h3" actions={groupControls}>
          {t("telemetry.top")}
        </SectionHeading>
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
            {(data.top ?? []).map((row) => (
              <tr key={row.key} className={tableClasses.compact.row}>
                <td className={`${tableClasses.compact.td} font-mono text-xs break-all`}>
                  {row.label ? (
                    <>
                      <span className="font-sans text-sm text-[var(--text-primary)]">{row.label}</span>{" "}
                      <span className="text-[var(--text-muted)]">{row.key}</span>
                    </>
                  ) : (
                    row.key || t("telemetry.unknown")
                  )}
                </td>
                <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{num(row.count)}</td>
                <td className={`${tableClasses.compact.td} text-right tabular-nums ${row.errors ? "text-[var(--danger)]" : ""}`}>{num(row.errors)}</td>
                <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{ms(row.p95)}</td>
                <td className={`${tableClasses.compact.td} text-right tabular-nums`}>{ms(row.p99)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

/** Request count, errors and latency percentiles of one API, service or host, read from SigNoz. */
export default function RequestsTab({ kind, id }: { kind: TelemetryKind; id: string | number }) {
  const { t } = useLocale();
  const [range, setRange] = useState<TelemetryRange>("24h");
  const [groupBy, setGroupBy] = useState<TelemetryGroupBy>(GROUPS[kind][0]);

  const { data, isLoading, error } = useQuery({
    queryKey: ["telemetry", kind, id, range, groupBy],
    queryFn: () => telemetryAPI.requests(kind, id, range, groupBy),
    retry: false,
    staleTime: 60_000,
  });

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

      {isLoading && <Skeleton className="h-64 w-full rounded-[var(--radius-md)]" />}
      {error && <StatusAlert variant="error">{t("telemetry.loadError")}</StatusAlert>}
      {data && !data.available && <StatusAlert variant="info">{t(`telemetry.reason.${data.reason}`)}</StatusAlert>}
      {data?.available && (
        <RequestsPanel
          data={data}
          groupBy={groupBy}
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
    </div>
  );
}
