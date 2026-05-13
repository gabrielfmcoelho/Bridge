"use client";

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { memo } from "react";
import type { LineageNode } from "@/lib/lineage/types";
import { NODE_COLORS, NODE_TYPE_LABELS } from "@/lib/lineage/style";
import type { TableLayer } from "@/lib/lineage/indexes";
import { getLayerStyle } from "../../shared/LayerBadge";

interface Data {
  node: LineageNode;
  lane?: TableLayer;
  isExternal?: boolean;
}

function EntityNodeImpl({ data, selected }: NodeProps) {
  const { node, lane } = data as unknown as Data;
  const color = NODE_COLORS[node.type] ?? NODE_COLORS.table;
  const layerStyle = lane ? getLayerStyle(lane) : null;

  return (
    <div
      className={`relative group flex flex-col rounded-[var(--radius-md)] border bg-[var(--bg-surface)] overflow-hidden transition-all ${
        selected
          ? "border-[var(--accent)] ring-2 ring-[var(--accent)]/30 shadow-[var(--shadow-md)]"
          : `${color.border} hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-sm)]`
      }`}
      style={{ width: "100%", height: "100%" }}
    >
      <Handle type="target" position={Position.Left} className="!w-1.5 !h-1.5 !bg-[var(--border-strong)] !border-0" />
      <Handle type="source" position={Position.Right} className="!w-1.5 !h-1.5 !bg-[var(--border-strong)] !border-0" />

      {/* Layer stripe (for swimlane mode) */}
      {layerStyle && <span className={`h-[2px] w-full ${layerStyle.dot}`} />}

      <div className="flex flex-col gap-1 px-2.5 py-1.5 flex-1 min-h-0">
        <div className="flex items-center gap-1.5">
          <span className={`text-[9px] uppercase tracking-[0.12em] px-1.5 py-0.5 rounded ${color.bg} ${color.text} font-semibold shrink-0`}>
            {NODE_TYPE_LABELS[node.type] ?? node.type}
          </span>
          {node.layer && (
            <span className="text-[9px] uppercase tracking-wider text-[var(--text-faint)] shrink-0">
              {node.layer}
            </span>
          )}
        </div>
        <span
          className="text-[12px] font-mono text-[var(--text-primary)] truncate"
          title={node.id}
        >
          {node.label}
        </span>
        {node.namespace && (
          <span className="text-[9px] text-[var(--text-faint)] truncate">{node.namespace}</span>
        )}
      </div>
    </div>
  );
}

export default memo(EntityNodeImpl);
