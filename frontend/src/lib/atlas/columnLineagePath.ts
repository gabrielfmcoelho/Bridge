import type { AtlasIndexes } from "./types";
import type { LineageNode, LineageEdge } from "@/lib/lineage/types";

export interface ColumnLineageStep {
  column: LineageNode;
  /** Edge that brought us here. Null for the seed column at depth 0. */
  edge: LineageEdge | null;
  /** Parent table (the table the column belongs to). */
  table: LineageNode | null;
  /** Producer of `column`'s parent table (a task or dbt model), if known. */
  producer: LineageNode | null;
  depth: number;
  /** "via" annotation copied from `edge.data.via` (e.g. "select", "join", "coalesce"). */
  via: string | null;
  /** Edge confidence: "exact", "inferred", or "dynamic". */
  confidence: string | null;
}

/**
 * Trace a single column's lineage via `column_lineage` edges.
 *
 *   direction = "upstream"   — follow edges that target this column back to their sources
 *   direction = "downstream" — follow edges that source from this column forward
 *
 * The walk is BFS with cycle protection. Depth is capped to prevent runaway
 * traversal on densely linked dbt models.
 */
export function buildColumnLineagePath(
  idx: AtlasIndexes,
  columnId: string,
  direction: "upstream" | "downstream",
  maxDepth = 8,
): ColumnLineageStep[] {
  const seed = idx.nodesById.get(columnId);
  if (!seed) return [];

  const steps: ColumnLineageStep[] = [];
  const visited = new Set<string>([columnId]);
  const queue: Array<{ id: string; edge: LineageEdge | null; depth: number }> = [
    { id: columnId, edge: null, depth: 0 },
  ];

  while (queue.length) {
    const { id, edge, depth } = queue.shift()!;
    const column = idx.nodesById.get(id);
    if (!column) continue;

    const table = column.parent ? idx.nodesById.get(column.parent) ?? null : null;
    const producer = table ? findColumnProducer(idx, table.id) : null;

    steps.push({
      column,
      edge,
      table,
      producer,
      depth,
      via: edge ? extractVia(edge) : null,
      confidence: edge ? (edge.confidence as string | undefined) ?? null : null,
    });

    if (depth >= maxDepth) continue;

    const edges = direction === "upstream"
      ? (idx.inEdges.get(id) ?? []).filter(e => e.kind === "column_lineage")
      : (idx.outEdges.get(id) ?? []).filter(e => e.kind === "column_lineage");

    for (const e of edges) {
      const nextId = direction === "upstream" ? e.source : e.target;
      if (visited.has(nextId)) continue;
      visited.add(nextId);
      queue.push({ id: nextId, edge: e, depth: depth + 1 });
    }
  }

  // Drop the seed itself; the caller already knows about it.
  return steps.slice(1);
}

function extractVia(edge: LineageEdge): string | null {
  const via = (edge.data as Record<string, unknown> | undefined)?.via;
  return typeof via === "string" ? via : null;
}

/** Find the task or dbt_model that writes a given table — for annotation. */
function findColumnProducer(idx: AtlasIndexes, tableId: string): LineageNode | null {
  for (const e of idx.inEdges.get(tableId) ?? []) {
    if (e.kind !== "writes") continue;
    const n = idx.nodesById.get(e.source);
    if (n) return n;
  }
  return null;
}
