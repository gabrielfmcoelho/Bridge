import type { LineageIndexes, TableLayer } from "@/lib/lineage/indexes";
import type { LineageNode } from "@/lib/lineage/types";

export type TableRole = "source" | "built";

export interface TableRecord {
  node: LineageNode;
  namespace: string;
  layer: TableLayer;
  role: TableRole;
  columnCount: number;
  hasWarning: boolean;
}

export interface AtlasIndexes extends LineageIndexes {
  /** All tables, classified with namespace/layer/role and column counts. */
  tables: TableRecord[];
  tablesById: Map<string, TableRecord>;
  /** Quick lookup: namespace → tables sorted by label. */
  tablesByNamespace: Map<string, TableRecord[]>;
  /** Sorted list of distinct namespaces present in the dataset. */
  namespaces: string[];
  /** child id → parent id, derived from `contains` edges (inverse of childrenOf). */
  parentOf: Map<string, string>;
}

export type CatalogViewMode = "tree" | "cards" | "split";
export const CATALOG_VIEW_MODES: readonly CatalogViewMode[] = ["tree", "cards", "split"];

export type PipelineViewMode = "groups" | "lanes";
export const PIPELINE_VIEW_MODES: readonly PipelineViewMode[] = ["groups", "lanes"];

export interface AtlasFilters {
  domains: string[];   // empty = all
  layers: string[];    // empty = all
  role: TableRole | "all";
  query: string;
}

export const EMPTY_FILTERS: AtlasFilters = {
  domains: [],
  layers: [],
  role: "all",
  query: "",
};
