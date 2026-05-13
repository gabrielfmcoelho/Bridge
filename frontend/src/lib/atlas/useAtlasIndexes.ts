"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchLineage } from "@/lib/lineage/loader";
import { buildIndexes, classifyTable } from "@/lib/lineage/indexes";
import type { LineageNode } from "@/lib/lineage/types";
import type { AtlasIndexes, TableRecord, TableRole } from "./types";

export function useAtlasIndexes() {
  const query = useQuery({
    queryKey: ["lineage"],
    queryFn: ({ signal }) => fetchLineage(signal),
    staleTime: Infinity,
  });

  const indexes = useMemo<AtlasIndexes | null>(() => {
    if (!query.data) return null;
    const base = buildIndexes(query.data);

    // The lineage parser only emits column nodes for entities with a declared
    // schema (dbt models from schema.yml, sources, seeds). Tables produced by
    // raw ingestion code (most bronze) have no column nodes — yet their column
    // IDs appear as endpoints of column_lineage edges. Synthesize column nodes
    // for those references so consumers (TableDetailPanel, ColumnLineagePanel)
    // can show them as inferred columns.
    synthesizeInferredColumns(base);

    // Classify every table once.
    const tables: TableRecord[] = [];
    const tablesById = new Map<string, TableRecord>();
    const tablesByNamespace = new Map<string, TableRecord[]>();
    const namespaceSet = new Set<string>();

    for (const tableNode of base.nodesByType.get("table") ?? []) {
      const c = classifyTable(base, tableNode.id);
      const role: TableRole = c.layer === "source" ? "source" : "built";
      const columnCount = (base.childrenOf.get(tableNode.id) ?? [])
        .reduce((n, id) => n + (base.nodesById.get(id)?.type === "column" ? 1 : 0), 0);
      const hasWarning = (base.warningsByNode.get(tableNode.id) ?? 0) > 0;
      const rec: TableRecord = {
        node: tableNode,
        namespace: c.namespace,
        layer: c.layer,
        role,
        columnCount,
        hasWarning,
      };
      tables.push(rec);
      tablesById.set(tableNode.id, rec);
      const arr = tablesByNamespace.get(c.namespace) ?? [];
      arr.push(rec);
      tablesByNamespace.set(c.namespace, arr);
      namespaceSet.add(c.namespace);
    }

    // Stable sort: namespace then label.
    for (const arr of tablesByNamespace.values()) {
      arr.sort((a, b) => a.node.label.localeCompare(b.node.label));
    }
    tables.sort((a, b) =>
      a.namespace.localeCompare(b.namespace) || a.node.label.localeCompare(b.node.label));

    const KNOWN = ["servidores", "sei", "mercado"];
    const namespaces = [
      ...KNOWN.filter(n => namespaceSet.has(n)),
      ...Array.from(namespaceSet).filter(n => !KNOWN.includes(n)).sort(),
    ];

    // Reverse the `childrenOf` map into a parent lookup once.
    const parentOf = new Map<string, string>();
    for (const [parent, children] of base.childrenOf.entries()) {
      for (const child of children) parentOf.set(child, parent);
    }

    return {
      ...base,
      tables,
      tablesById,
      tablesByNamespace,
      namespaces,
      parentOf,
    } satisfies AtlasIndexes;
  }, [query.data]);

  return {
    indexes,
    isLoading: query.isLoading,
    error: query.error,
    generatedAt: query.data?.generated_at,
  };
}

/**
 * Walk every column_lineage edge and create a synthetic `column` node for any
 * endpoint that isn't already a node — provided its inferred parent table IS
 * a known node. Synthetic nodes get `data.inferred_from_sql = true` so the UI
 * can mark them visually.
 *
 * Mutates the passed `LineageIndexes` in place (nodesById, childrenOf, nodesByType).
 */
function synthesizeInferredColumns(base: ReturnType<typeof buildIndexes>) {
  const tableIds = new Set<string>();
  for (const t of base.nodesByType.get("table") ?? []) tableIds.add(t.id);

  const COL_PREFIX = "column:";
  const seen = new Set<string>();

  const tryAdd = (colId: string) => {
    if (!colId.startsWith(COL_PREFIX)) return;
    if (seen.has(colId)) return;
    seen.add(colId);
    if (base.nodesById.has(colId)) return;
    const body = colId.slice(COL_PREFIX.length);
    const lastDot = body.lastIndexOf(".");
    if (lastDot < 0) return;
    const rawParent = body.slice(0, lastDot);
    const colName = body.slice(lastDot + 1);
    if (colName.startsWith("__") || colName === "*") return; // parser artifacts (e.g. __expr__)

    // Parser inconsistency: column ids are `column:cat.sch.tbl.col` (no prefix
    // on the embedded table path), but table node ids are `table:cat.sch.tbl`.
    // Resolve by trying both: the raw parent (model:foo, source:foo) AND the
    // `table:`-prefixed form.
    let parentId: string | null = null;
    if (tableIds.has(rawParent)) parentId = rawParent;
    else if (tableIds.has(`table:${rawParent}`)) parentId = `table:${rawParent}`;
    else if (base.nodesById.has(rawParent)) parentId = rawParent; // e.g. model:foo
    if (!parentId) return;

    const synthetic: LineageNode = {
      id: colId,
      type: "column",
      label: colName,
      parent: parentId,
      namespace: base.nodesById.get(parentId)?.namespace,
      data: { inferred_from_sql: true },
    };
    base.nodesById.set(colId, synthetic);
    const arr = base.childrenOf.get(parentId) ?? [];
    arr.push(colId);
    base.childrenOf.set(parentId, arr);
    const byType = base.nodesByType.get("column") ?? [];
    byType.push(synthetic);
    base.nodesByType.set("column", byType);
  };

  for (const e of base.raw.edges) {
    if (e.kind !== "column_lineage") continue;
    tryAdd(e.source);
    tryAdd(e.target);
  }
}
