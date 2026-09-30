"use client";

import { useRouter } from "next/navigation";
import SortableTable from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import Badge from "@/components/ui/Badge";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import type { ApiCatalog } from "@/lib/types";
import { baseHost, linkedNames, useApiLinkNames } from "./apiDisplay";

// Client-side table: the catalog list is small and unpaged server-side, so the
// page sorts the whole filtered list and this view shows one 20-row slice.
export const API_TABLE_PER_PAGE = 20;
type Sort = { field: string; direction: "asc" | "desc" };
type ColKey = "name" | "base" | "spec_version" | "operation_count" | "links" | "updated_at";
const SORTABLE = new Set<ColKey>(["name", "operation_count", "updated_at"]);

export default function ApisTableView({ apis, total, page, onPageChange, sort, onSortChange }: {
  /** The rows to show (already sliced to the page). */
  apis: ApiCatalog[];
  /** Omitted when grouped: the rows are the whole group, so no pager. */
  total?: number;
  page: number;
  onPageChange: (page: number) => void;
  sort: Sort;
  onSortChange: (s: Sort) => void;
}) {
  const { t, locale } = useLocale();
  const router = useRouter();
  const names = useApiLinkNames();
  return (
    <div className="animate-fade-in">
      <SortableTable
        columns={[
          { key: "name" as ColKey, label: t("atlas.apis.name") },
          { key: "base" as ColKey, label: t("atlas.apis.baseUrl"), sortable: false },
          { key: "spec_version" as ColKey, label: t("atlas.apis.specVersion"), sortable: false },
          { key: "operation_count" as ColKey, label: t("atlas.apis.tabEndpoints"), align: "right" },
          { key: "links" as ColKey, label: t("atlas.apis.links"), sortable: false },
          { key: "updated_at" as ColKey, label: t("atlas.apis.updated") },
        ]}
        sortKey={sort.field as ColKey}
        sortDir={sort.direction}
        onSortChange={(key, dir) => { if (SORTABLE.has(key)) onSortChange({ field: key, direction: dir }); }}
      >
        {() =>
          apis.map((a, i) => {
            const links = [linkedNames(a.project_ids, names.project), linkedNames(a.service_ids, names.service)].filter(Boolean).join(" · ");
            return (
              <tr
                key={a.id}
                className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
                onClick={() => router.push(`/atlas/apis/${a.id}`)}
              >
                <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]">{a.name}</td>
                <td className="px-4 py-2.5 font-mono text-xs text-[var(--text-secondary)] max-w-[220px] truncate">{baseHost(a) || <span className="text-[var(--text-muted)]">–</span>}</td>
                <td className="px-4 py-2.5"><Badge color="cyan">{a.spec_version || "–"}</Badge></td>
                <td className="px-4 py-2.5 text-right font-mono text-[var(--text-secondary)]">{a.operation_count}</td>
                <td className="px-4 py-2.5 text-[var(--text-secondary)] max-w-[240px] truncate">{links || <span className="text-[var(--text-muted)]">{t("atlas.apis.avulso")}</span>}</td>
                <td className="px-4 py-2.5 text-xs text-[var(--text-muted)] whitespace-nowrap" title={a.updated_at}>{getTimeAgo(a.updated_at, locale)}</td>
              </tr>
            );
          })
        }
      </SortableTable>
      {total != null && (
        <Pagination page={page} totalPages={Math.max(1, Math.ceil(total / API_TABLE_PER_PAGE))} total={total} perPage={API_TABLE_PER_PAGE} onChange={onPageChange} />
      )}
    </div>
  );
}
