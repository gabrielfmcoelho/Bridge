"use client";

import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import type { Host, HostFilters, HostSortConfig } from "@/lib/types";
import KpiSection from "./KpiSection";
import { hostBreakdowns, type BreakdownRow } from "./hostInsights";

/**
 * The Hosts "Dashboard" tab: the same customizable KPI tiles as the side
 * column, then one bar list per breakdown of the current listing. A row that
 * maps to a list filter applies it (and the page returns to Visão geral).
 */
export default function HostsDashboard({
  hosts,
  filters,
  sort,
  onSortChange,
  onApplyFilter,
}: {
  hosts: Host[];
  filters: HostFilters;
  sort: HostSortConfig;
  onSortChange: (s: HostSortConfig) => void;
  onApplyFilter: (f: Partial<HostFilters>) => void;
}) {
  const { t } = useLocale();
  const b = hostBreakdowns(hosts);
  const blocks: { title: string; rows: BreakdownRow[] }[] = [
    { title: t("host.dash.bySituacao"), rows: b.situacao },
    { title: t("host.dash.byUsage"), rows: b.usage },
    { title: t("host.dash.alertsByLevel"), rows: b.alerts },
    { title: t("host.dash.byHospedagem"), rows: b.hospedagem },
    { title: t("host.dash.byEntidade"), rows: b.entidade },
    { title: t("host.dash.byTags"), rows: b.tags },
  ];

  return (
    <div className="space-y-6">
      <KpiSection hosts={hosts} filters={filters} onFiltersChange={(f) => onApplyFilter(f)} sort={sort} onSortChange={onSortChange} />
      <p className="text-xs text-[var(--text-muted)]">{t("host.dash.clickToFilter")}</p>
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {blocks.map((block) => (
          <SectionCard key={block.title} as="h3" title={block.title}>
            <BarList rows={block.rows} total={hosts.length} onApply={onApplyFilter} />
          </SectionCard>
        ))}
      </div>
    </div>
  );
}

function BarList({ rows, total, onApply }: { rows: BreakdownRow[]; total: number; onApply: (f: Partial<HostFilters>) => void }) {
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
