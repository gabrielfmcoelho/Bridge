// The projects page's KPI catalog and dashboard breakdowns, in the shape of
// hostInsights. Self-contained (type-only imports) so `node --test` runs it.
import type { Project } from "@/lib/types";
import type { Insight, BreakdownRow as BreakdownRowOf } from "@/lib/insights";
import type { ProjectFilters } from "../FilterDrawer";

type T = (key: string, vars?: Record<string, string>) => string;
export type ProjectInsight = Insight<ProjectFilters>;
export type BreakdownRow = BreakdownRowOf<ProjectFilters>;

export const DEFAULT_PROJECT_INSIGHTS = ["total", "openIssues", "noOwner", "noServices", "external"];

// ponytail: same countBy as hostInsights — shared code would need a runtime
// import, which `node --test` can't resolve without file extensions.
function countBy(items: Project[], value: (p: Project) => string | undefined, emptyKey: string, filter?: (v: string) => Partial<ProjectFilters>): BreakdownRow[] {
  const m = new Map<string, number>();
  for (const p of items) {
    const v = value(p)?.trim() || "";
    m.set(v, (m.get(v) ?? 0) + 1);
  }
  return [...m]
    .sort((a, b) => b[1] - a[1] || (a[0] === "" ? 1 : b[0] === "" ? -1 : a[0].localeCompare(b[0])))
    .map(([v, count]) => (v ? { key: v, label: v, count, filter: filter?.(v) } : { key: emptyKey, label: emptyKey, labelKey: true, count }));
}

export function projectInsights(projects: Project[], t: T): ProjectInsight[] {
  const n = (pred: (p: Project) => boolean) => projects.filter(pred).length;
  const pct = (v: number) => (projects.length ? `${Math.round((v / projects.length) * 100)}%` : "0%");
  const openIssues = projects.reduce((sum, p) => sum + (p.issues_count ?? 0), 0);
  const withOpen = n((p) => (p.issues_count ?? 0) > 0);

  const fixed: ProjectInsight[] = [
    { key: "total", label: t("project.kpi.total"), icon: "folder", color: "warning", value: projects.length,
      hint: t("project.kpi.servicesTotal", { count: String(projects.reduce((s, p) => s + (p.services_count ?? 0), 0)) }) },
    { key: "openIssues", label: t("project.kpi.openIssues"), icon: "clipboard", color: "accent", value: openIssues, hint: t("project.kpi.inProjects", { count: String(withOpen) }) },
    { key: "noOwner", label: t("project.kpi.noOwner"), icon: "user", color: "warning", value: n((p) => !p.main_responsavel_name), hint: pct(n((p) => !p.main_responsavel_name)) },
    { key: "noServices", label: t("project.kpi.noServices"), icon: "serverStack", color: "info", value: n((p) => !p.services_count), hint: pct(n((p) => !p.services_count)) },
    { key: "external", label: t("project.kpi.external"), icon: "building", color: "warning", value: n((p) => !!p.tem_empresa_externa_responsavel) },
    { key: "noRepo", label: t("project.kpi.noRepo"), icon: "code", color: "info", value: n((p) => !p.repos_count), hint: pct(n((p) => !p.repos_count)) },
  ];

  const situacoes: ProjectInsight[] = countBy(projects, (p) => p.situacao, "")
    .filter((r) => r.key)
    .map((r) => ({ key: `sit:${r.key}`, label: r.key, icon: "checkCircle", color: "info", value: r.count, hint: pct(r.count), filter: { situacao: r.key } }));
  const tagCounts = new Map<string, number>();
  for (const p of projects) for (const tag of p.tags ?? []) tagCounts.set(tag, (tagCounts.get(tag) ?? 0) + 1);
  const tags: ProjectInsight[] = [...tagCounts]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([tag, count]) => ({ key: `tag:${tag}`, label: tag, icon: "bookmark", color: "accent", value: count, hint: pct(count), filter: { tag } }));

  return [...fixed, ...situacoes, ...tags];
}

export interface ProjectBreakdowns {
  situacao: BreakdownRow[];
  setor: BreakdownRow[];
  entidade: BreakdownRow[];
  byServices: BreakdownRow[];
  byIssues: BreakdownRow[];
  tags: BreakdownRow[];
}

/** Top projects by a count (rows are projects, labelled by name). */
const topBy = (projects: Project[], count: (p: Project) => number): BreakdownRow[] =>
  projects
    .filter((p) => count(p) > 0)
    .sort((a, b) => count(b) - count(a) || a.name.localeCompare(b.name))
    .map((p) => ({ key: String(p.id), label: p.name, count: count(p) }));

export function projectBreakdowns(projects: Project[]): ProjectBreakdowns {
  const tagRows = new Map<string, number>();
  for (const p of projects) for (const tag of p.tags ?? []) tagRows.set(tag, (tagRows.get(tag) ?? 0) + 1);
  return {
    situacao: countBy(projects, (p) => p.situacao, "inventory.dash.none", (v) => ({ situacao: v })),
    setor: countBy(projects, (p) => p.setor_responsavel, "inventory.dash.none"),
    entidade: countBy(projects, (p) => p.main_entidade, "inventory.dash.none"),
    byServices: topBy(projects, (p) => p.services_count ?? 0),
    byIssues: topBy(projects, (p) => p.issues_count ?? 0),
    tags: [...tagRows].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([tag, count]) => ({ key: tag, label: tag, count, filter: { tag } })),
  };
}
