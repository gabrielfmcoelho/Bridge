// The hosts page's KPI catalog. Each insight counts something in the current
// listing and, where the list can show exactly those hosts, carries the
// filter (or sort) a click applies. Tags become insights of their own.
import type { Host, HostFilters } from "@/lib/types";
import type { Insight, BreakdownRow as BreakdownRowOf } from "@/lib/insights";

type T = (key: string, vars?: Record<string, string>) => string;

export type HostInsight = Insight<HostFilters>;

export const DEFAULT_HOST_INSIGHTS = ["total", "critical", "idle", "highUsage", "noScan"];

/** Highest of CPU / RAM / disk use in percent, or null without scan data. */
export function peakUsage(h: Host): number | null {
  const r = h.scan_resources;
  if (!h.has_scan || !r) return null;
  const vals = [r.cpu_usage, r.ram_percent, r.disk_percent].map((v) => parseFloat(v ?? "")).filter((n) => !Number.isNaN(n));
  return vals.length ? Math.max(...vals) : null;
}

export const HIGH_USAGE = 80;

/** How the page reads situação roles (useSituacao); the defaults treat the
 *  factory values as their own roles, which is what the tests use. */
export interface SituacaoRoles {
  roleOf: (value: string | undefined) => string;
  valueOf: (role: string) => string;
}
const factoryRoles: SituacaoRoles = { roleOf: (v) => v ?? "", valueOf: (r) => r };

export function hostInsights(hosts: Host[], t: T, roles: SituacaoRoles = factoryRoles): HostInsight[] {
  const n = (pred: (h: Host) => boolean) => hosts.filter(pred).length;
  const bySituacao = (role: string) => n((h) => roles.roleOf(h.situacao) === role);
  const pct = (v: number) => (hosts.length ? `${Math.round((v / hosts.length) * 100)}%` : "0%");
  const scanned = n((h) => !!h.has_scan);
  const withAlerts = n((h) => (h.alerts?.length ?? 0) > 0);

  const fixed: HostInsight[] = [
    {
      key: "total", label: t("dashboard.totalHosts"), icon: "serverStack", color: "cyan", value: hosts.length,
      hint: [
        [bySituacao("active"), t("common.active")],
        [bySituacao("maintenance"), t("common.maintenance")],
        [bySituacao("inactive"), t("common.inactive")],
      ].filter(([c]) => c).map(([c, l]) => `${c} ${String(l).toLowerCase()}`).join(" · "),
    },
    { key: "maintenance", label: t("host.kpi.maintenance"), icon: "gear", color: "warning", value: bySituacao("maintenance"), hint: pct(bySituacao("maintenance")), filter: { situacao: roles.valueOf("maintenance") } },
    { key: "inactive", label: t("host.kpi.inactive"), icon: "archive", color: "var(--text-secondary)", value: bySituacao("inactive"), hint: pct(bySituacao("inactive")), filter: { situacao: roles.valueOf("inactive") } },
    {
      key: "critical", label: t("host.kpi.critical"), icon: "alert", color: "danger",
      value: n((h) => !!h.alerts?.some((a) => a.level === "critical")),
      hint: t("host.kpi.withAnyAlert", { count: String(withAlerts) }), filter: { alert_level: "critical" },
    },
    { key: "idle", label: t("host.idle"), icon: "moon", color: "info", value: n((h) => !!h.idle), hint: t("host.kpi.ofScanned", { count: String(scanned) }), filter: { idle: "idle" } },
    {
      key: "highUsage", label: t("host.kpi.highUsage"), icon: "bolt", color: "warning",
      value: n((h) => (peakUsage(h) ?? 0) >= HIGH_USAGE), hint: t("host.kpi.highUsageHint", { pct: String(HIGH_USAGE) }),
      sort: { field: "resource_cpu", direction: "desc" },
    },
    { key: "noScan", label: t("host.kpi.noScan"), icon: "scan", color: "var(--text-secondary)", value: hosts.length - scanned, hint: pct(hosts.length - scanned), filter: { has_scan: "without" } },
    { key: "noCreds", label: t("host.kpi.noCreds"), icon: "key", color: "warning", value: n((h) => !h.has_key && !h.has_password), hint: pct(n((h) => !h.has_key && !h.has_password)) },
    {
      key: "containers", label: t("host.containers"), icon: "container", color: "info",
      value: hosts.reduce((sum, h) => sum + (h.containers_count || 0), 0), hint: t("host.kpi.onHosts", { count: String(n((h) => (h.containers_count ?? 0) > 0)) }),
      sort: { field: "containers_count", direction: "desc" },
    },
  ];

  // Every tag in the listing, most used first.
  const tagCounts = new Map<string, number>();
  for (const h of hosts) for (const tag of h.tags ?? []) tagCounts.set(tag, (tagCounts.get(tag) ?? 0) + 1);
  const tags: HostInsight[] = [...tagCounts]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([tag, count]) => ({
      key: `tag:${tag}`, label: tag, icon: "bookmark", color: "accent", value: count, hint: pct(count), filter: { tag },
    }));

  return [...fixed, ...tags];
}

export type BreakdownRow = BreakdownRowOf<HostFilters>;

export interface HostBreakdowns {
  situacao: BreakdownRow[];
  hospedagem: BreakdownRow[];
  entidade: BreakdownRow[];
  tags: BreakdownRow[];
  usage: BreakdownRow[];
  alerts: BreakdownRow[];
}

/** Counts per value, most frequent first; empty values under `emptyKey`. */
function countBy(hosts: Host[], value: (h: Host) => string | undefined, emptyKey: string, filter?: (v: string) => Partial<HostFilters>): BreakdownRow[] {
  const m = new Map<string, number>();
  for (const h of hosts) {
    const v = value(h)?.trim() || "";
    m.set(v, (m.get(v) ?? 0) + 1);
  }
  return [...m]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([v, count]) => (v ? { key: v, label: v, count, filter: filter?.(v) } : { key: emptyKey, label: emptyKey, labelKey: true, count }));
}

/** The dashboard's breakdowns of the current listing. Rows carry the list
 *  filter a click applies, where the list can express it. */
export function hostBreakdowns(hosts: Host[]): HostBreakdowns {
  const tagRows = new Map<string, number>();
  for (const h of hosts) for (const tag of h.tags ?? []) tagRows.set(tag, (tagRows.get(tag) ?? 0) + 1);

  const band = (h: Host) => {
    const p = peakUsage(h);
    return p === null ? "none" : p >= HIGH_USAGE ? "high" : p >= 50 ? "mid" : "low";
  };
  const bands = { low: 0, mid: 0, high: 0, none: 0 };
  for (const h of hosts) bands[band(h)]++;

  const levels = ["critical", "warning", "info"] as const;
  return {
    situacao: countBy(hosts, (h) => h.situacao, "inventory.dash.none", (v) => ({ situacao: v })),
    hospedagem: countBy(hosts, (h) => h.hospedagem, "inventory.dash.none"),
    entidade: countBy(hosts, (h) => h.main_entidade, "inventory.dash.none"),
    tags: [...tagRows].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([tag, count]) => ({ key: tag, label: tag, count, filter: { tag } })),
    usage: [
      { key: "high", label: "host.dash.usageHigh", labelKey: true, count: bands.high },
      { key: "mid", label: "host.dash.usageMid", labelKey: true, count: bands.mid },
      { key: "low", label: "host.dash.usageLow", labelKey: true, count: bands.low },
      { key: "none", label: "host.dash.usageNone", labelKey: true, count: bands.none, filter: { has_scan: "without" } },
    ],
    alerts: levels.map((l) => ({
      key: l, label: `alert.${l}`, labelKey: true,
      count: hosts.filter((h) => h.alerts?.some((a) => a.level === l)).length,
      filter: { alert_level: l },
    })),
  };
}

export interface ResourceGroup {
  name: string;
  count: number;
  /** Average use in percent (0 when no host in the group reported it). */
  cpu: number; ram: number; disk: number;
  /** Summed capacity, e.g. "48 vCPU", "" when unknown. */
  totalCpu: string; totalRam: string; totalDisk: string;
}

/** Average CPU / RAM / disk use and summed capacity of the scanned hosts,
 *  grouped by `groupOf` (hospedagem, situação), largest group first. */
export function resourcesBy(hosts: Host[], groupOf: (h: Host) => string | undefined): ResourceGroup[] {
  const pct = (s?: string) => { const n = parseFloat((s ?? "").replace("%", "")); return Number.isNaN(n) ? null : n; };
  const groups = new Map<string, { count: number; cpu: number[]; ram: number[]; disk: number[]; tc: string[]; tr: string[]; td: string[] }>();
  for (const h of hosts) {
    const r = h.scan_resources;
    if (!h.has_scan || !r) continue;
    const name = groupOf(h)?.trim() || "";
    const g = groups.get(name) ?? { count: 0, cpu: [], ram: [], disk: [], tc: [], tr: [], td: [] };
    groups.set(name, g);
    g.count++;
    for (const [arr, v] of [[g.cpu, pct(r.cpu_usage)], [g.ram, pct(r.ram_percent)], [g.disk, pct(r.disk_percent)]] as const) if (v !== null) arr.push(v);
    if (r.cpu) g.tc.push(r.cpu);
    if (r.ram) g.tr.push(r.ram);
    if (r.storage) g.td.push(r.storage);
  }
  const avg = (a: number[]) => (a.length ? Math.round(a.reduce((x, y) => x + y, 0) / a.length) : 0);
  return [...groups]
    .sort((a, b) => b[1].count - a[1].count || a[0].localeCompare(b[0]))
    .map(([name, g]) => ({ name, count: g.count, cpu: avg(g.cpu), ram: avg(g.ram), disk: avg(g.disk), totalCpu: sumSizes(g.tc), totalRam: sumSizes(g.tr), totalDisk: sumSizes(g.td) }));
}

const SIZE_UNITS = "KMGTP";

/**
 * Sums capacities: "8 vCPU" + "4 vCPU" → "12 vCPU"; sizes are added in bytes
 * (1024-based) so units can mix — "150G" + "1.5T" → "1.6 T", "7,5Gi" + "8Gi"
 * → "15.5 Gi" — and shown in the largest unit, styled like the first one seen.
 */
export function sumSizes(a: string[]): string {
  const fmt = (n: number) => (Number.isInteger(n) ? String(n) : n.toFixed(1));
  let total = 0, bytes = 0, unit = "", suffix = "";
  for (const s of a) {
    const m = s.replace(/,/g, ".").trim().match(/^([\d.]+)\s*(.*)$/);
    if (!m) continue;
    const u = m[2].trim();
    const p = SIZE_UNITS.indexOf(u.charAt(0).toUpperCase());
    if (p >= 0 && /^[KMGTP](i|iB|B)?$/i.test(u)) {
      bytes += parseFloat(m[1]) * 1024 ** (p + 1);
      if (!unit) { unit = u; suffix = u.includes("i") ? "i" : ""; }
    } else {
      total += parseFloat(m[1]);
      unit ||= u;
    }
  }
  if (bytes) {
    let v = bytes / 1024, p = 0;
    while (v >= 1024 && p < SIZE_UNITS.length - 1) { v /= 1024; p++; }
    return `${fmt(v)} ${SIZE_UNITS[p]}${suffix}`;
  }
  return total ? `${fmt(total)} ${unit}`.trim() : "";
}

/** The `n` most recently scanned hosts, newest first. */
export function recentScans(hosts: Host[], n = 5): Host[] {
  return hosts.filter((h) => h.last_scan_at).sort((a, b) => b.last_scan_at!.localeCompare(a.last_scan_at!)).slice(0, n);
}
