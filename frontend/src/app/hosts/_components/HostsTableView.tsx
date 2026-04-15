import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import Badge from "@/components/ui/Badge";
import ScanIndicator from "./ScanIndicator";
import type { Host } from "@/lib/types";

export default function HostsTableView({
  hosts,
  tablePage,
  onPageChange,
  t,
}: {
  hosts: Host[];
  tablePage: number;
  onPageChange: (page: number) => void;
  t: (key: string) => string;
}) {
  return (
    <div className="animate-fade-in">
      <SortableTable
        columns={[
          { key: "nickname" as const, label: t("host.nickname") },
          { key: "hostname" as const, label: t("host.hostname") },
          { key: "hospedagem" as const, label: t("host.hospedagem") },
          { key: "situacao" as const, label: t("host.situacao") },
          { key: "scan" as const, label: t("host.scan"), align: "center" },
          { key: "tags" as const, label: t("common.tags") },
        ]}
        defaultSort="nickname"
      >
        {(sk, sd) => {
          const sorted = sortRows(hosts, sk, sd, {
            nickname: (a, b) => a.nickname.localeCompare(b.nickname),
            hostname: (a, b) => (a.hostname || "").localeCompare(b.hostname || ""),
            hospedagem: (a, b) => (a.hospedagem || "").localeCompare(b.hospedagem || ""),
            situacao: (a, b) => a.situacao.localeCompare(b.situacao),
            scan: (a, b) => (a.has_scan ? 1 : 0) - (b.has_scan ? 1 : 0),
            tags: (a, b) => (a.tags?.length || 0) - (b.tags?.length || 0),
          });
          const perPage = 20;
          const paged = sorted.slice((tablePage - 1) * perPage, tablePage * perPage);
          return paged.map((host, i) => (
            <tr
              key={host.id}
              className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              onClick={() => window.location.href = `/hosts/${host.oficial_slug}`}
            >
              <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{host.nickname}</td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{host.hostname || "-"}</td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)]">{host.hospedagem || "-"}</td>
              <td className="px-4 py-2.5"><Badge variant="situacao" situacao={host.situacao} dot>{host.situacao}</Badge></td>
              <td className="px-4 py-2.5 text-center"><ScanIndicator hasScan={host.has_scan} lastScanAt={host.last_scan_at} /></td>
              <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                  {host.tags?.slice(0, 3).map((tag) => <Badge key={tag}>{tag}</Badge>)}
                  {host.tags && host.tags.length > 3 && <span className="text-[10px] text-[var(--text-faint)]">+{host.tags.length - 3}</span>}
                </div>
              </td>
            </tr>
          ));
        }}
      </SortableTable>
      <Pagination page={tablePage} totalPages={Math.ceil(hosts.length / 20)} total={hosts.length} perPage={20} onChange={onPageChange} />
    </div>
  );
}
