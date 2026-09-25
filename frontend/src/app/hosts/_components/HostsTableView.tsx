import SortableTable from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import Tag from "@/components/ui/Tag";
import ScanIndicator from "./ScanIndicator";
import SituacaoCell from "./SituacaoCell";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { QuickLookOutlet } from "./HostQuickLook";
import { openQuickLook } from "./quickLookStore";
import type { Host, HostSortConfig } from "@/lib/types";

// Server-driven table (inventory pagination reference). `hosts` is ONE server
// page (already filtered+sorted+limited by the parent's paginated query),
// `total` is the full match count for the pager, and column-header clicks drive
// the page-level sort via onSortChange (→ server refetch). scan/tags aren't
// server-sortable, so those headers are non-sortable.
const PER_PAGE = 20;

export default function HostsTableView({
  hosts,
  total,
  tablePage,
  onPageChange,
  sort,
  onSortChange,
  canEdit,
  t,
}: {
  hosts: Host[];
  /** Omitted when grouped: the rows are the whole group, so no pager. */
  total?: number;
  tablePage: number;
  onPageChange: (page: number) => void;
  sort: HostSortConfig;
  onSortChange: (s: HostSortConfig) => void;
  canEdit: boolean;
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
          { key: "scan" as const, label: t("host.scan"), align: "center", sortable: false },
          { key: "tags" as const, label: t("common.tags"), sortable: false },
          { key: "actions" as const, label: t("common.actions"), align: "right", sortable: false },
        ]}
        sortKey={sort.field as "nickname" | "hostname" | "hospedagem" | "situacao" | "scan" | "tags" | "actions"}
        sortDir={sort.direction}
        onSortChange={(key, dir) => {
          if (key === "scan" || key === "tags" || key === "actions") return; // not server-sortable
          onSortChange({ field: key, direction: dir });
        }}
      >
        {() => {
          return hosts.map((host, i) => (
            <tr
              key={host.id}
              className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              onClick={() => window.location.href = `/hosts/${host.oficial_slug}`}
            >
              <td className="px-4 py-2.5 font-medium text-[var(--text-primary)] font-mono">{host.nickname}</td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)] font-mono">{host.hostname || "-"}</td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)]">{host.hospedagem || "-"}</td>
              <td className="px-4 py-2.5" onClick={(e) => { if (canEdit) e.stopPropagation(); }}>
                <SituacaoCell host={host} canEdit={canEdit} t={t} />
              </td>
              <td className="px-4 py-2.5 text-center"><ScanIndicator hasScan={host.has_scan} lastScanAt={host.last_scan_at} /></td>
              <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                  {host.tags?.slice(0, 3).map((tag) => <Tag key={tag}>{tag}</Tag>)}
                  {host.tags && host.tags.length > 3 && <span className="text-2xs text-[var(--text-faint)]">+{host.tags.length - 3}</span>}
                </div>
              </td>
              <td className="px-4 py-1.5 text-right">
                <IconButton
                  label={`${t("host.quickLook")}: ${host.nickname}`}
                  className="ml-auto"
                  onClick={(e) => {
                    e.stopPropagation();
                    openQuickLook(host);
                  }}
                >
                  <Icon path={ICON_PATHS.eye} />
                </IconButton>
              </td>
            </tr>
          ));
        }}
      </SortableTable>
      {total != null && (
        <Pagination page={tablePage} totalPages={Math.max(1, Math.ceil(total / PER_PAGE))} total={total} perPage={PER_PAGE} onChange={onPageChange} />
      )}
      <QuickLookOutlet />
    </div>
  );
}
