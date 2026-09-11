"use client";

import { useState } from "react";
import { NODE_COLORS, NODE_TYPE_LABELS, edgeStyle, EDGE_LABELS } from "@/lib/lineage/style";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import StatusDot from "@/components/ui/StatusDot";
import { useLocale } from "@/contexts/LocaleContext";

const NODE_KEYS = ["dag", "task", "dbt_source", "dbt_model", "table", "script", "dbt_macro"];
const EDGE_KEYS = ["ref", "uses_source", "writes", "executes", "triggers", "column_lineage"];

export default function LineageLegend() {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  return (
    <div className="pointer-events-auto bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] text-xs shadow-[var(--shadow-md)]">
      <button
        onClick={() => setOpen(o => !o)}
        className="px-2.5 py-1.5 text-2xs uppercase tracking-wider text-[var(--text-muted)] hover:text-[var(--text-primary)] flex items-center gap-1"
      >
        {t("atlas.lineage.legend.title")}
        <Icon path={ICON_PATHS.chevronDown} className={`w-3 h-3 transition-transform ${open ? "rotate-180" : ""}`} strokeWidth={1.5} />
      </button>
      {open && (
        <div className="px-3 pb-2 space-y-2 max-w-[260px]">
          <div>
            <div className="text-3xs uppercase tracking-wider text-[var(--text-faint)] mb-1">{t("atlas.lineage.cards.nodes")}</div>
            <div className="grid grid-cols-2 gap-1">
              {NODE_KEYS.map(k => {
                const c = NODE_COLORS[k];
                return (
                  <div key={k} className="flex items-center gap-1.5">
                    <span className={`w-2.5 h-2.5 rounded ${c.bg} ${c.border} border`} />
                    <span className="text-2xs text-[var(--text-secondary)]">{NODE_TYPE_LABELS[k]}</span>
                  </div>
                );
              })}
            </div>
          </div>
          <div>
            <div className="text-3xs uppercase tracking-wider text-[var(--text-faint)] mb-1">{t("atlas.lineage.cards.edges")}</div>
            <div className="space-y-1">
              {EDGE_KEYS.map(k => {
                const s = edgeStyle(k);
                return (
                  <div key={k} className="flex items-center gap-1.5">
                    <svg width="24" height="6" viewBox="0 0 24 6">
                      <line x1="0" y1="3" x2="24" y2="3" stroke={s.stroke} strokeWidth={s.strokeWidth} strokeDasharray={s.strokeDasharray} />
                    </svg>
                    <span className="text-2xs text-[var(--text-secondary)]">{EDGE_LABELS[k]}</span>
                  </div>
                );
              })}
            </div>
          </div>
          <div>
            <div className="text-3xs uppercase tracking-wider text-[var(--text-faint)] mb-1">{t("atlas.lineage.legend.markers")}</div>
            <div className="flex items-center gap-2">
              <span className="flex items-center gap-1"><StatusDot className="bg-[var(--danger)]" /><span className="text-2xs text-[var(--text-secondary)]">{t("atlas.lineage.legend.warningMarker")}</span></span>
              <span className="flex items-center gap-1"><StatusDot className="bg-[var(--warning)]" /><span className="text-2xs text-[var(--text-secondary)]">{t("atlas.lineage.legend.gapMarker")}</span></span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
