"use client";

import { useState } from "react";
import { NODE_COLORS, NODE_TYPE_LABELS, edgeStyle, EDGE_LABELS } from "@/lib/lineage/style";

const NODE_KEYS = ["dag", "task", "dbt_source", "dbt_model", "table", "script", "dbt_macro"];
const EDGE_KEYS = ["ref", "uses_source", "writes", "executes", "triggers", "column_lineage"];

export default function LineageLegend() {
  const [open, setOpen] = useState(false);
  return (
    <div className="pointer-events-auto bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] text-xs shadow-[var(--shadow-md)]">
      <button
        onClick={() => setOpen(o => !o)}
        className="px-2.5 py-1.5 text-[10px] uppercase tracking-wider text-[var(--text-muted)] hover:text-[var(--text-primary)] flex items-center gap-1"
      >
        Legend
        <svg className={`w-3 h-3 transition-transform ${open ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && (
        <div className="px-3 pb-2 space-y-2 max-w-[260px]">
          <div>
            <div className="text-[9px] uppercase tracking-wider text-[var(--text-faint)] mb-1">Nodes</div>
            <div className="grid grid-cols-2 gap-1">
              {NODE_KEYS.map(k => {
                const c = NODE_COLORS[k];
                return (
                  <div key={k} className="flex items-center gap-1.5">
                    <span className={`w-2.5 h-2.5 rounded ${c.bg} ${c.border} border`} />
                    <span className="text-[10px] text-[var(--text-secondary)]">{NODE_TYPE_LABELS[k]}</span>
                  </div>
                );
              })}
            </div>
          </div>
          <div>
            <div className="text-[9px] uppercase tracking-wider text-[var(--text-faint)] mb-1">Edges</div>
            <div className="space-y-1">
              {EDGE_KEYS.map(k => {
                const s = edgeStyle(k);
                return (
                  <div key={k} className="flex items-center gap-1.5">
                    <svg width="24" height="6" viewBox="0 0 24 6">
                      <line x1="0" y1="3" x2="24" y2="3" stroke={s.stroke} strokeWidth={s.strokeWidth} strokeDasharray={s.strokeDasharray} />
                    </svg>
                    <span className="text-[10px] text-[var(--text-secondary)]">{EDGE_LABELS[k]}</span>
                  </div>
                );
              })}
            </div>
          </div>
          <div>
            <div className="text-[9px] uppercase tracking-wider text-[var(--text-faint)] mb-1">Markers</div>
            <div className="flex items-center gap-2">
              <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-red-500" /><span className="text-[10px] text-[var(--text-secondary)]">warning</span></span>
              <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-amber-500" /><span className="text-[10px] text-[var(--text-secondary)]">gap</span></span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
