// The databases page's KPI catalog, dashboard breakdowns and list predicate,
// in the shape of apiInsights. Self-contained (type-only imports) so
// `node --test` runs it.
import type { Database } from "@/lib/types";
import type { Insight, BreakdownRow as BreakdownRowOf } from "@/lib/insights";

type T = (key: string, vars?: Record<string, string>) => string;

export type DatabaseFilters = {
  engine: string;
  environment: string;
  sensitivity: string;
  /** "instance" (runs on a discovered engine) | "external" */
  where: string;
  /** "none": no consumer service */
  consumers: string;
  /** "missing": prod without a backup policy */
  backup: string;
  /** "personal": pessoal or sensivel (LGPD) */
  lgpd: string;
  /** Linked project / consumer service id, as a string. */
  project: string;
  service: string;
};

export const emptyDatabaseFilters: DatabaseFilters = {
  engine: "", environment: "", sensitivity: "", where: "", consumers: "", backup: "", lgpd: "", project: "", service: "",
};

export type DatabaseInsight = Insight<DatabaseFilters>;
export type BreakdownRow = BreakdownRowOf<DatabaseFilters>;

export const DEFAULT_DATABASE_INSIGHTS = ["total", "personalData", "prodNoBackup", "noConsumers"];

export const ENGINES = ["postgres", "mysql", "mariadb", "oracle", "mssql", "trino", "mongodb", "redis", "other"] as const;
export const SENSITIVITIES = ["publico", "interno", "pessoal", "sensivel"] as const;
export const ENVIRONMENTS = ["prod", "homolog", "dev"] as const;

/** Holds personal data in the LGPD sense. */
export const isPersonal = (d: Pick<Database, "sensitivity">) => d.sensitivity === "pessoal" || d.sensitivity === "sensivel";
/** Production with no backup policy written down. */
export const lacksBackup = (d: Pick<Database, "environment" | "backup_policy">) => d.environment === "prod" && !d.backup_policy?.trim();
export const isExternal = (d: Pick<Database, "instance_service_id">) => d.instance_service_id == null;

/** The list predicate for a filter set (search is applied by the page). */
export function matchesDatabaseFilters(d: Database, f: DatabaseFilters): boolean {
  if (f.engine && d.engine !== f.engine) return false;
  if (f.environment && d.environment !== f.environment) return false;
  if (f.sensitivity && d.sensitivity !== f.sensitivity) return false;
  if (f.where === "instance" && isExternal(d)) return false;
  if (f.where === "external" && !isExternal(d)) return false;
  if (f.consumers === "none" && (d.consumer_service_ids?.length ?? 0) > 0) return false;
  if (f.backup === "missing" && !lacksBackup(d)) return false;
  if (f.lgpd === "personal" && !isPersonal(d)) return false;
  if (f.project && !(d.project_ids ?? []).includes(Number(f.project))) return false;
  if (f.service && !(d.consumer_service_ids ?? []).includes(Number(f.service))) return false;
  return true;
}

// ponytail: same countBy as apiInsights — shared code would need a runtime
// import, which `node --test` can't resolve without file extensions.
function countBy(items: Database[], value: (d: Database) => string | undefined, emptyKey: string, filter?: (v: string) => Partial<DatabaseFilters>, label?: (v: string) => string): BreakdownRow[] {
  const m = new Map<string, number>();
  for (const d of items) {
    const v = value(d)?.trim() || "";
    m.set(v, (m.get(v) ?? 0) + 1);
  }
  return [...m]
    .sort((x, y) => y[1] - x[1] || (x[0] === "" ? 1 : y[0] === "" ? -1 : x[0].localeCompare(y[0])))
    .map(([v, count]) => (v
      ? { key: v, label: label ? label(v) : v, labelKey: !!label, count, filter: filter?.(v) }
      : { key: emptyKey, label: emptyKey, labelKey: true, count }));
}

export function databaseInsights(dbs: Database[], t: T): DatabaseInsight[] {
  const n = (pred: (d: Database) => boolean) => dbs.filter(pred).length;
  const pct = (v: number) => (dbs.length ? `${Math.round((v / dbs.length) * 100)}%` : "0%");
  const external = n(isExternal);
  const personal = n(isPersonal);
  const noBackup = n(lacksBackup);
  const noConsumers = n((d) => (d.consumer_service_ids?.length ?? 0) === 0);

  const fixed: DatabaseInsight[] = [
    {
      key: "total", label: t("atlas.databases.kpi.total"), icon: "database", color: "accent", value: dbs.length,
      hint: t("atlas.databases.kpi.totalHint", { external: String(external) }),
    },
    { key: "personalData", label: t("atlas.databases.kpi.personalData"), icon: "lock", color: "danger", value: personal, hint: pct(personal), filter: { lgpd: "personal" } },
    { key: "prodNoBackup", label: t("atlas.databases.kpi.prodNoBackup"), icon: "alert", color: "warning", value: noBackup, hint: pct(noBackup), filter: { backup: "missing" } },
    { key: "noConsumers", label: t("atlas.databases.kpi.noConsumers"), icon: "link", color: "warning", value: noConsumers, hint: pct(noConsumers), filter: { consumers: "none" } },
    { key: "external", label: t("atlas.databases.kpi.external"), icon: "globe", color: "info", value: external, hint: pct(external), filter: { where: "external" } },
  ];
  const engines: DatabaseInsight[] = countBy(dbs, (d) => d.engine, "")
    .filter((r) => r.key)
    .map((r) => ({ key: `engine:${r.key}`, label: t(`atlas.databases.engine.${r.key}`), icon: "database", color: "info", value: r.count, hint: pct(r.count), filter: { engine: r.key } }));
  return [...fixed, ...engines];
}

export interface DatabaseBreakdowns {
  engine: BreakdownRow[];
  environment: BreakdownRow[];
  sensitivity: BreakdownRow[];
  projects: BreakdownRow[];
}

export function databaseBreakdowns(dbs: Database[], projectName: (id: number) => string | undefined = () => undefined): DatabaseBreakdowns {
  const projects = new Map<number, number>();
  let none = 0;
  for (const d of dbs) {
    const ids = d.project_ids ?? [];
    if (ids.length === 0) none++;
    for (const id of ids) projects.set(id, (projects.get(id) ?? 0) + 1);
  }
  const projectRows: BreakdownRow[] = [...projects]
    .sort((x, y) => y[1] - x[1] || x[0] - y[0])
    .map(([id, count]) => ({ key: String(id), label: projectName(id) ?? `#${id}`, count, filter: { project: String(id) } }));
  if (none) projectRows.push({ key: "atlas.databases.dash.noProject", label: "atlas.databases.dash.noProject", labelKey: true, count: none });
  return {
    engine: countBy(dbs, (d) => d.engine, "inventory.dash.none", (v) => ({ engine: v }), (v) => `atlas.databases.engine.${v}`),
    environment: countBy(dbs, (d) => d.environment, "inventory.dash.none", (v) => ({ environment: v }), (v) => `atlas.databases.environment.${v}`),
    sensitivity: countBy(dbs, (d) => d.sensitivity, "inventory.dash.none", (v) => ({ sensitivity: v }), (v) => `atlas.databases.sensitivity.${v}`),
    projects: projectRows,
  };
}

/** Badge colour of each sensitivity / environment. */
export const SENSITIVITY_COLOR = { publico: "success", interno: "info", pessoal: "warning", sensivel: "danger" } as const;
export const ENVIRONMENT_COLOR = { prod: "rose", homolog: "amber", dev: "cyan" } as const;

/** "db.example:5432" — where the database is reached: the override, else the instance. */
export function endpointLabel(d: Pick<Database, "host" | "port" | "instance">): string {
  const host = d.host || d.instance?.host_nickname || "";
  const port = d.port ?? (d.instance?.port ? Number(d.instance.port) || null : null);
  return host ? (port ? `${host}:${port}` : host) : "";
}

/** A services row's engine from its subtype / image (mirrors the backend's
 *  models.InferDatabaseEngine; mariadb before mysql). */
export function inferDatabaseEngine(subtype?: string, image?: string): Database["engine"] {
  const s = `${subtype ?? ""} ${image ?? ""}`.toLowerCase();
  const rules: [string, Database["engine"]][] = [
    ["postgres", "postgres"], ["postgis", "postgres"], ["timescale", "postgres"], ["mariadb", "mariadb"], ["mysql", "mysql"],
    ["oracle", "oracle"], ["mssql", "mssql"], ["sqlserver", "mssql"], ["sql server", "mssql"], ["trino", "trino"], ["mongo", "mongodb"],
    ["redis", "redis"], ["valkey", "redis"], ["dragonfly", "redis"], ["keydb", "redis"],
  ];
  return rules.find(([needle]) => s.includes(needle))?.[1] ?? "other";
}
