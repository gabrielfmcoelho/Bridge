"use client";

import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import type { BreakdownRow } from "@/lib/insights";

export interface BreakdownBlock<F> { title: string; rows: BreakdownRow<F>[] }

/**
 * A Dashboard tab's body: one bar list per breakdown of the current listing.
 * A row that maps to a list filter applies it (the page then returns to
 * Visão geral). Hosts, DNS and Serviços share it.
 */
export default function Breakdowns<F>({ blocks, total, onApply }: {
  blocks: BreakdownBlock<F>[];
  total: number;
  onApply: (f: Partial<F>) => void;
}) {
  const { t } = useLocale();
  return (
    <>
      <p className="text-xs text-[var(--text-muted)]">{t("inventory.dash.clickToFilter")}</p>
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {blocks.map((block) => (
          <SectionCard key={block.title} as="h3" title={block.title}>
            <BarList rows={block.rows} total={total} onApply={onApply} />
          </SectionCard>
        ))}
      </div>
    </>
  );
}

function BarList<F>({ rows, total, onApply }: { rows: BreakdownRow<F>[]; total: number; onApply: (f: Partial<F>) => void }) {
  const { t } = useLocale();
  if (rows.length === 0) return <p className="text-sm text-[var(--text-muted)]">–</p>;
  // ponytail: top 8 rows; a long tail (tags) would need a "ver todos" later.
  return (
    <ul className="space-y-1">
      {rows.slice(0, 8).map((r) => {
        const label = r.labelKey ? t(r.label) : r.label;
        const pct = total ? (r.count / total) * 100 : 0;
        const body = (
          <>
            <span className="flex items-center justify-between gap-2 text-sm">
              <span className="truncate text-[var(--text-secondary)]">{label}</span>
              <span className={`font-mono tabular-nums ${r.count ? "text-[var(--text-primary)]" : "text-[var(--text-muted)]"}`}>{r.count}</span>
            </span>
            <span aria-hidden className="mt-1 block h-1.5 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
              <span className="block h-full rounded-full bg-[var(--accent)]" style={{ width: `${pct}%` }} />
            </span>
          </>
        );
        return (
          <li key={r.key}>
            {r.filter ? (
              <button
                type="button"
                onClick={() => onApply(r.filter!)}
                className="block w-full text-left rounded-[var(--radius-sm)] px-1.5 py-1 -mx-1.5 hover:bg-[var(--bg-elevated)] transition-colors duration-100"
              >
                {body}
              </button>
            ) : (
              <div className="px-1.5 py-1 -mx-1.5">{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
