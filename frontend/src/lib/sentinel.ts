// Pure helpers for Coolify Sentinel data (runtime-import free, so Node's test
// runner can load them: node --test src/lib/sentinel.test.ts).
import type { SentinelServiceTraffic, SentinelPoint } from "./types";

type Stats = { count: number; users?: number; errors: number; p50: number; p95: number; p99: number };

/** Sentinel traffic in RequestsPanel's shape (errors = 5xx, users = unique visitors). */
export function trafficToPanel(tr: SentinelServiceTraffic): {
  summary: Stats;
  series: (Stats & { t: string })[];
  top: (Stats & { key: string })[];
} {
  const o = tr.overview;
  return {
    summary: { count: o.requests, users: o.unique_visitors, errors: o.status.s5xx, p50: o.latency.p50, p95: o.latency.p95, p99: o.latency.p99 },
    series: tr.series.map((b) => ({
      t: new Date(b.bucket).toISOString(),
      count: b.requests,
      users: b.unique_visitors,
      errors: b.s5xx,
      p50: 0,
      p95: b.p95,
      p99: 0,
    })),
    // Sentinel's paths carry p50/p95 only: the panel hides its p99 column.
    top: tr.paths.map((p) => ({ key: p.path, count: p.requests, errors: p.s5xx, p50: p.p50, p95: p.p95, p99: 0 })),
  };
}

/** Points → LineChart rows; several series join on t. */
export function pointsToRows(series: Record<string, SentinelPoint[]>): Record<string, number>[] {
  const by = new Map<number, Record<string, number>>();
  for (const [key, pts] of Object.entries(series)) {
    for (const p of pts) {
      const row = by.get(p.t) ?? { t: p.t };
      row[key] = p.v;
      by.set(p.t, row);
    }
  }
  return [...by.values()].sort((a, b) => a.t - b.t);
}

/** "12.3%" or "–" for a missing value. */
export const pct = (v: number | null | undefined, digits = 0) => (v == null ? "–" : `${v.toFixed(digits)}%`);

/** Latency in ms, as seconds from 1 s. */
export const ms = (v: number | null | undefined) => (v == null ? "–" : v >= 1000 ? `${(v / 1000).toFixed(1)} s` : `${Math.round(v)} ms`);

/** Share of 5xx in percent, or null under 1 request. */
export const errorRate = (req: number | null | undefined, s5xx: number | null | undefined) =>
  req && s5xx != null ? (100 * s5xx) / req : null;
