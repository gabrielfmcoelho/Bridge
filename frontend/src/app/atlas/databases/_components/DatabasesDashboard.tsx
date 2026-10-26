"use client";

import { useLocale } from "@/contexts/LocaleContext";
import InsightKpis from "@/components/inventory/InsightKpis";
import Breakdowns, { type BreakdownBlock } from "@/components/inventory/Breakdowns";
import type { Database } from "@/lib/types";
import { useApiLinkNames } from "@/app/atlas/apis/_components/apiDisplay";
import { databaseInsights, databaseBreakdowns, DEFAULT_DATABASE_INSIGHTS, type DatabaseFilters } from "./databaseInsights";

/** The databases "Dashboard" tab: KPI tiles, then engine, environment,
 *  sensitivity and project as bar lists. A row that maps to a filter applies it. */
export default function DatabasesDashboard({ dbs, filters, onApplyFilter }: {
  dbs: Database[];
  filters: DatabaseFilters;
  onApplyFilter: (f: Partial<DatabaseFilters>) => void;
}) {
  const { t } = useLocale();
  const names = useApiLinkNames();
  const b = databaseBreakdowns(dbs, (id) => names.project.get(id));
  const blocks: BreakdownBlock<DatabaseFilters>[] = [
    { title: t("atlas.databases.dash.byEngine"), rows: b.engine },
    { title: t("atlas.databases.dash.byEnvironment"), rows: b.environment },
    { title: t("atlas.databases.dash.bySensitivity"), rows: b.sensitivity },
    { title: t("atlas.databases.dash.byProject"), rows: b.projects },
  ];
  return (
    <div className="space-y-6">
      <InsightKpis insights={databaseInsights(dbs, t)} defaults={DEFAULT_DATABASE_INSIGHTS} storageKey="databases_kpis"
        filters={filters} onFiltersChange={(f) => onApplyFilter(f)} />
      <Breakdowns blocks={blocks} total={dbs.length} onApply={onApplyFilter} />
    </div>
  );
}
