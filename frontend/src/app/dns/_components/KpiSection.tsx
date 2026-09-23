import KpiGrid from "@/components/inventory/KpiGrid";
import { ICON_PATHS } from "@/lib/icon-paths";
import { certState } from "@/lib/dnsCert";
import type { DNSRecord } from "@/lib/types";

const TO_RENEW = new Set(["expired", "critical", "warning"]);

export default function KpiSection({ records, t }: { records: DNSRecord[]; t: (key: string) => string }) {
  const kpis = [
    { label: t("dns.totalDns"), value: records.length, color: "emerald", icon: "M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" },
    { label: t("dns.hasHttps"), value: records.filter((d) => d.has_https).length, color: "cyan", icon: "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" },
    { label: t("common.active"), value: records.filter((d) => d.situacao === "active").length, color: "accent", icon: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" },
    { label: t("dns.linkedToHosts"), value: records.filter((d) => d.host_ids && d.host_ids.length > 0).length, color: "amber", icon: "M5 12h14M12 5l7 7-7 7" },
    { label: t("dns.certToRenew"), value: records.filter((d) => TO_RENEW.has(certState(d))).length, color: "warning", icon: ICON_PATHS.clock },
  ];

  return <KpiGrid kpis={kpis} heading={t("common.indicators")} columns={5} />;
}
