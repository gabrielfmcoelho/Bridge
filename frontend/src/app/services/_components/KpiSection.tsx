import KpiGrid from "@/components/inventory/KpiGrid";
import type { Service } from "@/lib/types";

export default function KpiSection({ services, t }: { services: Service[]; t: (key: string) => string }) {
  const kpis = [
    { label: t("service.title"), value: services.length, color: "purple", icon: "M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" },
    { label: t("service.isExternalDependency"), value: services.filter(s => s.is_external_dependency).length, color: "amber", icon: "M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" },
    { label: t("service.orchestratorManaged"), value: services.filter(s => s.orchestrator_managed).length, color: "cyan", icon: "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" },
    { label: t("service.internal"), value: services.filter(s => !s.is_external_dependency && s.developed_by === "internal").length, color: "emerald", icon: "M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" },
  ];

  return <KpiGrid kpis={kpis} heading={t("common.indicators") || "Indicators"} columns={4} />;
}
