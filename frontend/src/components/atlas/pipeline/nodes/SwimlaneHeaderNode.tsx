"use client";

import { memo } from "react";
import type { NodeProps } from "@xyflow/react";
import type { TableLayer } from "@/lib/lineage/indexes";
import { useLocale } from "@/contexts/LocaleContext";
import { getLayerStyle } from "../../shared/LayerBadge";

interface Data {
  lane: TableLayer;
  label: string;
}

function SwimlaneHeaderNodeImpl({ data }: NodeProps) {
  const { t } = useLocale();
  const d = data as unknown as Data;
  const s = getLayerStyle(d.lane);
  // Try to look up a translated label if defined.
  const localized = (() => {
    try {
      const key = `atlas.pipeline.lane.${d.lane}`;
      const v = t(key);
      return v && v !== key ? v : d.label;
    } catch {
      return d.label;
    }
  })();

  return (
    <div
      className={`relative w-full h-full rounded-[var(--radius-md)] border ${s.bg} ${s.border} flex items-center px-4 pointer-events-none overflow-hidden`}
    >
      <div className="flex items-center gap-2">
        <span className={`w-2 h-2 rounded-full ${s.dot}`} />
        <span
          className={`text-sm font-bold uppercase tracking-[0.16em] ${s.text}`}
          style={{ fontFamily: "var(--font-display)" }}
        >
          {localized}
        </span>
      </div>
      <span className="absolute right-3 top-1.5 text-[9px] uppercase tracking-wider text-[var(--text-faint)] font-mono">
        {d.lane}
      </span>
    </div>
  );
}

export default memo(SwimlaneHeaderNodeImpl);
