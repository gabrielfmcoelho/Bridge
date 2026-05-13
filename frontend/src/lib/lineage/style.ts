// Node/edge visual encoding for the lineage graph. Colors map to tailwind
// classes used inside custom node components; edge styles are inline.

import type { Confidence, EdgeKind } from "./types";

export const NODE_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  namespace:  { bg: "bg-slate-500/15",   text: "text-slate-300",   border: "border-slate-500/40"   },
  dag:        { bg: "bg-cyan-500/15",    text: "text-cyan-300",    border: "border-cyan-500/40"    },
  task_group: { bg: "bg-gray-500/15",    text: "text-gray-300",    border: "border-gray-500/40"    },
  task:       { bg: "bg-purple-500/15",  text: "text-purple-300",  border: "border-purple-500/40"  },
  dbt_source: { bg: "bg-blue-500/15",    text: "text-blue-300",    border: "border-blue-500/40"    },
  dbt_model:  { bg: "bg-emerald-500/15", text: "text-emerald-300", border: "border-emerald-500/40" },
  dbt_seed:   { bg: "bg-yellow-500/15",  text: "text-yellow-300",  border: "border-yellow-500/40"  },
  dbt_macro:  { bg: "bg-indigo-500/15",  text: "text-indigo-300",  border: "border-indigo-500/40"  },
  table:      { bg: "bg-sky-500/15",     text: "text-sky-300",     border: "border-sky-500/40"     },
  column:     { bg: "bg-zinc-500/15",    text: "text-zinc-300",    border: "border-zinc-500/40"    },
  script:     { bg: "bg-orange-500/15",  text: "text-orange-300",  border: "border-orange-500/40"  },
  external:   { bg: "bg-rose-500/15",    text: "text-rose-300",    border: "border-rose-500/40"    },
};

export const LAYER_COLORS: Record<string, string> = {
  bronze: "text-amber-700",
  silver: "text-slate-300",
  gold:   "text-yellow-400",
  iaspi:  "text-pink-300",
  iapep:  "text-violet-300",
};

// Minimap color per node type
export function minimapColor(type: string): string {
  switch (type) {
    case "namespace":  return "#94a3b8";
    case "dag":        return "#22d3ee";
    case "task_group": return "#9ca3af";
    case "task":       return "#c4b5fd";
    case "dbt_source": return "#60a5fa";
    case "dbt_model":  return "#6ee7b7";
    case "dbt_seed":   return "#fde047";
    case "dbt_macro":  return "#a5b4fc";
    case "table":      return "#7dd3fc";
    case "column":     return "#a1a1aa";
    case "script":     return "#fdba74";
    case "external":   return "#fda4af";
    default:           return "#5a6a80";
  }
}

export interface EdgeStyle {
  stroke: string;
  strokeWidth: number;
  strokeDasharray?: string;
  animated?: boolean;
  markerLabel?: string;
}

export function edgeStyle(kind: EdgeKind | string, confidence?: Confidence | string): EdgeStyle {
  const base: EdgeStyle = (() => {
    switch (kind) {
      case "contains":       return { stroke: "var(--border-subtle)", strokeWidth: 1, strokeDasharray: "3 3" };
      case "depends_on":     return { stroke: "var(--border-default)", strokeWidth: 1 };
      case "triggers":       return { stroke: "#3b82f6", strokeWidth: 2, strokeDasharray: "5 4", animated: true };
      case "executes":       return { stroke: "#10b981", strokeWidth: 1.5, animated: true };
      case "invokes":        return { stroke: "#f97316", strokeWidth: 1.5, animated: true };
      case "ref":            return { stroke: "var(--text-primary)", strokeWidth: 1.5 };
      case "uses_source":    return { stroke: "#60a5fa", strokeWidth: 1.5 };
      case "reads":          return { stroke: "#7dd3fc", strokeWidth: 1 };
      case "writes":         return { stroke: "#f87171", strokeWidth: 2 };
      case "column_lineage": return { stroke: "#a3a3a3", strokeWidth: 0.75 };
      default:               return { stroke: "var(--border-default)", strokeWidth: 1 };
    }
  })();

  if (confidence === "inferred") {
    base.strokeDasharray = base.strokeDasharray ?? "4 3";
  } else if (confidence === "dynamic") {
    base.strokeDasharray = "1 2";
  }
  return base;
}

export const EDGE_LABELS: Record<string, string> = {
  contains: "contains",
  depends_on: "depends on",
  triggers: "triggers",
  executes: "executes",
  invokes: "invokes",
  ref: "ref",
  uses_source: "uses source",
  reads: "reads",
  writes: "writes",
  column_lineage: "col",
};

export const NODE_TYPE_LABELS: Record<string, string> = {
  namespace: "Namespace",
  dag: "DAG",
  task_group: "Task Group",
  task: "Task",
  dbt_source: "dbt Source",
  dbt_model: "dbt Model",
  dbt_seed: "dbt Seed",
  dbt_macro: "dbt Macro",
  table: "Table",
  column: "Column",
  script: "Script",
  external: "External",
};

// ---------- Layer inference for DAGs ----------------------------------------

export type InferredLayer = "bronze" | "silver" | "gold" | "iapep" | "iaspi" | "other";
export const INFERRED_LAYER_ORDER: readonly InferredLayer[] = ["bronze", "silver", "gold", "iapep", "iaspi", "other"];

const _LAYER_PATTERNS: Array<[RegExp, InferredLayer]> = [
  [/(^|_)bronze(_|$)/i, "bronze"],
  [/(^|_)silver(_|$)/i, "silver"],
  [/(^|_)gold(_|$)/i,   "gold"],
  [/(^|_)iapep(_|$)/i,  "iapep"],
  [/(^|_)iaspi(_|$)/i,  "iaspi"],
];

export function inferLayer(label: string): InferredLayer {
  for (const [re, layer] of _LAYER_PATTERNS) if (re.test(label)) return layer;
  return "other";
}

export const LAYER_GROUP_BG: Record<InferredLayer, { bg: string; border: string; text: string }> = {
  bronze: { bg: "rgba(217,119,6,0.06)",   border: "rgba(217,119,6,0.30)",   text: "text-amber-600" },
  silver: { bg: "rgba(148,163,184,0.06)", border: "rgba(148,163,184,0.30)", text: "text-slate-300" },
  gold:   { bg: "rgba(234,179,8,0.06)",   border: "rgba(234,179,8,0.30)",   text: "text-yellow-400" },
  iapep:  { bg: "rgba(139,92,246,0.06)",  border: "rgba(139,92,246,0.30)",  text: "text-violet-300" },
  iaspi:  { bg: "rgba(236,72,153,0.06)",  border: "rgba(236,72,153,0.30)",  text: "text-pink-300" },
  other:  { bg: "rgba(107,114,128,0.05)", border: "rgba(107,114,128,0.20)", text: "text-gray-400" },
};
