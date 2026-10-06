"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { telemetryAPI, type TelemetryKind, type TelemetryTraceSpan } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Drawer from "@/components/ui/Drawer";
import StatusAlert from "@/components/ui/StatusAlert";
import SectionHeading from "@/components/ui/SectionHeading";
import Icon from "@/components/ui/Icon";
import { Skeleton } from "@/components/ui/Skeleton";
import { CHART_COLORS, CHART_OTHER } from "@/components/ui/LineChart";
import { ICON_PATHS } from "@/lib/icon-paths";

const ms = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(2)} s` : `${v < 10 ? v.toFixed(1) : Math.round(v)} ms`);

/** One request's whole trace (gateway + the API's own spans) as a waterfall, with each span's attributes. */
export default function TraceDrawer({
  kind,
  id,
  traceId,
  onClose,
}: {
  kind: TelemetryKind;
  id: string | number;
  traceId: string | null;
  onClose: () => void;
}) {
  const { t } = useLocale();
  const [picked, setPicked] = useState<string | null>(null);
  const { data, isLoading, error } = useQuery({
    queryKey: ["telemetry-trace", kind, id, traceId],
    queryFn: () => telemetryAPI.trace(kind, id, traceId!),
    enabled: !!traceId,
    retry: false,
  });

  const rows = useMemo(() => layout(data?.spans ?? []), [data]);
  // Color follows the service: same slot for a service across the whole trace.
  const colorOf = useMemo(() => {
    const order = [...new Set(rows.map((r) => r.span.service))];
    return (svc: string) => CHART_COLORS[order.indexOf(svc)] ?? CHART_OTHER;
  }, [rows]);
  const total = Math.max(1, ...rows.map((r) => r.span.start_ms + r.span.duration_ms));
  const selected = rows.find((r) => r.span.span_id === picked)?.span ?? rows.find((r) => r.span.service === "apisix-gateway")?.span ?? rows[0]?.span;

  return (
    <Drawer
      open={!!traceId}
      onClose={() => {
        setPicked(null);
        onClose();
      }}
      title={t("telemetry.trace.title")}
      wide
      subHeader={<span className="font-mono text-xs text-[var(--text-muted)] break-all">{traceId}</span>}
      headerAction={
        data?.signoz_url ? (
          <a href={data.signoz_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-xs text-[var(--accent)] hover:underline">
            {t("telemetry.openSignoz")}
            <Icon path={ICON_PATHS.externalLink} className="w-3.5 h-3.5" />
          </a>
        ) : undefined
      }
    >
      <div className="space-y-4">
        {isLoading && <Skeleton className="h-48 w-full rounded-[var(--radius-md)]" />}
        {error && <StatusAlert variant="error">{t("telemetry.trace.loadError")}</StatusAlert>}
        {data?.cut && <StatusAlert variant="warning">{t("telemetry.trace.cut")}</StatusAlert>}

        {rows.length > 0 && (
          <div>
            <SectionHeading variant="label" as="h3" hint={`${rows.length} spans · ${ms(total)}`}>
              {t("telemetry.trace.waterfall")}
            </SectionHeading>
            <ul className="mt-2 space-y-0.5">
              {rows.map(({ span, depth }) => {
                const active = span.span_id === selected?.span_id;
                return (
                  <li key={span.span_id}>
                    <button
                      type="button"
                      onClick={() => setPicked(span.span_id)}
                      aria-pressed={active}
                      className={`w-full grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)_4.5rem] items-center gap-2 px-2 py-1 rounded-[var(--radius-sm)] text-left text-xs transition-colors ${
                        active ? "bg-[var(--accent-muted)]" : "hover:bg-[var(--bg-elevated)]"
                      }`}
                    >
                      <span className="flex items-center gap-1.5 min-w-0" style={{ paddingLeft: Math.min(depth, 8) * 10 }}>
                        <span className="w-2 h-2 rounded-full shrink-0" style={{ background: colorOf(span.service) }} />
                        <span className="truncate text-[var(--text-secondary)]">
                          <span className="text-[var(--text-muted)]">{span.service}</span> {span.name}
                        </span>
                      </span>
                      <span className="relative h-2.5 rounded-full bg-[var(--bg-elevated)]">
                        <span
                          className="absolute top-0 h-full rounded-full"
                          style={{
                            left: `${(100 * span.start_ms) / total}%`,
                            width: `max(3px, ${(100 * span.duration_ms) / total}%)`,
                            background: span.error ? "var(--danger)" : colorOf(span.service),
                          }}
                        />
                      </span>
                      <span className="text-right font-mono tabular-nums text-[var(--text-secondary)]">{ms(span.duration_ms)}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        )}

        {selected && (
          <div>
            <SectionHeading variant="label" as="h3" hint={`${selected.service} · ${selected.kind}${selected.status ? ` · ${selected.status}` : ""}`}>
              {selected.name}
            </SectionHeading>
            {data?.redacted && <p className="mt-1 text-xs text-[var(--text-muted)]">{t("telemetry.trace.redacted")}</p>}
            <dl className="mt-2 grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-x-3 gap-y-1 text-xs">
              {Object.entries(selected.attributes)
                .sort(([a], [b]) => a.localeCompare(b))
                .map(([k, v]) => (
                  <div key={k} className="contents">
                    <dt className="font-mono text-[var(--text-muted)] break-all">{k}</dt>
                    <dd className="font-mono text-[var(--text-primary)] break-all">{v}</dd>
                  </div>
                ))}
            </dl>
          </div>
        )}
      </div>
    </Drawer>
  );
}

/** Spans in tree order (parents before children, siblings by start) with their depth. */
function layout(spans: TelemetryTraceSpan[]): { span: TelemetryTraceSpan; depth: number }[] {
  const ids = new Set(spans.map((s) => s.span_id));
  const kids = new Map<string, TelemetryTraceSpan[]>();
  const roots: TelemetryTraceSpan[] = [];
  for (const s of spans) {
    // A parent outside the stored trace (e.g. an untraced caller) makes it a root.
    if (s.parent_id && ids.has(s.parent_id)) kids.set(s.parent_id, [...(kids.get(s.parent_id) ?? []), s]);
    else roots.push(s);
  }
  const out: { span: TelemetryTraceSpan; depth: number }[] = [];
  const walk = (list: TelemetryTraceSpan[], depth: number) => {
    for (const s of [...list].sort((a, b) => a.start_ms - b.start_ms)) {
      out.push({ span: s, depth });
      walk(kids.get(s.span_id) ?? [], depth + 1);
    }
  };
  walk(roots, 0);
  return out;
}
