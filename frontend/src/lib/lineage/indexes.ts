import type { Lineage, LineageEdge, LineageNode } from "./types";

export interface LineageIndexes {
  raw: Lineage;
  nodesById: Map<string, LineageNode>;
  outEdges: Map<string, LineageEdge[]>;     // edges where source == id
  inEdges: Map<string, LineageEdge[]>;      // edges where target == id
  childrenOf: Map<string, string[]>;         // parent → child ids via contains
  nodesByType: Map<string, LineageNode[]>;
  nodesByNamespace: Map<string, LineageNode[]>;
  modelsByLayer: Map<string, LineageNode[]>;
  warningsByNode: Map<string, number>;
}

export function buildIndexes(lineage: Lineage): LineageIndexes {
  const nodesById = new Map<string, LineageNode>();
  const outEdges = new Map<string, LineageEdge[]>();
  const inEdges = new Map<string, LineageEdge[]>();
  const childrenOf = new Map<string, string[]>();
  const nodesByType = new Map<string, LineageNode[]>();
  const nodesByNamespace = new Map<string, LineageNode[]>();
  const modelsByLayer = new Map<string, LineageNode[]>();
  const warningsByNode = new Map<string, number>();

  for (const n of lineage.nodes) {
    nodesById.set(n.id, n);
    push(nodesByType, n.type, n);
    if (n.namespace) push(nodesByNamespace, n.namespace, n);
    if (n.type === "dbt_model" && n.layer) push(modelsByLayer, n.layer, n);
  }

  for (const e of lineage.edges) {
    push(outEdges, e.source, e);
    push(inEdges, e.target, e);
    if (e.kind === "contains") {
      const arr = childrenOf.get(e.source) ?? [];
      arr.push(e.target);
      childrenOf.set(e.source, arr);
    }
  }

  for (const w of lineage.warnings ?? []) {
    if (w.source_id) {
      warningsByNode.set(w.source_id, (warningsByNode.get(w.source_id) ?? 0) + 1);
    }
  }

  return {
    raw: lineage,
    nodesById, outEdges, inEdges, childrenOf,
    nodesByType, nodesByNamespace, modelsByLayer,
    warningsByNode,
  };
}

function push<K, V>(map: Map<K, V[]>, key: K, value: V) {
  const arr = map.get(key);
  if (arr) arr.push(value); else map.set(key, [value]);
}

export function neighborsOut(idx: LineageIndexes, id: string, kinds?: string[]): LineageEdge[] {
  const all = idx.outEdges.get(id) ?? [];
  return kinds ? all.filter(e => kinds.includes(e.kind)) : all;
}

export function neighborsIn(idx: LineageIndexes, id: string, kinds?: string[]): LineageEdge[] {
  const all = idx.inEdges.get(id) ?? [];
  return kinds ? all.filter(e => kinds.includes(e.kind)) : all;
}

// ---------- Tables-by-domain classification ----------------------------------

export type TableLayer = "bronze" | "silver" | "gold" | "iapep" | "iaspi" | "source" | "other";
export const TABLE_LAYERS: readonly TableLayer[] = ["bronze", "silver", "gold", "iapep", "iaspi", "source", "other"];

export interface TableClassification {
  namespace: string;   // "servidores" | "sei" | "mercado" | "shared"
  layer: TableLayer;
}

const _LAYER_PATTERN = /(bronze|silver|gold|iapep|iaspi)/i;

/** Classify a `table` node into (namespace, layer) using surrounding edges
 *  and the table's catalog/schema metadata. Deterministic + O(1) per call. */
export function classifyTable(idx: LineageIndexes, tableId: string): TableClassification {
  const n = idx.nodesById.get(tableId);
  if (!n || n.type !== "table") return { namespace: "shared", layer: "other" };

  // --- Layer -----------------------------------------------------------------
  let layer: TableLayer = "other";

  // 1. Materialized by a model? Inherit its layer.
  for (const e of idx.inEdges.get(tableId) ?? []) {
    if (e.kind !== "writes") continue;
    const src = idx.nodesById.get(e.source);
    if (src?.type === "dbt_model" && src.layer && _isLayer(src.layer)) {
      layer = src.layer as TableLayer;
      break;
    }
  }

  // 2. Declared as a source in sources.yml?
  if (layer === "other" && n.data?.source_group) {
    layer = "source";
  }

  // 3. Catalog / schema hints.
  if (layer === "other") {
    const catalog = String(n.data?.catalog ?? "").toLowerCase();
    const schema = String(n.data?.schema ?? "").toLowerCase();
    if (catalog === "siape" || catalog.startsWith("oracle")) {
      layer = "source";
    } else {
      const m = _LAYER_PATTERN.exec(schema);
      if (m) layer = m[1].toLowerCase() as TableLayer;
    }
  }

  // --- Namespace -------------------------------------------------------------
  let namespace = "shared";

  // 1. Any incident edge to a node carrying a namespace.
  const adjacent = [
    ...(idx.outEdges.get(tableId) ?? []),
    ...(idx.inEdges.get(tableId) ?? []),
  ];
  for (const e of adjacent) {
    const otherId = e.source === tableId ? e.target : e.source;
    const other = idx.nodesById.get(otherId);
    if (other?.namespace) { namespace = other.namespace; break; }
    if (other?.type === "dbt_model" && other.data?.schema) {
      // dbt schemas follow "<domain>_<layer>" → derive domain prefix.
      const sch = String(other.data.schema);
      const i = sch.indexOf("_");
      if (i > 0) { namespace = sch.slice(0, i); break; }
    }
  }

  // 2. Catalog convention fallback.
  if (namespace === "shared") {
    const catalog = String(n.data?.catalog ?? "").toLowerCase();
    if (catalog === "siape") namespace = "servidores";
  }

  // 3. Schema-prefix fallback (e.g., "servidores_bronze").
  if (namespace === "shared") {
    const sch = String(n.data?.schema ?? "");
    const i = sch.indexOf("_");
    if (i > 0) namespace = sch.slice(0, i);
  }

  return { namespace, layer };
}

function _isLayer(s: string): s is TableLayer {
  return s === "bronze" || s === "silver" || s === "gold" || s === "iapep" || s === "iaspi";
}

/** Build the full (namespace, layer) → table[] matrix once. */
export function buildTableMatrix(idx: LineageIndexes): {
  matrix: Map<string, Map<TableLayer, LineageNode[]>>;
  namespaces: string[];
  classification: Map<string, TableClassification>;
} {
  const matrix = new Map<string, Map<TableLayer, LineageNode[]>>();
  const classification = new Map<string, TableClassification>();
  const namespaces = new Set<string>();
  for (const t of idx.nodesByType.get("table") ?? []) {
    const c = classifyTable(idx, t.id);
    classification.set(t.id, c);
    namespaces.add(c.namespace);
    const row = matrix.get(c.namespace) ?? new Map<TableLayer, LineageNode[]>();
    const cell = row.get(c.layer) ?? [];
    cell.push(t);
    row.set(c.layer, cell);
    matrix.set(c.namespace, row);
  }
  // Sort cells alphabetically by table label for stable rendering.
  for (const row of matrix.values()) {
    for (const cell of row.values()) cell.sort((a, b) => a.label.localeCompare(b.label));
  }
  // Order: known domains first, then alpha.
  const KNOWN = ["servidores", "sei", "mercado"];
  const ordered = [
    ...KNOWN.filter(n => namespaces.has(n)),
    ...Array.from(namespaces).filter(n => !KNOWN.includes(n)).sort(),
  ];
  return { matrix, namespaces: ordered, classification };
}
