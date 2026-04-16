import KpiGrid from "@/components/inventory/KpiGrid";
import type { Host } from "@/lib/types";

export default function KpiSection({ hosts, t }: { hosts: Host[]; t: (key: string) => string }) {
  const kpis = [
    { label: t("dashboard.totalHosts"), value: hosts.length, color: "cyan", icon: "M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zm0 10h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4a2 2 0 012-2z" },
    { label: t("host.withCredentials"), value: hosts.filter(h => h.has_key || h.has_password).length, color: "emerald", icon: "M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" },
    { label: t("host.scanned"), value: hosts.filter(h => h.has_scan).length, color: "purple", icon: "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" },
    { label: t("host.containers"), value: hosts.reduce((sum, h) => sum + (h.containers_count || 0), 0), color: "sky", icon: "M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" },
    { label: t("host.alerts"), value: hosts.filter(h => h.alerts && h.alerts.length > 0).length, color: "amber", icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" },
  ];

  return <KpiGrid kpis={kpis} heading={t("common.indicators") || "Indicators"} columns={5} />;
}
