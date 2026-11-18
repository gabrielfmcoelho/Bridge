"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import TabBar from "@/components/ui/TabBar";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import Pagination from "@/components/ui/Pagination";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { useLocale } from "@/contexts/LocaleContext";
import { vulnAPI } from "@/lib/api";
import { mergeFindings } from "@/lib/vulns";
import type { VulnEntityType } from "@/lib/types";
import VulnTable from "./VulnTable";
import VulnDrawer from "./VulnDrawer";

const PER_PAGE = 25;

/** Open count for an entity's tab badge (same query key prefix as the list,
 *  so a status change refreshes it). */
export function useEntityVulnCount(entityType: VulnEntityType, entityId: number | undefined) {
  const { data } = useQuery({
    queryKey: ["vulnerabilities", "count", entityType, entityId],
    queryFn: () => vulnAPI.list({ entity_type: entityType, entity_id: String(entityId), status: "open", per_page: "1" }),
    enabled: !!entityId,
  });
  return data?.meta.total ?? 0;
}

/** Per source, when several are merged here. */
const MERGE_CAP = 200;

/** The vulnerabilities tab of a host / DNS / service / project: its own
 *  findings plus those on the hosts and DNS records it relates to.
 *  sources narrows it to those finding sources (default: all). */
export default function EntityVulnsTab({ entityType, entityId, sources }: { entityType: VulnEntityType; entityId: number; sources?: readonly string[] }) {
  const { t } = useLocale();
  const [view, setView] = useState<"open" | "closed">("open");
  const [page, setPage] = useState(1);
  const [openId, setOpenId] = useState<number | null>(null);

  // shortcut: the API filters one source at a time, so several are fetched whole (MERGE_CAP
  // each, the rest is cut) and paged here; drop this once `source` takes a comma list (VulnRepo.List).
  const merged = (sources?.length ?? 0) > 1;
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "entity", entityType, entityId, view, merged ? 0 : page, sources ?? null],
    queryFn: async () => {
      const base = {
        entity_type: entityType,
        entity_id: String(entityId),
        status: view === "open" ? "open" : "accepted,false_positive,resolved",
      };
      if (!merged) return vulnAPI.list({ ...base, ...(sources?.[0] ? { source: sources[0] } : {}), page: String(page), per_page: String(PER_PAGE) });
      const parts = await Promise.all(sources!.map((source) => vulnAPI.list({ ...base, source, per_page: String(MERGE_CAP) })));
      const all = mergeFindings(parts.map((p) => p.data));
      return { data: all, meta: { page: 1, per_page: all.length, total: all.length } };
    },
  });
  const rows = merged ? (data?.data ?? []).slice((page - 1) * PER_PAGE, page * PER_PAGE) : data?.data ?? [];
  const total = data?.meta.total ?? 0;

  return (
    <div className="space-y-3">
      <TabBar
        tabs={[
          { key: "open", label: t("vuln.view.open") },
          { key: "closed", label: t("vuln.view.closed") },
        ]}
        activeTab={view}
        onChange={(k) => { setView(k as "open" | "closed"); setPage(1); }}
      />
      {isLoading ? (
        <SkeletonTable rows={4} />
      ) : isError ? (
        <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>
      ) : rows.length === 0 ? (
        <EmptyState compact icon="search" title={t(view === "open" ? "vuln.emptyEntityOpen" : "vuln.emptyEntityClosed")} description={t("vuln.emptyEntityHint")} />
      ) : (
        <>
          <VulnTable rows={rows} onOpen={setOpenId} showVia={entityType !== "dns"} />
          {total > PER_PAGE && (
            <Pagination page={page} totalPages={Math.ceil(total / PER_PAGE)} total={total} perPage={PER_PAGE} onChange={setPage} />
          )}
        </>
      )}
      <VulnDrawer id={openId} onClose={() => setOpenId(null)} />
    </div>
  );
}
