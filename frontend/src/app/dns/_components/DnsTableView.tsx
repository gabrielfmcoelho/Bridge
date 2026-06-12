"use client";

import { useRouter } from "next/navigation";
import Badge from "@/components/ui/Badge";
import SortableTable from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import type { DNSRecord } from "@/lib/types";

// Server-driven table (inventory pagination). `records` is ONE server page
// (filtered+sorted+limited by the parent's paginated query), `total` is the full
// match count for the pager, and column-header clicks drive the page-level sort
// via onSortChange (→ server refetch). https/tags aren't server-sortable.
const PER_PAGE = 20;
type Sort = { field: string; direction: "asc" | "desc" };
type ColKey = "domain" | "https" | "situacao" | "responsavel" | "tags";

interface DnsTableViewProps {
  records: DNSRecord[];
  total: number;
  tablePage: number;
  onPageChange: (page: number) => void;
  sort: Sort;
  onSortChange: (s: Sort) => void;
  t: (key: string) => string;
}

export default function DnsTableView({ records, total, tablePage, onPageChange, sort, onSortChange, t }: DnsTableViewProps) {
  const router = useRouter();
  return (
    <div className="space-y-3 animate-fade-in">
      <SortableTable
        columns={[
          { key: "domain" as ColKey, label: t("dns.domain") },
          { key: "https" as ColKey, label: "HTTPS", sortable: false },
          { key: "situacao" as ColKey, label: t("common.status") },
          { key: "responsavel" as ColKey, label: t("dns.responsavel") },
          { key: "tags" as ColKey, label: t("common.tags"), sortable: false },
        ]}
        sortKey={sort.field as ColKey}
        sortDir={sort.direction}
        onSortChange={(key, dir) => {
          if (key === "https" || key === "tags") return; // not server-sortable
          onSortChange({ field: key, direction: dir });
        }}
      >
        {() =>
          records.map((dns, i) => (
            <tr
              key={dns.id}
              className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              onClick={() => router.push(`/dns/${dns.id}`)}
            >
              <td className="px-4 py-2.5 text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{dns.domain}</td>
              <td className="px-4 py-2.5">
                {dns.has_https ? (
                  <Badge color="emerald">HTTPS</Badge>
                ) : (
                  <span className="text-[var(--text-faint)]">-</span>
                )}
              </td>
              <td className="px-4 py-2.5">
                <Badge variant="situacao" situacao={dns.situacao} dot>{dns.situacao}</Badge>
              </td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)]">{dns.main_responsavel_name || dns.responsavel || "-"}</td>
              <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                  {(dns.tags || []).slice(0, 3).map((tag) => <Badge key={tag}>{tag}</Badge>)}
                  {(dns.tags?.length || 0) > 3 && <Badge>+{(dns.tags?.length || 0) - 3}</Badge>}
                </div>
              </td>
            </tr>
          ))
        }
      </SortableTable>
      <Pagination page={tablePage} totalPages={Math.max(1, Math.ceil(total / PER_PAGE))} total={total} perPage={PER_PAGE} onChange={onPageChange} />
    </div>
  );
}
