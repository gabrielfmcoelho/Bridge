"use client";

import { Handle, Position } from "@xyflow/react";
import type { LineageNode } from "@/lib/lineage/types";
import { NODE_COLORS, NODE_TYPE_LABELS, LAYER_COLORS } from "@/lib/lineage/style";

interface BaseNodeProps {
  node: LineageNode;
  emoji?: string;
  badge?: string;
  /** Render a tiny gap-marker dot in the top-right corner (orange = orphan, etc.) */
  gapMarker?: "warning" | "orphan" | "isolated" | "unused" | null;
}

export default function BaseNode({ node, emoji, badge, gapMarker }: BaseNodeProps) {
  const c = NODE_COLORS[node.type] ?? NODE_COLORS.table;
  const layerCls = node.layer ? LAYER_COLORS[node.layer] : null;

  const gapColor =
    gapMarker === "warning"  ? "bg-red-500" :
    gapMarker === "orphan"   ? "bg-amber-500" :
    gapMarker === "isolated" ? "bg-amber-500" :
    gapMarker === "unused"   ? "bg-yellow-500/70" : null;

  return (
    <div
      className={`relative w-[200px] min-h-[60px] rounded-[var(--radius-md)] border ${c.border} ${c.bg} backdrop-blur-sm px-3 py-2 shadow-sm hover:shadow-md transition-shadow cursor-pointer`}
      title={node.label}
    >
      <Handle type="target" position={Position.Left} className="!w-1.5 !h-1.5 !bg-[var(--border-strong)]" />
      <Handle type="source" position={Position.Right} className="!w-1.5 !h-1.5 !bg-[var(--border-strong)]" />

      {gapColor && (
        <span className={`absolute -top-1 -right-1 w-2.5 h-2.5 rounded-full ${gapColor} ring-2 ring-[var(--bg-base)]`} />
      )}

      <div className="flex items-center gap-1.5">
        {emoji && <span className="text-[11px]">{emoji}</span>}
        <span className={`text-[10px] font-semibold uppercase tracking-wider ${c.text}`}>
          {NODE_TYPE_LABELS[node.type] ?? node.type}
        </span>
        {badge && (
          <span className="ml-auto text-[9px] px-1.5 py-0.5 rounded-full bg-black/20 text-[var(--text-muted)]">
            {badge}
          </span>
        )}
      </div>
      <div className="text-[12px] font-medium text-[var(--text-primary)] mt-0.5 truncate" style={{ fontFamily: "var(--font-mono)" }}>
        {node.label}
      </div>
      {layerCls && (
        <div className={`text-[9px] uppercase tracking-wider mt-0.5 ${layerCls}`}>{node.layer}</div>
      )}
    </div>
  );
}
