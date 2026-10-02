// The APIs page's KPI catalog, dashboard breakdowns and list predicate, in
// the shape of serviceInsights. Self-contained (type-only imports) so
// `node --test` runs it.
import type { ApiCatalog } from "@/lib/types";
import type { Insight, BreakdownRow as BreakdownRowOf } from "@/lib/insights";

type T = (key: string, vars?: Record<string, string>) => string;

export type ApiFilters = {
  /** "upload" | "url" */
  source: string;
  /** "none" (avulso) | "linked" */
  links: string;
  /** "none": no base URL */
  base: string;
  /** "yes": URL import not refreshed in SPEC_STALE_DAYS */
  stale: string;
  /** Linked project / service id, as a string. */
  project: string;
  service: string;
  /** spec_version, e.g. "3.0.3" */
  spec: string;
};

export const emptyApiFilters: ApiFilters = { source: "", links: "", base: "", stale: "", project: "", service: "", spec: "" };

export type ApiInsight = Insight<ApiFilters>;
export type BreakdownRow = BreakdownRowOf<ApiFilters>;

export const DEFAULT_API_INSIGHTS = ["total", "noLinks", "noBaseUrl", "specStale"];

/** A URL-imported spec not refreshed in this long counts as stale. */
export const SPEC_STALE_DAYS = 30;
const DAY = 86_400_000;

export function isSpecStale(a: ApiCatalog, now = Date.now()): boolean {
  return a.source_type === "url" && !!a.updated_at && now - Date.parse(a.updated_at) > SPEC_STALE_DAYS * DAY;
}

const linked = (a: ApiCatalog) => (a.service_ids?.length ?? 0) + (a.project_ids?.length ?? 0) > 0;

/** The list predicate for a filter set (search is applied by the page). */
export function matchesApiFilters(a: ApiCatalog, f: ApiFilters, now = Date.now()): boolean {
  if (f.source && a.source_type !== f.source) return false;
  if (f.links === "none" && linked(a)) return false;
  if (f.links === "linked" && !linked(a)) return false;
  if (f.base === "none" && a.base_url) return false;
  if (f.stale === "yes" && !isSpecStale(a, now)) return false;
  if (f.project && !(a.project_ids ?? []).includes(Number(f.project))) return false;
  if (f.service && !(a.service_ids ?? []).includes(Number(f.service))) return false;
  if (f.spec && a.spec_version !== f.spec) return false;
  return true;
}

// ponytail: same countBy as serviceInsights — shared code would need a runtime
// import, which `node --test` can't resolve without file extensions.
function countBy(items: ApiCatalog[], value: (a: ApiCatalog) => string | undefined, emptyKey: string, filter?: (v: string) => Partial<ApiFilters>, label?: (v: string) => string): BreakdownRow[] {
  const m = new Map<string, number>();
  for (const a of items) {
    const v = value(a)?.trim() || "";
    m.set(v, (m.get(v) ?? 0) + 1);
  }
  return [...m]
    .sort((x, y) => y[1] - x[1] || (x[0] === "" ? 1 : y[0] === "" ? -1 : x[0].localeCompare(y[0])))
    .map(([v, count]) => (v
      ? { key: v, label: label ? label(v) : v, labelKey: !!label, count, filter: filter?.(v) }
      : { key: emptyKey, label: emptyKey, labelKey: true, count }));
}

/** One row per linked id (an API linked to two projects counts in both), then the unlinked bucket. */
function countLinks(items: ApiCatalog[], ids: (a: ApiCatalog) => number[], name: (id: number) => string | undefined, filter: (id: string) => Partial<ApiFilters>): BreakdownRow[] {
  const m = new Map<number, number>();
  let none = 0;
  for (const a of items) {
    const list = ids(a) ?? [];
    if (list.length === 0) none++;
    for (const id of list) m.set(id, (m.get(id) ?? 0) + 1);
  }
  const rows: BreakdownRow[] = [...m]
    .sort((x, y) => y[1] - x[1] || x[0] - y[0])
    .map(([id, count]) => ({ key: String(id), label: name(id) ?? `#${id}`, count, filter: filter(String(id)) }));
  if (none) rows.push({ key: "atlas.apis.dash.unlinked", label: "atlas.apis.dash.unlinked", labelKey: true, count: none });
  return rows;
}

export function apiInsights(apis: ApiCatalog[], t: T, now = Date.now()): ApiInsight[] {
  const n = (pred: (a: ApiCatalog) => boolean) => apis.filter(pred).length;
  const pct = (v: number) => (apis.length ? `${Math.round((v / apis.length) * 100)}%` : "0%");
  const fromUrl = n((a) => a.source_type === "url");
  const noLinks = n((a) => !linked(a));
  const noBase = n((a) => !a.base_url);
  const stale = n((a) => isSpecStale(a, now));
  const ops = apis.reduce((sum, a) => sum + (a.operation_count || 0), 0);

  const fixed: ApiInsight[] = [
    {
      key: "total", label: t("atlas.apis.kpi.total"), icon: "code", color: "accent", value: apis.length,
      hint: t("atlas.apis.kpi.totalHint", { url: String(fromUrl), upload: String(apis.length - fromUrl) }),
    },
    { key: "operations", label: t("atlas.apis.kpi.operations"), icon: "bolt", color: "info", value: ops },
    { key: "noLinks", label: t("atlas.apis.kpi.noLinks"), icon: "link", color: "warning", value: noLinks, hint: pct(noLinks), filter: { links: "none" } },
    { key: "noBaseUrl", label: t("atlas.apis.kpi.noBaseUrl"), icon: "globe", color: "warning", value: noBase, hint: pct(noBase), filter: { base: "none" } },
    {
      key: "specStale", label: t("atlas.apis.kpi.specStale"), icon: "clock", color: "danger", value: stale,
      hint: t("atlas.apis.kpi.specStaleHint", { days: String(SPEC_STALE_DAYS) }), filter: { stale: "yes" },
    },
    { key: "fromUrl", label: t("atlas.apis.kpi.fromUrl"), icon: "globe", color: "info", value: fromUrl, hint: pct(fromUrl), filter: { source: "url" } },
  ];

  // One insight per spec version present, most common first.
  const specs: ApiInsight[] = countBy(apis, (a) => a.spec_version, "")
    .filter((r) => r.key)
    .map((r) => ({ key: `spec:${r.key}`, label: `OpenAPI ${r.key}`, icon: "document", color: "info", value: r.count, hint: pct(r.count), filter: { spec: r.key } }));

  return [...fixed, ...specs];
}

export interface ApiBreakdowns {
  source: BreakdownRow[];
  projects: BreakdownRow[];
  services: BreakdownRow[];
  spec: BreakdownRow[];
}

/** The dashboard's breakdowns; linked rows are labelled through the page's id → name maps. */
export function apiBreakdowns(
  apis: ApiCatalog[],
  projectName: (id: number) => string | undefined = () => undefined,
  serviceName: (id: number) => string | undefined = () => undefined,
): ApiBreakdowns {
  return {
    source: countBy(apis, (a) => a.source_type, "inventory.dash.none", (v) => ({ source: v }), (v) => `atlas.apis.sourceType.${v}`),
    projects: countLinks(apis, (a) => a.project_ids, projectName, (id) => ({ project: id })),
    services: countLinks(apis, (a) => a.service_ids, serviceName, (id) => ({ service: id })),
    spec: countBy(apis, (a) => a.spec_version, "inventory.dash.none", (v) => ({ spec: v })),
  };
}

/** Card colour of each API origin (Card accent / Badge color names). */
export const ORIGEM_COLOR = { propria: "rose", terceiro: "cyan", externa: "sky" } as const;

/** Markdown as one plain line for a card: the first paragraph, without
 *  heading/list/emphasis/code marks, links reduced to their text. */
export function plainText(md: string | undefined): string {
  const para = (md ?? "").trim().split(/\n\s*\n/)[0] ?? "";
  return para
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/^\s{0,3}(#{1,6}\s+|[-*+]\s+|\d+\.\s+|>\s?)/gm, "")
    .replace(/(\*\*|__|\*|_|`)/g, "")
    .replace(/\s+/g, " ")
    .trim();
}
