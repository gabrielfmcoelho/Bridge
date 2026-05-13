import type { AtlasIndexes } from "./types";
import type { LineageNode } from "@/lib/lineage/types";
import type { TableLayer } from "@/lib/lineage/indexes";

export interface LineageChainStep {
  layer: TableLayer;
  tables: LineageNode[];
}

/** Display order of layers in the chain breadcrumb (right to left = upstream). */
export const LAYER_ORDER: readonly TableLayer[] = ["source", "bronze", "silver", "gold", "iapep", "iaspi", "other"];

/**
 * Walk upstream from `tableId` along producer/consumer edges and group the
 * encountered tables by their classified layer.
 *
 * Edges considered (target → source traversal):
 *   writes        — task/model writes the target table; walk through that producer
 *                   back via its reads/refs/uses_source/executes
 *   ref           — dbt model references another model (which materializes a table)
 *   uses_source   — dbt model uses a declared source (with a corresponding table)
 *   executes      — task executes a dbt model that ultimately materializes a table
 *
 * The traversal is cycle-safe (visited set) and depth-limited.
 */
export function buildTableLineageChain(idx: AtlasIndexes, tableId: string, maxDepth = 6): LineageChainStep[] {
  const collected = new Map<string, LineageNode>();   // table_id → node
  const visited = new Set<string>([tableId]);
  const focal = idx.nodesById.get(tableId);
  if (focal) collected.set(focal.id, focal);

  const queue: Array<[string, number]> = [[tableId, 0]];

  while (queue.length) {
    const [cur, depth] = queue.shift()!;
    if (depth >= maxDepth) continue;
    const upstreamTables = findUpstreamTables(idx, cur, visited);
    for (const t of upstreamTables) {
      if (!collected.has(t.id)) collected.set(t.id, t);
      queue.push([t.id, depth + 1]);
    }
  }

  // Group by layer
  const byLayer = new Map<TableLayer, LineageNode[]>();
  for (const t of collected.values()) {
    const rec = idx.tablesById.get(t.id);
    const layer = rec?.layer ?? "other";
    const arr = byLayer.get(layer) ?? [];
    arr.push(t);
    byLayer.set(layer, arr);
  }
  for (const arr of byLayer.values()) arr.sort((a, b) => a.label.localeCompare(b.label));

  return LAYER_ORDER
    .filter(l => byLayer.has(l))
    .map(layer => ({ layer, tables: byLayer.get(layer)! }));
}

function findUpstreamTables(idx: AtlasIndexes, tableId: string, visited: Set<string>): LineageNode[] {
  const out: LineageNode[] = [];
  const seen = new Set<string>();

  const enqueue = (id: string) => {
    if (seen.has(id) || visited.has(id)) return;
    seen.add(id);
    visited.add(id);
    const node = idx.nodesById.get(id);
    if (node?.type === "table") out.push(node);
  };

  // 1. Producers of this table (writes target == tableId).
  for (const e of idx.inEdges.get(tableId) ?? []) {
    if (e.kind !== "writes") continue;
    const producer = e.source;
    // From producer, walk to *its* upstream tables/models via reads/ref/uses_source/executes
    for (const up of idx.outEdges.get(producer) ?? []) {
      if (up.kind === "reads") enqueue(up.target);
      else if (up.kind === "uses_source") enqueue(up.target);
      else if (up.kind === "ref") {
        // ref → dbt_model; resolve the table that model materializes
        for (const w of idx.outEdges.get(up.target) ?? []) {
          if (w.kind === "writes") enqueue(w.target);
        }
      } else if (up.kind === "executes") {
        // task executes a model → model's refs/sources resolve to tables
        const model = up.target;
        for (const me of idx.outEdges.get(model) ?? []) {
          if (me.kind === "uses_source") enqueue(me.target);
          else if (me.kind === "ref") {
            for (const w of idx.outEdges.get(me.target) ?? []) {
              if (w.kind === "writes") enqueue(w.target);
            }
          }
        }
      }
    }
  }
  // 2. Also: if this table is the target of a `ref` from a model, the model's own writes
  //    don't apply, but a dbt source backed by this table table is leaf.

  return out;
}

/** Resolve the producer (task or dbt model) that wrote a given table. */
export function findTableProducers(idx: AtlasIndexes, tableId: string): LineageNode[] {
  const out: LineageNode[] = [];
  for (const e of idx.inEdges.get(tableId) ?? []) {
    if (e.kind !== "writes") continue;
    const n = idx.nodesById.get(e.source);
    if (n) out.push(n);
  }
  return out;
}

/** Direct upstream / downstream table neighbors (for the Built from / Used by chips). */
export function findDirectNeighbors(idx: AtlasIndexes, tableId: string): {
  upstream: LineageNode[];
  downstream: LineageNode[];
} {
  const upstream = new Set<string>();
  const downstream = new Set<string>();

  // upstream: producers of this table → their reads/refs/uses_source resolve to tables
  for (const e of idx.inEdges.get(tableId) ?? []) {
    if (e.kind !== "writes") continue;
    for (const up of idx.outEdges.get(e.source) ?? []) {
      if (up.kind === "reads" || up.kind === "uses_source") upstream.add(up.target);
      else if (up.kind === "ref") {
        for (const w of idx.outEdges.get(up.target) ?? []) {
          if (w.kind === "writes") upstream.add(w.target);
        }
      }
    }
  }

  // downstream: anyone that reads / refs / uses_source this table
  for (const e of idx.outEdges.get(tableId) ?? []) {
    if (e.kind === "reads" || e.kind === "uses_source" || e.kind === "ref") {
      // e.source = consumer entity, but here e.source == tableId so target is consumer
      // wait — for "reads" we have task → table (task.source, table.target)? Let's just be
      // defensive: walk both inEdges and outEdges of the table for these kinds.
    }
  }
  for (const e of idx.inEdges.get(tableId) ?? []) {
    if (e.kind === "reads" || e.kind === "uses_source") {
      // consumer (task/model) is e.source — find what tables it ultimately writes
      for (const w of idx.outEdges.get(e.source) ?? []) {
        if (w.kind === "writes") downstream.add(w.target);
        else if (w.kind === "executes") {
          for (const me of idx.outEdges.get(w.target) ?? []) {
            if (me.kind === "ref") {
              for (const ww of idx.outEdges.get(me.target) ?? []) {
                if (ww.kind === "writes") downstream.add(ww.target);
              }
            }
          }
        }
      }
    }
  }

  upstream.delete(tableId);
  downstream.delete(tableId);
  return {
    upstream: Array.from(upstream).map(id => idx.nodesById.get(id)).filter((n): n is LineageNode => Boolean(n)),
    downstream: Array.from(downstream).map(id => idx.nodesById.get(id)).filter((n): n is LineageNode => Boolean(n)),
  };
}
