"use client";

import type { NodeProps } from "@xyflow/react";
import { LAYER_GROUP_BG, type InferredLayer } from "@/lib/lineage/style";

type Data = { layer: InferredLayer; label: string };

export default function LayerGroupNode({ data, width, height }: NodeProps) {
  const { layer, label } = data as unknown as Data;
  const s = LAYER_GROUP_BG[layer];
  return (
    <div
      className="relative rounded-[var(--radius-lg)] border-2 border-dashed pointer-events-none"
      style={{
        width: width ?? 0,
        height: height ?? 0,
        background: s.bg,
        borderColor: s.border,
      }}
    >
      <span
        className={`absolute top-2 left-3 text-[10px] uppercase tracking-[0.25em] font-bold ${s.text}`}
        style={{ fontFamily: "var(--font-display)" }}
      >
        {label}
      </span>
    </div>
  );
}
