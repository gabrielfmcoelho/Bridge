"use client";

import { useLocale } from "@/contexts/LocaleContext";
import InsightKpis from "@/components/inventory/InsightKpis";
import Breakdowns, { type BreakdownBlock } from "@/components/inventory/Breakdowns";
import { useHostNames } from "@/hooks/useHostNames";
import { certState } from "@/lib/dnsCert";
import type { DNSRecord } from "@/lib/types";
import type { DNSFilters } from "../FilterDrawer";
import { dnsInsights, dnsBreakdowns, DEFAULT_DNS_INSIGHTS } from "./dnsInsights";

/** The DNS "Dashboard" tab: the KPI tiles, then one bar list per breakdown —
 *  situação, certificate health, issuers, hosts, entidade, tags. */
export default function DnsDashboard({ records, filters, onApplyFilter }: {
  records: DNSRecord[];
  filters: DNSFilters;
  onApplyFilter: (f: Partial<DNSFilters>) => void;
}) {
  const { t } = useLocale();
  const hostNames = useHostNames();
  const b = dnsBreakdowns(records, certState, (id) => hostNames.get(id));
  const blocks: BreakdownBlock<DNSFilters>[] = [
    { title: t("inventory.dash.bySituacao"), rows: b.situacao },
    { title: t("dns.dash.certHealth"), rows: b.cert },
    { title: t("dns.dash.byIssuer"), rows: b.issuer },
    { title: t("dns.dash.byHost"), rows: b.hosts },
    { title: t("inventory.dash.byEntidade"), rows: b.entidade },
    { title: t("inventory.dash.byTags"), rows: b.tags },
  ];
  return (
    <div className="space-y-6">
      <InsightKpis insights={dnsInsights(records, t, certState)} defaults={DEFAULT_DNS_INSIGHTS} storageKey="dns_kpis"
        filters={filters} onFiltersChange={(f) => onApplyFilter(f)} />
      <Breakdowns blocks={blocks} total={records.length} onApply={onApplyFilter} />
    </div>
  );
}
