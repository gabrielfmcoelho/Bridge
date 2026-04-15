import StatCard from "@/components/ui/StatCard";
import type { Project } from "@/lib/types";

export default function KpiSection({ projects, t }: { projects: Project[]; t: (key: string) => string }) {
  const kpis = [
    {
      label: t("project.title") || "Total Projects",
      value: projects.length,
      color: "amber",
      icon: "M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z",
    },
    {
      label: t("common.active") || "Active",
      value: projects.filter((p) => p.situacao === "active").length,
      color: "emerald",
      icon: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z",
    },
    {
      label: t("project.temEmpresaExterna") || "With External Company",
      value: projects.filter((p) => p.tem_empresa_externa_responsavel).length,
      color: "purple",
      icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
    },
    {
      label: t("issue.title") || "With Issues",
      value: 0,
      color: "cyan",
      icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    },
  ];

  return (
    <div className="mb-5">
      <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">
        {t("common.indicators") || "Indicators"}
      </h2>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {kpis.map((kpi) => (
          <StatCard key={kpi.label} label={kpi.label} value={kpi.value} color={kpi.color} icon={kpi.icon} />
        ))}
      </div>
    </div>
  );
}
