"use client";

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { memo } from "react";
import type { LineageNode } from "@/lib/lineage/types";
import { useLocale } from "@/contexts/LocaleContext";

interface Data {
  node: LineageNode;
  collapsed: boolean;
  childCount: number;
}

function DagFrameNodeImpl({ data, selected }: NodeProps) {
  const { t } = useLocale();
  const { node, collapsed, childCount } = data as unknown as Data;
  const schedule = (node.data as Record<string, unknown> | undefined)?.schedule;

  return (
    <div
      className={`relative w-full h-full rounded-[var(--radius-md)] border bg-[var(--bg-base)]/40 transition overflow-hidden ${
        selected
          ? "border-[var(--accent)] ring-2 ring-[var(--accent)]/30"
          : "border-[var(--cyan)]/30 hover:border-[var(--cyan)]/50"
      }`}
    >
      <Handle type="target" position={Position.Left} className="!w-1.5 !h-1.5 !bg-[var(--cyan)] !border-0" />
      <Handle type="source" position={Position.Right} className="!w-1.5 !h-1.5 !bg-[var(--cyan)] !border-0" />

      <div className="absolute top-0 left-0 right-0 h-7 px-2.5 flex items-center gap-1.5 bg-[var(--cyan)]/10 border-b border-[var(--cyan)]/30">
        <span className="text-3xs uppercase tracking-[0.14em] text-[var(--cyan)] font-semibold shrink-0">DAG</span>
        <span className="text-xs font-mono text-[var(--text-primary)] truncate flex-1" title={node.id}>
          {node.label}
        </span>
        {Boolean(schedule) && (
          <span className="text-3xs uppercase tracking-wider text-[var(--cyan)]/70 font-mono shrink-0">
            {String(schedule)}
          </span>
        )}
      </div>

      {collapsed && (
        <div className="absolute inset-0 top-7 flex items-center justify-center text-xs text-[var(--text-muted)]">
          <span className="px-2 py-1 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)]">
            {t("atlas.pipeline.group.nodesCollapsed", { count: String(childCount) })}
          </span>
        </div>
      )}
    </div>
  );
}

export default memo(DagFrameNodeImpl);
