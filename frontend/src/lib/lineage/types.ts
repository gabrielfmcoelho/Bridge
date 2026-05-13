import { z } from "zod";

// Mirrors airflow/scripts/lineage/schema.py.
// All optional fields use .optional() because the Python serializer strips
// empty values (see _strip_empty in schema.py).

export const NodeType = z.enum([
  "namespace", "dag", "task_group", "task",
  "dbt_source", "dbt_model", "dbt_seed", "dbt_macro",
  "table", "column", "external", "script",
]);
export type NodeType = z.infer<typeof NodeType>;

export const EdgeKind = z.enum([
  "contains", "depends_on", "triggers", "executes", "invokes",
  "ref", "uses_source", "reads", "writes", "column_lineage",
]);
export type EdgeKind = z.infer<typeof EdgeKind>;

export const Confidence = z.enum(["exact", "inferred", "dynamic"]);
export type Confidence = z.infer<typeof Confidence>;

export const Severity = z.enum(["info", "warning", "error"]);
export type Severity = z.infer<typeof Severity>;

export const WarningKind = z.enum([
  "unresolved_executes_target",
  "unknown_source",
  "missing_models_dir",
  "no_dag_in_file",
  "non_literal_dag_id",
  "non_literal_task_id",
  "taskgroup_missing_id",
  "unknown_dag_var",
  "unresolved_dep_name",
  "syntax_error",
  "file_read_failed",
  "yaml_load_failed",
  "csv_read_failed",
  "parser_error",
  "sql_parse_failed",
  "dynamic_columns",
  "unresolved_column_ref",
  "general",
]);
export type WarningKind = z.infer<typeof WarningKind>;

export const LineageNode = z.object({
  id: z.string(),
  type: NodeType.or(z.string()),
  label: z.string(),
  namespace: z.string().optional(),
  parent: z.string().optional(),
  layer: z.string().optional(),
  file: z.string().optional(),
  lines: z.array(z.number()).optional(),
  description: z.string().optional(),
  data: z.record(z.string(), z.unknown()).optional(),
});
export type LineageNode = z.infer<typeof LineageNode>;

export const LineageEdge = z.object({
  source: z.string(),
  target: z.string(),
  kind: EdgeKind.or(z.string()),
  confidence: Confidence.or(z.string()).optional(),
  data: z.record(z.string(), z.unknown()).optional(),
});
export type LineageEdge = z.infer<typeof LineageEdge>;

export const LineageWarning = z.object({
  kind: WarningKind.or(z.string()),
  message: z.string(),
  severity: Severity.or(z.string()).optional(),
  source_id: z.string().optional(),
  target_id: z.string().optional(),
  file: z.string().optional(),
  line: z.number().optional(),
  data: z.record(z.string(), z.unknown()).optional(),
});
export type LineageWarning = z.infer<typeof LineageWarning>;

export const LineageCoverage = z.object({
  node_total: z.number().optional(),
  edge_total: z.number().optional(),
  warning_total: z.number().optional(),
  by_node_type: z.record(z.string(), z.number()).optional(),
  by_edge_kind: z.record(z.string(), z.number()).optional(),
  gaps: z.object({
    orphan_tasks: z.array(z.string()).optional(),
    isolated_models: z.array(z.string()).optional(),
    unused_sources: z.array(z.string()).optional(),
    unused_macros: z.array(z.string()).optional(),
  }).optional(),
  column_lineage: z.object({
    edges_emitted: z.number().optional(),
    models_with_column_lineage: z.array(z.string()).optional(),
    models_without_column_lineage: z.array(z.string()).optional(),
    models_with_dynamic_columns: z.array(z.string()).optional(),
    models_with_parse_failed: z.array(z.string()).optional(),
  }).optional(),
}).passthrough();
export type LineageCoverage = z.infer<typeof LineageCoverage>;

export const Lineage = z.object({
  version: z.string().optional(),
  generated_at: z.string().optional(),
  coverage: LineageCoverage.optional(),
  warnings: z.array(LineageWarning).optional(),
  nodes: z.array(LineageNode),
  edges: z.array(LineageEdge),
});
export type Lineage = z.infer<typeof Lineage>;

// Logical layer values surfaced from the parser; "ingest" and "meta" appear
// in schema.py but aren't currently emitted.
export type Layer = "bronze" | "silver" | "gold" | "iaspi" | "iapep";

export const LAYERS: readonly Layer[] = ["bronze", "silver", "gold", "iaspi", "iapep"];
