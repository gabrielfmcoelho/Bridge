"use client";

import { useRouter } from "next/navigation";
import SortableTable from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import Badge from "@/components/ui/Badge";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import type { Database } from "@/lib/types";
import { linkedNames, useApiLinkNames } from "@/app/atlas/apis/_components/apiDisplay";
import { ENVIRONMENT_COLOR, SENSITIVITY_COLOR, endpointLabel, isExternal } from "./databaseInsights";

// Client-side table over the unpaged list, one 20-row slice at a time.
export const DATABASE_TABLE_PER_PAGE = 20;
type Sort = { field: string; direction: "asc" | "desc" };
type ColKey = "name" | "engine" | "where" | "environment" | "sensitivity" | "consumers" | "updated_at";
const SORTABLE = new Set<ColKey>(["name", "engine", "updated_at"]);

export default function DatabasesTableView({ dbs, total, page, onPageChange, sort, onSortChange }: {
  dbs: Database[];
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
  const muted = <span className="text-[var(--text-muted)]">–</span>;
  return (
    <div className="animate-fade-in">
      <SortableTable
        columns={[
          { key: "name" as ColKey, label: t("atlas.databases.name") },
          { key: "engine" as ColKey, label: t("atlas.databases.engineLabel") },
          { key: "where" as ColKey, label: t("atlas.databases.where"), sortable: false },
          { key: "environment" as ColKey, label: t("atlas.databases.environmentLabel"), sortable: false },
          { key: "sensitivity" as ColKey, label: t("atlas.databases.sensitivityLabel"), sortable: false },
          { key: "consumers" as ColKey, label: t("atlas.databases.consumers"), sortable: false },
          { key: "updated_at" as ColKey, label: t("atlas.apis.updated") },
        ]}
        sortKey={sort.field as ColKey}
        sortDir={sort.direction}
        onSortChange={(key, dir) => { if (SORTABLE.has(key)) onSortChange({ field: key, direction: dir }); }}
      >
        {() =>
          dbs.map((d, i) => (
            <tr
              key={d.id}
              className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              onClick={() => router.push(`/atlas/databases/${d.id}`)}
            >
              <td className="px-4 py-2.5">
                <span className="font-medium text-[var(--text-primary)]">{d.name}</span>
                {d.db_name && <span className="ml-2 font-mono text-xs text-[var(--text-muted)]">{d.db_name}</span>}
              </td>
              <td className="px-4 py-2.5"><Badge color="cyan">{t(`atlas.databases.engine.${d.engine}`)}</Badge></td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)] max-w-[220px] truncate">
                {isExternal(d) ? (d.external_provider || t("atlas.databases.external")) : (d.instance?.nickname ?? muted)}
                {endpointLabel(d) && <span className="block font-mono text-xs text-[var(--text-muted)]">{endpointLabel(d)}</span>}
              </td>
              <td className="px-4 py-2.5"><Badge color={ENVIRONMENT_COLOR[d.environment]}>{t(`atlas.databases.environment.${d.environment}`)}</Badge></td>
              <td className="px-4 py-2.5"><Badge color={SENSITIVITY_COLOR[d.sensitivity]}>{t(`atlas.databases.sensitivity.${d.sensitivity}`)}</Badge></td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)] max-w-[220px] truncate">{linkedNames(d.consumer_service_ids, names.service) || muted}</td>
              <td className="px-4 py-2.5 text-xs text-[var(--text-muted)] whitespace-nowrap" title={d.updated_at}>{getTimeAgo(d.updated_at, locale)}</td>
            </tr>
          ))
        }
      </SortableTable>
      {total != null && (
        <Pagination page={page} totalPages={Math.max(1, Math.ceil(total / DATABASE_TABLE_PER_PAGE))} total={total} perPage={DATABASE_TABLE_PER_PAGE} onChange={onPageChange} />
      )}
    </div>
  );
}
