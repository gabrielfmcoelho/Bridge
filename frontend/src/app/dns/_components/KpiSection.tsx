import StatCard from "@/components/ui/StatCard";
import type { DNSRecord } from "@/lib/types";

export default function KpiSection({ records, t }: { records: DNSRecord[]; t: (key: string) => string }) {
  const kpis = [
    {
      label: "Total DNS",
      value: records.length,
      color: "emerald",
      icon: "M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9",
    },
    {
      label: t("dns.hasHttps") || "Com HTTPS",
      value: records.filter((d) => d.has_https).length,
      color: "cyan",
      icon: "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z",
    },
    {
      label: t("common.active") || "Ativos",
      value: records.filter((d) => d.situacao === "active").length,
      color: "purple",
      icon: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z",
    },
    {
      label: t("dns.linkedToHosts") || "Vinculados a Hosts",
      value: records.filter((d) => d.host_ids && d.host_ids.length > 0).length,
      color: "amber",
      icon: "M5 12h14M12 5l7 7-7 7",
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
