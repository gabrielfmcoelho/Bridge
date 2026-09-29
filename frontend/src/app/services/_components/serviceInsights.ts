// The services page's KPI catalog and dashboard breakdowns, in the shape of
// hostInsights. Self-contained (type-only imports) so `node --test` runs it.
import type { Service } from "@/lib/types";
import type { Insight, BreakdownRow as BreakdownRowOf } from "@/lib/insights";
import type { ServiceFilters } from "../FilterDrawer";

type T = (key: string, vars?: Record<string, string>) => string;
export type ServiceInsight = Insight<ServiceFilters>;
export type BreakdownRow = BreakdownRowOf<ServiceFilters>;

export const DEFAULT_SERVICE_INSIGHTS = ["total", "offline", "kind:database", "autoReview", "noResponsavel"];

/** A service the scan has not heard from in this long counts as stale. */
export const STALE_DAYS = 7;
const DAY = 86_400_000;

export function isStale(s: Service, now = Date.now()): boolean {
  return s.source !== "manual" && !!s.last_seen_at && now - Date.parse(s.last_seen_at) > STALE_DAYS * DAY;
}

// ponytail: same countBy as hostInsights — shared code would need a runtime
// import, which `node --test` can't resolve without file extensions.
function countBy(items: Service[], value: (s: Service) => string | undefined, emptyKey: string, filter?: (v: string) => Partial<ServiceFilters>, label?: (v: string) => string): BreakdownRow[] {
  const m = new Map<string, number>();
  for (const s of items) {
    const v = value(s)?.trim() || "";
    m.set(v, (m.get(v) ?? 0) + 1);
  }
  // Most frequent first; on a tie the empty bucket goes last.
  return [...m]
    .sort((a, b) => b[1] - a[1] || (a[0] === "" ? 1 : b[0] === "" ? -1 : a[0].localeCompare(b[0])))
    .map(([v, count]) => (v
      ? { key: v, label: label ? label(v) : v, labelKey: !!label, count, filter: filter?.(v) }
      : { key: emptyKey, label: emptyKey, labelKey: true, count, filter: filter?.("none") }));
}

export function serviceInsights(services: Service[], t: T): ServiceInsight[] {
  const n = (pred: (s: Service) => boolean) => services.filter(pred).length;
  const pct = (v: number) => (services.length ? `${Math.round((v / services.length) * 100)}%` : "0%");
  const online = n((s) => s.container_status === "online");
  const offline = n((s) => s.container_status === "offline");
  const auto = n((s) => s.source === "auto");

  const fixed: ServiceInsight[] = [
    {
      key: "total", label: t("service.kpi.total"), icon: "serverStack", color: "accent", value: services.length,
      hint: t("service.kpi.onlineOffline", { online: String(online), offline: String(offline) }),
    },
    { key: "offline", label: t("service.kpi.offline"), icon: "alert", color: "danger", value: offline, hint: pct(offline), filter: { status: "offline" } },
    { key: "autoReview", label: t("service.kpi.autoReview"), icon: "scan", color: "info", value: auto, hint: t("service.kpi.autoReviewHint"), filter: { origin: "auto" } },
    { key: "noResponsavel", label: t("service.kpi.noResponsavel"), icon: "user", color: "warning", value: n((s) => !s.main_responsavel_name), hint: pct(n((s) => !s.main_responsavel_name)) },
    { key: "external", label: t("service.isExternalDependency"), icon: "link", color: "warning", value: n((s) => !!s.is_external_dependency), filter: { is_external_dependency: "yes" } },
    { key: "stale", label: t("service.kpi.stale"), icon: "clock", color: "warning", value: n((s) => isStale(s)), hint: t("service.kpi.staleHint", { days: String(STALE_DAYS) }) },
  ];

  // One insight per category present, most common first.
  const kinds: ServiceInsight[] = countBy(services, (s) => s.service_kind, "")
    .filter((r) => r.key)
    .map((r) => ({
      key: `kind:${r.key}`, label: t(`service.kind.${r.key}`), icon: "cube", color: "info", value: r.count, hint: pct(r.count), filter: { kind: r.key },
    }));

  const tagCounts = new Map<string, number>();
  for (const s of services) for (const tag of s.tags ?? []) tagCounts.set(tag, (tagCounts.get(tag) ?? 0) + 1);
  const tags: ServiceInsight[] = [...tagCounts]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([tag, count]) => ({ key: `tag:${tag}`, label: tag, icon: "bookmark", color: "accent", value: count, hint: pct(count), filter: { tag } }));

  return [...fixed, ...kinds, ...tags];
}

export interface ServiceBreakdowns {
  kind: BreakdownRow[];
  origin: BreakdownRow[];
  status: BreakdownRow[];
  engines: BreakdownRow[];
  hosts: BreakdownRow[];
  tags: BreakdownRow[];
}

/** The dashboard's breakdowns. Host rows are labelled through `hostName`
 *  (the page's id → nickname map); they carry no filter (grouping covers it). */
export function serviceBreakdowns(services: Service[], hostName: (id: number) => string | undefined = () => undefined): ServiceBreakdowns {
  const originOf = (s: Service) => s.source === "manual" || !s.source ? "manual" : s.source === "fixed" ? "fixed" : s.discovery_kind === "host" ? "host" : "container";
  const now = Date.now();
  const statusOf = (s: Service) => (isStale(s, now) ? "stale" : s.container_status || "");

  const hostCounts = new Map<number, number>();
  for (const s of services) for (const id of s.host_ids ?? []) hostCounts.set(id, (hostCounts.get(id) ?? 0) + 1);
  const tagRows = new Map<string, number>();
  for (const s of services) for (const tag of s.tags ?? []) tagRows.set(tag, (tagRows.get(tag) ?? 0) + 1);

  return {
    kind: countBy(services, (s) => s.service_kind, "service.kind.none", (v) => ({ kind: v }), (v) => `service.kind.${v}`),
    origin: countBy(services, originOf, "service.origin.manual", (v) => ({ origin: v }), (v) => `service.origin.${v}`),
    status: countBy(services, statusOf, "inventory.dash.none", (v) => (v === "online" || v === "offline" ? { status: v } : {}), (v) => `service.status.${v}`)
      .map((r) => (r.filter && Object.keys(r.filter).length === 0 ? { ...r, filter: undefined } : r)),
    engines: countBy(services.filter((s) => s.service_kind === "database" || s.service_kind === "cache"), (s) => s.service_subtype, "inventory.dash.none"),
    hosts: [...hostCounts]
      .sort((a, b) => b[1] - a[1] || a[0] - b[0])
      .map(([id, count]) => ({ key: String(id), label: hostName(id) ?? `#${id}`, count })),
    tags: [...tagRows].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([tag, count]) => ({ key: tag, label: tag, count, filter: { tag } })),
  };
}
