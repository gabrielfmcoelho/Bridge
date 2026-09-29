"use client";

import { useLocale } from "@/contexts/LocaleContext";
import InsightKpis from "@/components/inventory/InsightKpis";
import Breakdowns, { type BreakdownBlock } from "@/components/inventory/Breakdowns";
import type { Project } from "@/lib/types";
import type { ProjectFilters } from "../FilterDrawer";
import { projectInsights, projectBreakdowns, DEFAULT_PROJECT_INSIGHTS } from "./projectInsights";

/** The Projetos "Dashboard" tab: KPI tiles, then situação, setor, entidade,
 *  projects with most services / open issues, and tags. */
export default function ProjectsDashboard({ projects, filters, onApplyFilter }: {
  projects: Project[];
  filters: ProjectFilters;
  onApplyFilter: (f: Partial<ProjectFilters>) => void;
}) {
  const { t } = useLocale();
  const b = projectBreakdowns(projects);
  const blocks: BreakdownBlock<ProjectFilters>[] = [
    { title: t("inventory.dash.bySituacao"), rows: b.situacao },
    { title: t("project.dash.bySetor"), rows: b.setor },
    { title: t("inventory.dash.byEntidade"), rows: b.entidade },
    { title: t("project.dash.byServices"), rows: b.byServices },
    { title: t("project.dash.byIssues"), rows: b.byIssues },
    { title: t("inventory.dash.byTags"), rows: b.tags },
  ];
  return (
    <div className="space-y-6">
      <InsightKpis insights={projectInsights(projects, t)} defaults={DEFAULT_PROJECT_INSIGHTS} storageKey="projects_kpis"
        filters={filters} onFiltersChange={(f) => onApplyFilter(f)} />
      <Breakdowns blocks={blocks} total={projects.length} onApply={onApplyFilter} />
    </div>
  );
}
