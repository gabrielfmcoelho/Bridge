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

/** The vulnerabilities tab of a host / DNS / service / project: its own
 *  findings plus those on the hosts and DNS records it relates to. */
export default function EntityVulnsTab({ entityType, entityId }: { entityType: VulnEntityType; entityId: number }) {
  const { t } = useLocale();
  const [view, setView] = useState<"open" | "closed">("open");
  const [page, setPage] = useState(1);
  const [openId, setOpenId] = useState<number | null>(null);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "entity", entityType, entityId, view, page],
    queryFn: () =>
      vulnAPI.list({
        entity_type: entityType,
        entity_id: String(entityId),
        status: view === "open" ? "open" : "accepted,false_positive,resolved",
        page: String(page),
        per_page: String(PER_PAGE),
      }),
  });
  const rows = data?.data ?? [];
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
