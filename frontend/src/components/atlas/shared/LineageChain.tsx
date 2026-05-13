"use client";

import { useLocale } from "@/contexts/LocaleContext";
import type { LineageChainStep } from "@/lib/atlas/tableLineageChain";
import LayerBadge from "./LayerBadge";

interface Props {
  steps: LineageChainStep[];
  focalTableId: string;
  onSelectTable: (id: string) => void;
}

/**
 * Horizontal stepped breadcrumb showing the layered chain a table is built from.
 * Each step is a column; tables in the same layer stack vertically.
 *
 * Visual language: layer-colored stripe across the top, mono-font table chips,
 * an arrow glyph between steps. The focal table glows.
 */
export default function LineageChain({ steps, focalTableId, onSelectTable }: Props) {
  const { t } = useLocale();

  if (steps.length === 0 || (steps.length === 1 && steps[0].tables.length === 1)) {
    return (
      <p className="text-xs text-[var(--text-muted)] italic px-1 py-2">
        {t("atlas.catalog.detail.noLineage")}
      </p>
    );
  }

  return (
    <div className="overflow-x-auto -mx-1">
      <div className="inline-flex items-stretch gap-1 px-1 min-w-full">
        {steps.map((step, i) => (
          <ChainStep
            key={step.layer}
            step={step}
            focalTableId={focalTableId}
            onSelectTable={onSelectTable}
            isLast={i === steps.length - 1}
          />
        ))}
      </div>
      <p className="text-[10px] text-[var(--text-faint)] mt-2 px-1 uppercase tracking-wider">
        {t("atlas.catalog.detail.lineageChainHint")}
      </p>
    </div>
  );
}

function ChainStep({
  step,
  focalTableId,
  onSelectTable,
  isLast,
}: {
  step: LineageChainStep;
  focalTableId: string;
  onSelectTable: (id: string) => void;
  isLast: boolean;
}) {
  return (
    <div className="flex items-stretch">
      <div className="flex flex-col gap-1.5 min-w-[180px] max-w-[240px]">
        <LayerBadge layer={step.layer} size="sm" />
        <div className="flex flex-col gap-1">
          {step.tables.map(t => {
            const isFocal = t.id === focalTableId;
            return (
              <button
                key={t.id}
                onClick={() => onSelectTable(t.id)}
                className={`text-left px-2 py-1.5 rounded-[var(--radius-sm)] border text-[11px] font-mono truncate transition-all ${
                  isFocal
                    ? "bg-[var(--accent-muted)] border-[var(--accent)]/40 text-[var(--accent)] shadow-[0_0_0_1px_var(--accent)/0.2,0_4px_12px_-4px_var(--accent)/0.4]"
                    : "bg-[var(--bg-base)] border-[var(--border-subtle)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:border-[var(--border-default)] hover:text-[var(--text-primary)]"
                }`}
                title={t.id}
              >
                {t.label}
              </button>
            );
          })}
        </div>
      </div>
      {!isLast && (
        <div className="flex items-center px-1 text-[var(--text-faint)]">
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
          </svg>
        </div>
      )}
    </div>
  );
}
