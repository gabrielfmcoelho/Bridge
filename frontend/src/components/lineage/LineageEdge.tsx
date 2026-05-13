"use client";

import { BaseEdge, getBezierPath, type EdgeProps } from "@xyflow/react";
import type { LineageEdge as LineageEdgeT } from "@/lib/lineage/types";
import { edgeStyle, EDGE_LABELS } from "@/lib/lineage/style";

export default function LineageEdgeComponent(props: EdgeProps) {
  const { sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, id, data } = props;
  const edge = (data as { edge?: LineageEdgeT } | undefined)?.edge;
  const kind = edge?.kind ?? "depends_on";
  const confidence = edge?.confidence;

  const [path, labelX, labelY] = getBezierPath({
    sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition,
  });

  const s = edgeStyle(kind, confidence);

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{
          stroke: s.stroke,
          strokeWidth: s.strokeWidth,
          strokeDasharray: s.strokeDasharray,
          fill: "none",
        }}
      />
      {kind !== "contains" && kind !== "column_lineage" && (
        <text
          x={labelX}
          y={labelY}
          textAnchor="middle"
          dominantBaseline="middle"
          style={{ fill: "var(--text-muted)", fontSize: 9, fontFamily: "var(--font-mono)", pointerEvents: "none" }}
        >
          {EDGE_LABELS[kind] ?? kind}
        </text>
      )}
    </>
  );
}
