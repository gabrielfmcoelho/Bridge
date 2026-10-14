// The home page's model: who the user is here (personas), which widgets that
// implies, and the first steps to show. Pure and runtime-import free so
// `node --test` can run homeModel.test.ts. Stored per user in users.home_prefs.

export const PERSONAS = ["infra", "dev", "chamados", "suporte", "atlas"] as const;
export type Persona = (typeof PERSONAS)[number];

/** Canonical order; "main" widgets hold work, "side" ones are shortcuts and
 *  reference. `movable` ones can be placed in either column (HomePrefs.placement). */
export const WIDGETS = [
  { id: "assistant", column: "main", permission: "ai.use", movable: true },
  { id: "toHandle", column: "main", permission: undefined },
  { id: "myRequests", column: "main", permission: undefined },
  { id: "myIssues", column: "main", permission: "issues.view", movable: true },
  { id: "infraHealth", column: "main", permission: "hosts.view" },
  { id: "atlas", column: "main", permission: undefined },
  { id: "quickActions", column: "side", permission: undefined },
  { id: "chamados", column: "side", permission: undefined },
  { id: "inventory", column: "side", permission: "hosts.view" },
  { id: "wiki", column: "side", permission: undefined },
] as const;
export type WidgetId = (typeof WIDGETS)[number]["id"];
export type Column = "main" | "side";

const PERSONA_WIDGETS: Record<Persona, WidgetId[]> = {
  infra: ["infraHealth", "myIssues", "inventory"],
  dev: ["myIssues", "atlas"],
  chamados: ["myRequests", "chamados"],
  suporte: ["toHandle", "chamados"],
  atlas: ["atlas"],
};
const EVERYONE: WidgetId[] = ["assistant", "quickActions", "wiki"];

export interface HomePrefs {
  v: 1;
  personas: Persona[];
  /** Visible widgets, in display order (per column). */
  widgets: WidgetId[];
  onboarded_at: string;
  /** First-step ids the user opened or dismissed. */
  done_steps: string[];
  /** Column chosen for a movable widget; absent = its default column. */
  placement: Partial<Record<WidgetId, Column>>;
}

export const isMovable = (id: WidgetId) => !!WIDGETS.find((w) => w.id === id && "movable" in w && w.movable);

/** The column a widget renders in for these prefs. */
export function columnOf(id: WidgetId, prefs: Pick<HomePrefs, "placement">): Column {
  return (isMovable(id) && prefs.placement[id]) || WIDGETS.find((w) => w.id === id)!.column;
}

/** May this user see the widget? Admins see everything (their permission
 *  list is empty under dev:mock, like the sidebar's admin-only items). */
export function canSeeWidget(id: WidgetId, permissions: string[], role?: string): boolean {
  const p = WIDGETS.find((w) => w.id === id)?.permission;
  return !p || role === "admin" || permissions.includes(p);
}

/** The widgets a set of personas starts with, in canonical order. */
export function defaultWidgets(personas: Persona[]): WidgetId[] {
  const want = new Set<WidgetId>([...EVERYONE, ...personas.flatMap((p) => PERSONA_WIDGETS[p])]);
  return WIDGETS.map((w) => w.id).filter((id) => want.has(id));
}

/** Onboarding pre-checks. Permissions can tell infra and dev apart; whether
 *  someone files chamados or answers them, or lives in the Atlas, they can't,
 *  so those are left for the user to pick. Nothing recognizable → chamados,
 *  the one thing every user can do. */
export function suggestPersonas(permissions: string[], role?: string): Persona[] {
  const has = (p: string) => role === "admin" || permissions.includes(p);
  const out: Persona[] = [];
  if (has("hosts.view") && has("ssh.operate")) out.push("infra");
  if (has("issues.view") && has("projects.view")) out.push("dev");
  return out.length ? out : ["chamados"];
}

/** Stored JSON → prefs, or null when the user hasn't been through onboarding.
 *  Unknown personas / widgets (renamed or removed since) are dropped. */
export function readPrefs(raw: unknown): HomePrefs | null {
  if (!raw || typeof raw !== "object") return null;
  const r = raw as Partial<HomePrefs>;
  if (!r.onboarded_at) return null;
  const ids = new Set<string>(WIDGETS.map((w) => w.id));
  const list = <T extends string>(v: unknown, ok: (s: string) => boolean) =>
    (Array.isArray(v) ? v.filter((s): s is T => typeof s === "string" && ok(s)) : []);
  return {
    v: 1,
    personas: list<Persona>(r.personas, (s) => (PERSONAS as readonly string[]).includes(s)),
    widgets: list<WidgetId>(r.widgets, (s) => ids.has(s)),
    onboarded_at: r.onboarded_at,
    done_steps: list<string>(r.done_steps, () => true),
    placement: Object.fromEntries(
      Object.entries(r.placement ?? {}).filter(([id, col]) => ids.has(id) && isMovable(id as WidgetId) && (col === "main" || col === "side")),
    ),
  };
}

export function newPrefs(personas: Persona[], now = new Date()): HomePrefs {
  return { v: 1, personas, widgets: defaultWidgets(personas), onboarded_at: now.toISOString(), done_steps: [], placement: {} };
}

export interface Step { id: string; labelKey: string; href: string }

const STEPS: Record<Persona, Step[]> = {
  infra: [
    { id: "sshKey", labelKey: "home.steps.sshKey", href: "/secrets" },
    { id: "scan", labelKey: "home.steps.scan", href: "/hosts" },
    { id: "topology", labelKey: "home.steps.topology", href: "/atlas/topology" },
  ],
  dev: [
    { id: "projects", labelKey: "home.steps.projects", href: "/projects" },
    { id: "issues", labelKey: "home.steps.issues", href: "/issues" },
    { id: "apis", labelKey: "home.steps.apis", href: "/atlas/apis" },
  ],
  chamados: [
    { id: "openChamado", labelKey: "home.steps.openChamado", href: "/chamados/forms" },
    { id: "catalog", labelKey: "home.steps.catalog", href: "/catalog" },
  ],
  suporte: [
    { id: "requests", labelKey: "home.steps.requests", href: "/requests" },
    { id: "chamados", labelKey: "home.steps.chamados", href: "/chamados" },
  ],
  atlas: [
    { id: "atlasCatalog", labelKey: "home.steps.atlasCatalog", href: "/atlas/catalog" },
    { id: "lineage", labelKey: "home.steps.lineage", href: "/atlas/lineage" },
    { id: "apis", labelKey: "home.steps.apis", href: "/atlas/apis" },
  ],
};

/** First steps still to do for these personas, each step once, wiki last. */
export function pendingSteps(personas: Persona[], done: string[]): Step[] {
  const seen = new Set(done);
  const out: Step[] = [];
  for (const s of [...personas.flatMap((p) => STEPS[p]), { id: "wiki", labelKey: "home.steps.wiki", href: "/wiki" }]) {
    if (seen.has(s.id)) continue;
    seen.add(s.id);
    out.push(s);
  }
  return out;
}
