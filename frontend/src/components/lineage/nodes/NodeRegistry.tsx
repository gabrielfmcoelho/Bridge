"use client";

import type { NodeProps } from "@xyflow/react";
import type { LineageNode } from "@/lib/lineage/types";
import BaseNode from "./BaseNode";
import LayerGroupNode from "./LayerGroupNode";

// One xyflow node-type per source type. They all defer to BaseNode but pass
// emoji/badge hints so the graph stays scannable.

type NodeData = { node: LineageNode; gapMarker?: "warning" | "orphan" | "isolated" | "unused" | null };

function withEmoji(emoji: string, getBadge?: (n: LineageNode) => string | undefined) {
  return function Wrapped({ data }: NodeProps) {
    const d = data as unknown as NodeData;
    return <BaseNode node={d.node} emoji={emoji} badge={getBadge?.(d.node)} gapMarker={d.gapMarker} />;
  };
}

export const NamespaceNode = withEmoji("📁");
export const DagNode = withEmoji("🪂", (n) => (n.data?.schedule_interval as string | undefined) ?? undefined);
export const TaskGroupNode = withEmoji("▢");
export const TaskNode = withEmoji("⚙", (n) => (n.data?.operator as string | undefined)?.replace("Operator", ""));
export const DbtSourceNode = withEmoji("⛁");
export const DbtModelNode = withEmoji("◫", (n) => (n.data?.materialized as string | undefined));
export const DbtSeedNode = withEmoji("🌱");
export const DbtMacroNode = withEmoji("⬡");
export const TableNode = withEmoji("⛁");
export const ColumnNode = withEmoji("◌");
export const ScriptNode = withEmoji("</>");
export const ExternalNode = withEmoji("🌐");

export const NODE_TYPES = {
  namespace: NamespaceNode,
  dag: DagNode,
  task_group: TaskGroupNode,
  task: TaskNode,
  dbt_source: DbtSourceNode,
  dbt_model: DbtModelNode,
  dbt_seed: DbtSeedNode,
  dbt_macro: DbtMacroNode,
  table: TableNode,
  column: ColumnNode,
  script: ScriptNode,
  external: ExternalNode,
  layer_group: LayerGroupNode,
};
