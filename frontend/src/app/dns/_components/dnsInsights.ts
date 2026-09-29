// The DNS page's KPI catalog and dashboard breakdowns, in the shape of
// hostInsights. Self-contained (type-only imports) so `node --test` runs it;
// the certificate bucket is injected (the page passes lib/dnsCert.certState).
import type { DNSRecord } from "@/lib/types";
import type { Insight, BreakdownRow as BreakdownRowOf } from "@/lib/insights";
import type { CertState } from "@/lib/dnsCert";
import type { DNSFilters } from "../FilterDrawer";

type T = (key: string, vars?: Record<string, string>) => string;
type CertOf = (d: DNSRecord) => CertState;
export type DnsInsight = Insight<DNSFilters>;
export type BreakdownRow = BreakdownRowOf<DNSFilters>;

export const DEFAULT_DNS_INSIGHTS = ["total", "certExpiring", "certError", "noHttps", "orphan"];

/** Linked to nothing: no host, no service, no project. */
export const isOrphan = (d: DNSRecord) => !d.host_ids?.length && !d.services_count && !d.projects_count;

// ponytail: same countBy as hostInsights — shared code would need a runtime
// import, which `node --test` can't resolve without file extensions.
function countBy(items: DNSRecord[], value: (d: DNSRecord) => string | undefined, emptyKey: string, filter?: (v: string) => Partial<DNSFilters>): BreakdownRow[] {
  const m = new Map<string, number>();
  for (const d of items) {
    const v = value(d)?.trim() || "";
    m.set(v, (m.get(v) ?? 0) + 1);
  }
  return [...m]
    .sort((a, b) => b[1] - a[1] || (a[0] === "" ? 1 : b[0] === "" ? -1 : a[0].localeCompare(b[0])))
    .map(([v, count]) => (v ? { key: v, label: v, count, filter: filter?.(v) } : { key: emptyKey, label: emptyKey, labelKey: true, count }));
}

export function dnsInsights(records: DNSRecord[], t: T, certOf: CertOf): DnsInsight[] {
  const n = (pred: (d: DNSRecord) => boolean) => records.filter(pred).length;
  const pct = (v: number) => (records.length ? `${Math.round((v / records.length) * 100)}%` : "0%");
  const https = n((d) => d.has_https);
  const expiring = n((d) => ["warning", "critical"].includes(certOf(d)));

  const fixed: DnsInsight[] = [
    { key: "total", label: t("dns.totalDns"), icon: "globeMeridian", color: "success", value: records.length, hint: t("dns.kpi.httpsCount", { count: String(https) }) },
    { key: "certExpiring", label: t("dns.kpi.certExpiring"), icon: "clock", color: "warning", value: expiring, hint: t("dns.kpi.certExpiringHint"), filter: { cert: "30" } },
    { key: "certExpired", label: t("dns.certExpired"), icon: "alert", color: "danger", value: n((d) => certOf(d) === "expired"), filter: { cert: "expired" } },
    { key: "certError", label: t("dns.kpi.certError"), icon: "alert", color: "danger", value: n((d) => !!d.cert_error), hint: t("dns.kpi.certErrorHint"), filter: { cert: "error" } },
    { key: "unscanned", label: t("dns.certUnscanned"), icon: "scan", color: "info", value: n((d) => certOf(d) === "unscanned"), filter: { cert: "unscanned" } },
    { key: "noHttps", label: t("dns.kpi.noHttps"), icon: "lock", color: "warning", value: records.length - https, hint: pct(records.length - https), filter: { has_https: "no" } },
    { key: "orphan", label: t("dns.kpi.orphan"), icon: "link", color: "warning", value: n(isOrphan), hint: t("dns.kpi.orphanHint") },
  ];

  // One insight per situação present, then tags.
  const situacoes: DnsInsight[] = countBy(records, (d) => d.situacao, "")
    .filter((r) => r.key)
    .map((r) => ({ key: `sit:${r.key}`, label: r.key, icon: "checkCircle", color: "info", value: r.count, hint: pct(r.count), filter: { situacao: r.key } }));
  const tagCounts = new Map<string, number>();
  for (const d of records) for (const tag of d.tags ?? []) tagCounts.set(tag, (tagCounts.get(tag) ?? 0) + 1);
  const tags: DnsInsight[] = [...tagCounts]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([tag, count]) => ({ key: `tag:${tag}`, label: tag, icon: "bookmark", color: "accent", value: count, hint: pct(count), filter: { tag } }));

  return [...fixed, ...situacoes, ...tags];
}

export interface DnsBreakdowns {
  situacao: BreakdownRow[];
  cert: BreakdownRow[];
  issuer: BreakdownRow[];
  hosts: BreakdownRow[];
  entidade: BreakdownRow[];
  tags: BreakdownRow[];
}

/** Certificate health buckets, worst first, each with the list filter it maps to. */
const CERT_BUCKETS: { key: string; states: CertState[]; filter?: Partial<DNSFilters> }[] = [
  { key: "expired", states: ["expired"], filter: { cert: "expired" } },
  { key: "critical", states: ["critical"], filter: { cert: "7" } },
  { key: "warning", states: ["warning"], filter: { cert: "30" } },
  { key: "error", states: ["unreachable", "untrusted"], filter: { cert: "error" } },
  { key: "unscanned", states: ["unscanned"], filter: { cert: "unscanned" } },
  { key: "none", states: ["none"], filter: { has_https: "no" } },
  { key: "ok", states: ["ok"] },
];

export function dnsBreakdowns(records: DNSRecord[], certOf: CertOf, hostName: (id: number) => string | undefined = () => undefined): DnsBreakdowns {
  const states = records.map((d) => certOf(d)); // not map(certOf): certState takes `now` second
  const hostCounts = new Map<number, number>();
  for (const d of records) for (const id of d.host_ids ?? []) hostCounts.set(id, (hostCounts.get(id) ?? 0) + 1);
  const tagRows = new Map<string, number>();
  for (const d of records) for (const tag of d.tags ?? []) tagRows.set(tag, (tagRows.get(tag) ?? 0) + 1);

  return {
    situacao: countBy(records, (d) => d.situacao, "inventory.dash.none", (v) => ({ situacao: v })),
    cert: CERT_BUCKETS.map((b) => ({
      key: b.key, label: `dns.certBucket.${b.key}`, labelKey: true,
      count: states.filter((s) => b.states.includes(s)).length, filter: b.filter,
    })),
    issuer: countBy(records.filter((d) => d.cert_checked_at), (d) => d.cert_issuer, "inventory.dash.none"),
    hosts: [...hostCounts]
      .sort((a, b) => b[1] - a[1] || a[0] - b[0])
      .map(([id, count]) => ({ key: String(id), label: hostName(id) ?? `#${id}`, count })),
    entidade: countBy(records, (d) => d.main_entidade, "inventory.dash.none"),
    tags: [...tagRows].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([tag, count]) => ({ key: tag, label: tag, count, filter: { tag } })),
  };
}
