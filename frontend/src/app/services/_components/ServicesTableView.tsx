"use client";

import SortableTable from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import Badge from "@/components/ui/Badge";
import type { Service } from "@/lib/types";

// Server-driven table (inventory pagination). `services` is ONE server page
// (filtered+sorted+limited by the parent's paginated query), `total` is the full
// match count for the pager, and column-header clicks drive the page-level sort
// via onSortChange (→ server refetch). Only nickname/technology_stack are
// server-sortable.
const PER_PAGE = 20;
type Sort = { field: string; direction: "asc" | "desc" };
type ColKey = "nickname" | "description" | "source" | "technology_stack" | "developed_by" | "tags";

interface ServicesTableViewProps {
  services: Service[];
  total: number;
  tablePage: number;
  onPageChange: (page: number) => void;
  sort: Sort;
  onSortChange: (s: Sort) => void;
  t: (key: string) => string;
}

export default function ServicesTableView({ services, total, tablePage, onPageChange, sort, onSortChange, t }: ServicesTableViewProps) {
  return (
    <div className="animate-fade-in">
      <SortableTable
        columns={[
          { key: "nickname" as ColKey, label: t("service.nickname") },
          { key: "description" as ColKey, label: t("common.description"), sortable: false },
          { key: "source" as ColKey, label: t("service.source"), sortable: false },
          { key: "technology_stack" as ColKey, label: t("service.technologyStack") },
          { key: "developed_by" as ColKey, label: t("service.developedBy"), sortable: false },
          { key: "tags" as ColKey, label: t("common.tags"), sortable: false },
        ]}
        sortKey={sort.field as ColKey}
        sortDir={sort.direction}
        onSortChange={(key, dir) => {
          if (key !== "nickname" && key !== "technology_stack") return; // not server-sortable
          onSortChange({ field: key, direction: dir });
        }}
      >
        {() =>
          services.map((svc, i) => (
            <tr
              key={svc.id}
              className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              onClick={() => window.location.href = `/services/${svc.id}`}
            >
              <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]">
                <div className="flex items-center gap-1.5">
                  {svc.container_status && (
                    <span className={`inline-block w-2 h-2 rounded-full shrink-0 ${svc.container_status === "online" ? "bg-emerald-400" : "bg-gray-400"}`} />
                  )}
                  {svc.nickname}
                </div>
              </td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)] max-w-[200px] truncate">{svc.description || "-"}</td>
              <td className="px-4 py-2.5">
                <Badge color={svc.source === "auto" ? "blue" : svc.source === "fixed" ? "emerald" : "default"}>
                  {svc.source === "auto" ? t("service.sourceAuto") : svc.source === "fixed" ? t("service.sourceFixed") : t("service.sourceManual")}
                </Badge>
              </td>
              <td className="px-4 py-2.5">{svc.technology_stack ? <Badge>{svc.technology_stack}</Badge> : <span className="text-[var(--text-faint)]">-</span>}</td>
              <td className="px-4 py-2.5">
                {svc.is_external_dependency ? (
                  <Badge color="amber">{t("service.isExternalDependency")}</Badge>
                ) : (
                  <Badge color={svc.developed_by === "internal" ? "cyan" : "amber"}>
                    {svc.developed_by === "internal" ? t("service.internal") : t("service.external")}
                  </Badge>
                )}
              </td>
              <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                  {svc.tags && svc.tags.length > 0
                    ? svc.tags.slice(0, 3).map((tag) => <Badge key={tag}>{tag}</Badge>)
                    : <span className="text-[var(--text-faint)]">-</span>}
                  {svc.tags && svc.tags.length > 3 && <span className="text-[10px] text-[var(--text-faint)]">+{svc.tags.length - 3}</span>}
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
