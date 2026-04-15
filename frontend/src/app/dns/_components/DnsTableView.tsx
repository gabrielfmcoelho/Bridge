"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Badge from "@/components/ui/Badge";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import type { DNSRecord } from "@/lib/types";

const ROWS_PER_PAGE = 20;

interface DnsTableViewProps {
  records: DNSRecord[];
  t: (key: string) => string;
}

export default function DnsTableView({ records, t }: DnsTableViewProps) {
  const router = useRouter();
  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil(records.length / ROWS_PER_PAGE));

  return (
    <div className="space-y-3 animate-fade-in">
      <SortableTable
        columns={[
          { key: "domain" as const, label: t("dns.domain") },
          { key: "https" as const, label: "HTTPS" },
          { key: "situacao" as const, label: t("common.status") },
          { key: "responsavel" as const, label: t("dns.responsavel") },
          { key: "tags" as const, label: t("common.tags") },
        ]}
        defaultSort="domain"
      >
        {(sk, sd) => {
          const sorted = sortRows(records, sk, sd, {
            domain: (a, b) => a.domain.localeCompare(b.domain),
            https: (a, b) => Number(b.has_https) - Number(a.has_https),
            situacao: (a, b) => a.situacao.localeCompare(b.situacao),
            responsavel: (a, b) => (a.main_responsavel_name || a.responsavel || "").localeCompare(b.main_responsavel_name || b.responsavel || ""),
            tags: (a, b) => (a.tags?.length || 0) - (b.tags?.length || 0),
          });
          const paged = sorted.slice((page - 1) * ROWS_PER_PAGE, page * ROWS_PER_PAGE);
          return paged.map((dns, i) => (
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
          ));
        }}
      </SortableTable>

      {totalPages > 1 && (
        <Pagination page={page} totalPages={totalPages} total={records.length} perPage={ROWS_PER_PAGE} onChange={setPage} />
      )}
    </div>
  );
}
