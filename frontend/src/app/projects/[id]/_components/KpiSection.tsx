import StatCard from "@/components/ui/StatCard";

interface DetailKpiSectionProps {
  servicesCount: number;
  hostsCount: number;
  dnsCount: number;
  issuesCount: number;
  t: (key: string) => string;
}

export default function DetailKpiSection({ servicesCount, hostsCount, dnsCount, issuesCount, t }: DetailKpiSectionProps) {
  const kpis = [
    {
      label: t("service.title"),
      value: servicesCount,
      color: "purple",
      icon: "M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z",
    },
    {
      label: "Hosts",
      value: hostsCount,
      color: "cyan",
      icon: "M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zm0 10h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4a2 2 0 012-2z",
    },
    {
      label: "DNS",
      value: dnsCount,
      color: "emerald",
      icon: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z",
    },
    {
      label: t("issue.title"),
      value: issuesCount,
      color: "amber",
      icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    },
  ];

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
      {kpis.map((kpi) => (
        <StatCard key={kpi.label} label={kpi.label} value={kpi.value} color={kpi.color} icon={kpi.icon} />
      ))}
    </div>
  );
}
