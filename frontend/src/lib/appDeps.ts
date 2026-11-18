// App dependencies: the catalogs (catalog/dependencies.yml) read from GitLab,
// turned into the report rows and the graphs the module draws. A port of
// catalog-parser's MermaidRenderer (diagram.py) to a node/edge model.
// Runtime-import free so node --test can run appDeps.test.ts.
import type { AppDepAssetLink, AppDepCatalog, AppDepComponent, AppDepEntityType, AppDepLink, AppDepSource } from "@/lib/types";

/** catalog-governance's criticalities, most critical first. Unknown values sort after them. */
export const CRITICALITY_ORDER = ["high", "medium", "low"] as const;
/** The route of a CI's page (its diagram). */
export function appDepsHref(ci: string): string {
  return `/atlas/app-deps/${encodeURIComponent(ci)}`;
}

/** Categories of stored data (no protocol/port; drawn as data). */
export const DATA_CATEGORIES = ["data"];

export function criticalityRank(c?: string): number {
  const i = (CRITICALITY_ORDER as readonly string[]).indexOf(c ?? "");
  return i < 0 ? CRITICALITY_ORDER.length + (c ? 0 : 1) : i;
}

/** The most critical of the values ("" / undefined ignored), or undefined. */
export function worstCriticality(values: (string | undefined)[]): string | undefined {
  let best: string | undefined;
  for (const v of values) if (v && (best === undefined || criticalityRank(v) < criticalityRank(best))) best = v;
  return best;
}

/** "https:8443", "https", ":5432" or "" — the endpoint label of diagram.py. */
export function endpoint(protocol?: string, port?: number): string {
  const p = protocol ?? "";
  return port ? `${p}:${port}` : p;
}

// ── index ────────────────────────────────────────────────────────────────

export interface AppDepEntry {
  catalog: AppDepCatalog;
  source: AppDepSource;
}

export interface AppDepIndex {
  byCi: Map<string, AppDepEntry>;
  /** A CI declared by more than one repository: the repositories after the first (by path). */
  duplicates: Map<string, AppDepSource[]>;
  /** Files that failed to parse. */
  failed: AppDepSource[];
}

/** The parsed catalogs by CI. With duplicates, the first repository by path wins. */
export function indexByCi(sources: AppDepSource[]): AppDepIndex {
  const idx: AppDepIndex = { byCi: new Map(), duplicates: new Map(), failed: [] };
  const sorted = [...sources].sort((a, b) => a.project_path.localeCompare(b.project_path));
  for (const s of sorted) {
    if (!s.catalog) {
      idx.failed.push(s);
      continue;
    }
    const ci = s.catalog.ci;
    if (idx.byCi.has(ci)) idx.duplicates.set(ci, [...(idx.duplicates.get(ci) ?? []), s]);
    else idx.byCi.set(ci, { catalog: s.catalog, source: s });
  }
  return idx;
}

function componentOf(idx: AppDepIndex, ci: string, name?: string): AppDepComponent | undefined {
  if (!name) return undefined;
  return idx.byCi.get(ci)?.catalog.components.find((c) => c.name === name);
}

// ── report ───────────────────────────────────────────────────────────────

export interface AppDepRow {
  ci: string;
  description: string;
  entities: string[];
  owners: string[];
  technologies: string[];
  components: number;
  upstream: number;
  downstream: number;
  /** Most critical of the CI's components. */
  criticality?: string;
  source: AppDepSource;
}

export function reportRows(idx: AppDepIndex): AppDepRow[] {
  return [...idx.byCi.values()]
    .map(({ catalog: c, source }) => ({
      ci: c.ci,
      description: c.description,
      entities: c.entity.map((e) => e.name || e.slug),
      owners: c.owners.map((o) => o.name || o.username || o.email),
      technologies: [...new Set(c.components.map((x) => x.technology).filter(Boolean))],
      components: c.components.length,
      upstream: c.dependencies.upstream.length,
      downstream: c.dependencies.downstream.length,
      criticality: worstCriticality(c.components.map((x) => x.criticality)),
      source,
    }))
    .sort((a, b) => a.ci.localeCompare(b.ci));
}

/** Case-insensitive match on CI, description, technologies, owners, entities and repository path. */
export function rowMatches(row: AppDepRow, catalog: AppDepCatalog | undefined, q: string): boolean {
  const needle = q.trim().toLowerCase();
  if (!needle) return true;
  const hay = [
    row.ci, row.description, row.source.project_path, ...row.technologies, ...row.entities,
    ...(catalog?.owners.flatMap((o) => [o.name, o.email, o.username]) ?? []),
  ];
  return hay.some((h) => h?.toLowerCase().includes(needle));
}

export type LinkDirection = "upstream" | "downstream";

/** One declared link of a CI, with the other end's traits resolved. */
export interface AppDepLinkRow {
  ci: string;
  direction: LinkDirection;
  /** The other CI. */
  other: string;
  /** The other end's component (none: an external system, or a consumer without a catalog). */
  component?: string;
  localComponent?: string;
  via?: string;
  category?: string;
  technology?: string;
  criticality?: string;
  description?: string;
  /** What is connected to: the provider's protocol:port (upstream) or this CI's (downstream). */
  endpoint: string;
  /** The other CI has a catalog in Bridge (its diagram can be opened). */
  catalogued: boolean;
}

/** The links a CI declares, upstream first. A link to a catalogued component takes that
 *  component's traits (as catalog-diagram --peers does); an external one its inline traits. */
export function linkRows(catalog: AppDepCatalog, idx: AppDepIndex): AppDepLinkRow[] {
  const own = (name?: string) => componentOf(idx, catalog.ci, name) ?? catalog.components.find((c) => c.name === name);
  const row = (l: AppDepLink, direction: LinkDirection): AppDepLinkRow => {
    const peer = componentOf(idx, l.ci, l.component);
    const traits = l.component ? peer : l;
    const local = own(l.local_component);
    return {
      ci: catalog.ci,
      direction,
      other: l.ci,
      component: l.component,
      localComponent: l.local_component,
      via: l.via,
      category: traits?.category,
      technology: traits?.technology,
      criticality: traits?.criticality,
      description: traits?.description,
      endpoint: direction === "upstream" ? endpoint(traits?.protocol, traits?.port) : endpoint(local?.protocol, local?.port),
      catalogued: idx.byCi.has(l.ci),
    };
  };
  return [
    ...catalog.dependencies.upstream.map((l) => row(l, "upstream")),
    ...catalog.dependencies.downstream.map((l) => row(l, "downstream")),
  ];
}

// ── graph ────────────────────────────────────────────────────────────────

export type AppGraphNodeKind = "component" | "ci" | "external";

export interface AppGraphNode {
  id: string;
  kind: AppGraphNodeKind;
  label: string;
  /** The CI it belongs to (component, ci) or is (external). */
  ci: string;
  /** The CI has a catalog in Bridge: a click opens its diagram. */
  catalogued: boolean;
  technology?: string;
  endpoint?: string;
  criticality?: string;
  data?: boolean;
  description?: string;
  /** Part of the CI the diagram is about. */
  focus?: boolean;
}

/** A frame around one CI's nodes. */
export interface AppGraphGroup {
  id: string;
  label: string;
  ci: string;
  catalogued: boolean;
  focus: boolean;
  members: string[];
}

/** source depends on target (catalog-parser's arrow direction). */
export interface AppGraphEdge {
  id: string;
  source: string;
  target: string;
  label: string;
}

export interface AppGraph {
  nodes: AppGraphNode[];
  groups: AppGraphGroup[];
  edges: AppGraphEdge[];
}

// Plain fields, no parameter properties: Node's type stripping can't run them.
class GraphBuilder {
  nodes = new Map<string, AppGraphNode>();
  groups = new Map<string, AppGraphGroup>();
  edges = new Map<string, AppGraphEdge>();
  idx: AppDepIndex;
  focus?: string;

  constructor(idx: AppDepIndex, focus?: string) {
    this.idx = idx;
    this.focus = focus;
  }

  member(ci: string, node: AppGraphNode): string {
    if (!this.nodes.has(node.id)) {
      this.nodes.set(node.id, node);
      let g = this.groups.get(ci);
      if (!g) {
        g = { id: `g:${ci}`, label: ci, ci, catalogued: this.idx.byCi.has(ci), focus: ci === this.focus, members: [] };
        this.groups.set(ci, g);
      }
      g.members.push(node.id);
    }
    return node.id;
  }

  /** A component of ci: full traits when its catalog declares it, else its bare name. */
  component(ci: string, name: string): string {
    const c = componentOf(this.idx, ci, name);
    return this.member(ci, {
      id: `c:${ci}/${name}`,
      kind: "component",
      label: name,
      ci,
      catalogued: this.idx.byCi.has(ci),
      technology: c?.technology,
      endpoint: c ? endpoint(c.protocol, c.port) : undefined,
      criticality: c?.criticality,
      data: c ? DATA_CATEGORIES.includes(c.category) : undefined,
      description: c?.description,
      focus: ci === this.focus,
    });
  }

  /** The CI itself, inside its frame: the end of a link with no (known) component. */
  ciNode(ci: string): string {
    return this.member(ci, { id: `n:${ci}`, kind: "ci", label: ci, ci, catalogued: this.idx.byCi.has(ci), focus: ci === this.focus });
  }

  /** A CI with no catalog in Bridge and no component: an external system or a consumer. */
  external(l: AppDepLink): string {
    const id = `x:${l.ci}`;
    const prev = this.nodes.get(id);
    if (prev) {
      // Shared by name: the first link that carries traits fills them.
      if (!prev.technology && l.technology) Object.assign(prev, this.externalTraits(l));
      return id;
    }
    this.nodes.set(id, { id, kind: "external", label: l.ci, ci: l.ci, catalogued: false, ...this.externalTraits(l) });
    return id;
  }

  externalTraits(l: AppDepLink) {
    return {
      technology: l.technology,
      endpoint: endpoint(l.protocol, l.port) || undefined,
      criticality: l.criticality,
      data: l.category ? DATA_CATEGORIES.includes(l.category) : undefined,
      description: l.description,
    };
  }

  /** This CI's end of a link: the local component it names, else the CI node. */
  ownEnd(c: AppDepCatalog, local?: string): string {
    return local && c.components.some((x) => x.name === local) ? this.component(c.ci, local) : this.ciNode(c.ci);
  }

  /** The other end: its component, the CI node of a catalogued CI, or an external node. */
  otherEnd(l: AppDepLink): string {
    if (l.component) return this.component(l.ci, l.component);
    if (this.idx.byCi.has(l.ci)) return this.ciNode(l.ci);
    return this.external(l);
  }

  edge(source: string, target: string, ep: string, via?: string) {
    if (source === target) return;
    const label = [ep, via ? `via ${via}` : ""].filter(Boolean).join(" · ");
    const key = `${source}->${target}`;
    const prev = this.edges.get(key);
    if (prev) {
      if (!prev.label && label) prev.label = label;
      return;
    }
    this.edges.set(key, { id: `e:${key}`, source, target, label });
  }

  /** Draws what catalog c declares (diagram.py's MermaidRenderer.render). */
  declare(c: AppDepCatalog, allComponents: boolean) {
    if (allComponents) {
      c.components.forEach((x) => this.component(c.ci, x.name));
      if (c.components.length === 0) this.ciNode(c.ci);
    }
    for (const l of c.dependencies.upstream) {
      // this CI consumes the other end
      const peer = componentOf(this.idx, l.ci, l.component);
      const ep = l.component ? endpoint(peer?.protocol, peer?.port) : endpoint(l.protocol, l.port);
      this.edge(this.ownEnd(c, l.local_component), this.otherEnd(l), ep, l.via);
    }
    for (const l of c.dependencies.downstream) {
      // the other end consumes this CI
      const local = c.components.find((x) => x.name === l.local_component);
      this.edge(this.otherEnd(l), this.ownEnd(c, l.local_component), endpoint(local?.protocol, local?.port), l.via);
    }
  }

  build(): AppGraph {
    return { nodes: [...this.nodes.values()], groups: [...this.groups.values()], edges: [...this.edges.values()] };
  }
}

/** One CI's diagram, as catalog-diagram draws it: its components, and the CIs and
 *  components its own catalog links to (other catalogs' declarations are not read). */
export function buildCiGraph(ci: string, idx: AppDepIndex): AppGraph | null {
  const entry = idx.byCi.get(ci);
  if (!entry) return null;
  const b = new GraphBuilder(idx, ci);
  b.declare(entry.catalog, true);
  return b.build();
}

/** Every catalogued CI on one canvas. A link declared on both sides (the mirror) is drawn once;
 *  external systems are shared by name. */
export function buildLandscapeGraph(idx: AppDepIndex): AppGraph {
  const b = new GraphBuilder(idx);
  for (const { catalog } of idx.byCi.values()) b.declare(catalog, true);
  return b.build();
}

/** The token a criticality is drawn with; unknown values stay neutral. */
export function criticalityColor(c?: string): string {
  switch (c) {
    case "high": return "var(--danger)";
    case "medium": return "var(--warning)";
    case "low": return "var(--success)";
    default: return "var(--border-strong)";
  }
}

/** i18n key of a known criticality, or null (show the raw value). */
export function criticalityKey(c?: string): string | null {
  return c && (CRITICALITY_ORDER as readonly string[]).includes(c) ? `appDeps.criticality.${c}` : null;
}

// ── links to hosts, DNS, services and projetos ───────────────────────────

export const APP_LINK_TYPES: AppDepEntityType[] = ["host", "service", "dns", "project"];

export type AppLinkCounts = Record<AppDepEntityType, number>;

/** How many hosts, services, DNS records and projetos each CI links to. */
export function linkCounts(links: AppDepAssetLink[]): Map<string, AppLinkCounts> {
  const out = new Map<string, AppLinkCounts>();
  for (const l of links) {
    let c = out.get(l.ci);
    if (!c) out.set(l.ci, (c = { host: 0, service: 0, dns: 0, project: 0 }));
    c[l.entity_type]++;
  }
  return out;
}

/** i18n key of where a link comes from (its most specific source). */
export function linkSourceKey(l: Pick<AppDepAssetLink, "sources">): string {
  return `appDeps.source.${l.sources[0] ?? "repo"}`;
}

/** Whether the link was added by hand (and so can be removed here). */
export function isManual(l: Pick<AppDepAssetLink, "sources">): boolean {
  return l.sources.includes("manual");
}
