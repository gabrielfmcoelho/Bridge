"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Breakdowns, { type BreakdownBlock } from "@/components/inventory/Breakdowns";
import SectionCard from "@/components/ui/SectionCard";
import SituacaoText from "@/components/ui/SituacaoText";
import StatusDot from "@/components/ui/StatusDot";
import type { Host, HostFilters, HostSortConfig } from "@/lib/types";
import { getTimeAgo } from "@/lib/utils";
import KpiSection from "./KpiSection";
import { hostBreakdowns, recentScans, resourcesBy, type ResourceGroup } from "./hostInsights";

/**
 * The Hosts "Dashboard" tab: the same customizable KPI tiles as the side
 * column, then one bar list per breakdown of the current listing. A row that
 * maps to a list filter applies it (and the page returns to Visão geral).
 * Below them, resource use by hospedagem / situação and the latest scans.
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
  const { t, locale } = useLocale();
  const b = hostBreakdowns(hosts);
  const blocks: BreakdownBlock<HostFilters>[] = [
    { title: t("inventory.dash.bySituacao"), rows: b.situacao },
    { title: t("host.dash.byUsage"), rows: b.usage },
    { title: t("host.dash.alertsByLevel"), rows: b.alerts },
    { title: t("host.dash.byHospedagem"), rows: b.hospedagem },
    { title: t("inventory.dash.byEntidade"), rows: b.entidade },
    { title: t("inventory.dash.byTags"), rows: b.tags },
  ];
  const recent = recentScans(hosts);

  return (
    <div className="space-y-6">
      <KpiSection hosts={hosts} filters={filters} onFiltersChange={(f) => onApplyFilter(f)} sort={sort} onSortChange={onSortChange} />
      <Breakdowns blocks={blocks} total={hosts.length} onApply={onApplyFilter} />
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_2fr_1fr] gap-4">
        <ResourceCard title={t("host.dash.resourcesByHospedagem")} groups={resourcesBy(hosts, (h) => h.hospedagem)} />
        <ResourceCard title={t("host.dash.resourcesBySituacao")} groups={resourcesBy(hosts, (h) => h.situacao)} situacao />
        <SectionCard as="h3" title={t("host.dash.recentScans")} body="flush" empty={recent.length ? undefined : t("host.dash.noScansYet")}>
          <ul>
            {recent.map((h) => (
              <li key={h.id}>
                <Link href={`/hosts/${h.oficial_slug}`} className="flex items-center gap-3 px-5 py-2 hover:bg-[var(--bg-elevated)] transition-colors">
                  <StatusDot className="bg-[var(--success)]" />
                  <span className="text-sm text-[var(--text-primary)] font-mono truncate">{h.nickname}</span>
                  <span className="text-xs text-[var(--text-muted)] ml-auto whitespace-nowrap">{getTimeAgo(h.last_scan_at!, locale)}</span>
                </Link>
              </li>
            ))}
          </ul>
        </SectionCard>
      </div>
    </div>
  );
}

function ResourceCard({ title, groups, situacao }: { title: string; groups: ResourceGroup[]; situacao?: boolean }) {
  const { t } = useLocale();
  return (
    <SectionCard as="h3" title={title} description={t("host.dash.avgUsage")} empty={groups.length ? undefined : t("host.dash.noScansYet")}>
      <div className="space-y-4">
        {groups.map((g) => (
          <div key={g.name}>
            <div className="flex items-center justify-between mb-2">
              {situacao && g.name ? <SituacaoText situacao={g.name} /> : (
                <span className="text-xs text-[var(--text-primary)] font-medium">{g.name || t("inventory.dash.none")}</span>
              )}
              <span className="text-2xs text-[var(--text-muted)] font-mono">{t("host.dash.hostCount", { count: String(g.count) })}</span>
            </div>
            <div className="grid grid-cols-3 gap-2">
              <UsageBar label="CPU" value={g.cpu} total={g.totalCpu} />
              <UsageBar label="RAM" value={g.ram} total={g.totalRam} />
              <UsageBar label={t("host.dash.disk")} value={g.disk} total={g.totalDisk} />
            </div>
          </div>
        ))}
      </div>
    </SectionCard>
  );
}

function UsageBar({ label, value, total }: { label: string; value: number; total: string }) {
  const tone = value > 80 ? "var(--danger)" : value > 60 ? "var(--warning)" : "var(--success)";
  return (
    <div>
      <div className="flex items-center justify-between mb-0.5">
        <span className="text-2xs text-[var(--text-muted)]">{label}</span>
        <span className="text-2xs font-semibold font-mono" style={{ color: tone }}>{value}%</span>
      </div>
      <div className="h-1 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
        <div className="h-full rounded-full transition duration-500" style={{ width: `${value}%`, backgroundColor: tone }} />
      </div>
      {total && <p className="text-3xs text-[var(--text-muted)] mt-0.5 text-right font-mono">{total}</p>}
    </div>
  );
}
