"use client";

import { useRouter } from "next/navigation";
import SortableTable, { type SortableColumn } from "@/components/ui/SortableTable";
import Badge from "@/components/ui/Badge";
import { assetTypeLabelKey } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { CatalogHit } from "@/lib/types";

export type AssetSortKey = "name" | "asset_type" | "detail" | "description" | "entidade_name";

// The "already exists" group, as one polymorphic table. Six asset types share
// these columns because the question being asked is the same for all of them —
// "does my thing already exist?" — and `detail` carries whatever single fact
// distinguishes each type (a host's FQDN, a service's stack and port, an API's
// spec version), so one row shape stays honest across all of them.
//
// Sort is server-driven (controlled mode), matching HostsTableView: the header
// drives a refetch rather than reordering the current page, so sorting means
// the same thing on page 1 and page 3.
export default function CatalogAssetsTable({
  hits,
  sortKey,
  sortDir,
  onSortChange,
}: {
  hits: CatalogHit[];
  sortKey: AssetSortKey;
  sortDir: "asc" | "desc";
  onSortChange: (key: AssetSortKey, dir: "asc" | "desc") => void;
}) {
  const { t } = useLocale();
  const router = useRouter();

  const columns: SortableColumn<AssetSortKey>[] = [
    { key: "name", label: t("common.name") },
    { key: "asset_type", label: t("common.type") },
    { key: "detail", label: t("catalog.detail") },
    // Prose does not sort into anything a user would want.
    { key: "description", label: t("common.description"), sortable: false },
    { key: "entidade_name", label: t("requests.entidade") },
  ];

  return (
    <div className="animate-fade-in">
      <SortableTable
        columns={columns}
        sortKey={sortKey}
        sortDir={sortDir}
        onSortChange={(key, dir) => {
          if (key === "description") return;
          onSortChange(key, dir);
        }}
      >
        {() =>
          hits.map((hit, i) => {
            const typeLabelKey = assetTypeLabelKey(hit.asset_type);
            return (
              <tr
                key={`${hit.asset_type}-${hit.id}`}
                onClick={() => router.push(hit.href)}
                className={`cursor-pointer border-t border-[var(--border-subtle)] transition-colors hover:bg-[var(--bg-elevated)] ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              >
                <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>
                  {hit.name}
                </td>
                <td className="px-4 py-2.5">
                  <Badge color="gray">{typeLabelKey ? t(typeLabelKey) : hit.asset_type}</Badge>
                </td>
                <td className="px-4 py-2.5 text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>
                  {hit.detail || "-"}
                </td>
                <td className="max-w-[28rem] truncate px-4 py-2.5 text-[var(--text-muted)]">{hit.description || "-"}</td>
                <td className="px-4 py-2.5 text-[var(--text-secondary)]">{hit.entidade_name || "-"}</td>
              </tr>
            );
          })
        }
      </SortableTable>
    </div>
  );
}
