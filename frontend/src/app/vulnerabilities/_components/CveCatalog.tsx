"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import Pagination from "@/components/ui/Pagination";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import CveAnalysis from "@/components/vulns/CveAnalysis";
import { useLocale } from "@/contexts/LocaleContext";
import { vulnAPI } from "@/lib/api";

const PER_PAGE = 25;

/** The CVE catalog tab: every analysed CVE the caller can see. Read-only —
 *  (re)analysis runs from a finding. */
export default function CveCatalog({ search }: { search: string }) {
  const { t, formatDate } = useLocale();
  const [paged, setPaged] = useState({ key: search, page: 1 });
  const page = paged.key === search ? paged.page : 1;
  const [open, setOpen] = useState<string | null>(null);

  const params: Record<string, string> = { page: String(page), per_page: String(PER_PAGE) };
  if (search) params.search = search;
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "cves", params],
    queryFn: () => vulnAPI.cves(params),
  });
  const rows = data?.data ?? [];
  const total = data?.meta.total ?? 0;

  return (
    <>
      {isLoading ? (
        <SkeletonTable rows={8} />
      ) : isError ? (
        <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>
      ) : rows.length === 0 ? (
        <EmptyState icon="search" title={t("vuln.empty.catalog")} description={t("vuln.cveAnalysis.catalogHint")} />
      ) : (
        <div className="space-y-3">
          <div className={tableClasses.wrapper}>
            <table className={tableClasses.table}>
              <thead>
                <tr className={tableClasses.headRow}>
                  <th className={tableClasses.th}>{t("vuln.cve")}</th>
                  <th className={tableClasses.th}>{t("vuln.cvss")}</th>
                  <th className={tableClasses.th}>{t("vuln.cveAnalysis.description")}</th>
                  <th className={tableClasses.th}>{t("vuln.cveAnalysis.openFindings")}</th>
                  <th className={tableClasses.th}>{t("vuln.cveAnalysis.analyzedAt")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((e, i) => (
                  <tr
                    key={e.cve_id}
                    className={`${tableClasses.row} cursor-pointer ${i % 2 === 1 ? tableClasses.rowAlt : ""}`}
                    onClick={() => setOpen(e.cve_id)}
                  >
                    <td className={`${tableClasses.td} font-mono text-xs whitespace-nowrap`}>{e.cve_id}</td>
                    <td className={tableClasses.td}>{e.cvss ?? "—"}</td>
                    <td className={`${tableClasses.td} max-w-0 w-[55%]`}>
                      <span className="block truncate text-[var(--text-secondary)]" title={e.description}>{e.description}</span>
                    </td>
                    <td className={tableClasses.td}>{e.open_findings}</td>
                    <td className={`${tableClasses.td} text-[var(--text-muted)] whitespace-nowrap`}>{formatDate(e.analyzed_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination
            page={page}
            totalPages={Math.max(1, Math.ceil(total / PER_PAGE))}
            total={total}
            perPage={PER_PAGE}
            onChange={(p) => setPaged({ key: search, page: p })}
          />
        </div>
      )}
      <Drawer open={open != null} onClose={() => setOpen(null)} wide title={open ?? ""}>
        {open && <CveAnalysis cve={open} showRecord />}
      </Drawer>
    </>
  );
}
