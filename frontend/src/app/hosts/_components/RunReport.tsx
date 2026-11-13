"use client";

import SectionHeading from "@/components/ui/SectionHeading";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import type { ReleaseReport } from "@/lib/types";
import { RiskList } from "./ReleaseWarnings";

type ListKey = "dpkgErrors" | "dpkgBroken" | "newFailedUnits" | "stoppedServices" | "lostPhpHandlers" | "newDistFiles" | "newForeignPackages";

/** What got worse on the host, list by list (dpkg's half-configured
 *  packages only when it gave no error). */
function reportLists(r: ReleaseReport): [ListKey, string[]][] {
  const lists: [ListKey, string[] | undefined][] = [
    ["dpkgErrors", r.dpkg_errors],
    ["dpkgBroken", r.dpkg_errors?.length ? undefined : r.dpkg_broken],
    ["newFailedUnits", r.new_failed_units],
    ["stoppedServices", r.stopped_services],
    ["lostPhpHandlers", r.lost_php_handlers],
    ["newDistFiles", r.new_dist_files],
    ["newForeignPackages", r.new_foreign_packages],
  ];
  return lists.filter((l): l is [ListKey, string[]] => !!l[1]?.length);
}

/** How many things the report says to look at (the drawer's badge). */
export function reportIssues(r?: ReleaseReport): number {
  if (!r) return 0;
  return reportLists(r).reduce((n, [, items]) => n + items.length, 0) + (r.sites ?? []).filter((s) => s.broken).length;
}

/** One host's report of a run: the scan's warnings right before it, what
 *  dpkg failed on, what the scans before and after differ by, and its sites. */
export default function RunReport({ report }: { report: ReleaseReport }) {
  const { t } = useLocale();
  const lists = reportLists(report);
  const sites = report.sites ?? [];
  const status = (s: number) => (s ? String(s) : t("host.osUpdate.report.noAnswer"));
  return (
    <div className="space-y-3 mb-3">
      {lists.map(([key, items]) => (
        <div key={key}>
          <SectionHeading>{t(`host.osUpdate.report.${key}`, { count: String(items.length) })}</SectionHeading>
          <ul className="mt-1 text-xs font-mono text-[var(--danger)] space-y-0.5">
            {items.map((x) => <li key={x} className="break-all">{x}</li>)}
          </ul>
        </div>
      ))}
      {lists.length === 0 && !sites.some((s) => s.broken) && (
        <p className="text-xs text-[var(--success)]">{t("host.osUpdate.report.nothingWorse")}</p>
      )}
      {sites.length > 0 && (
        <div>
          <SectionHeading>{t("host.osUpdate.report.sites", { count: String(sites.length) })}</SectionHeading>
          <div className={`${tableClasses.wrapper} mt-1`}>
            <table className={tableClasses.table}>
              <thead>
                <tr className={tableClasses.headRow}>
                  <th className={tableClasses.th}>{t("host.osUpdate.report.site")}</th>
                  <th className={tableClasses.th}>{t("host.osUpdate.report.before")}</th>
                  <th className={tableClasses.th}>{t("host.osUpdate.report.after")}</th>
                </tr>
              </thead>
              <tbody>
                {sites.map((s) => (
                  <tr key={s.name} className={tableClasses.row}>
                    <td className={`${tableClasses.td} font-mono text-xs`}>{s.name}</td>
                    <td className={`${tableClasses.td} text-xs tabular-nums`}>{status(s.before)}</td>
                    <td className={`${tableClasses.td} text-xs tabular-nums ${s.broken ? "text-[var(--danger)] font-medium" : ""}`}>
                      {status(s.after)}{s.error && ` · ${t("host.osUpdate.report.phpError", { error: s.error })}`}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
      {!!report.warnings_before?.length && (
        <div>
          <SectionHeading>{t("host.osUpdate.report.warningsBefore")}</SectionHeading>
          <div className="mt-1 text-xs text-[var(--text-secondary)]"><RiskList warnings={report.warnings_before} /></div>
        </div>
      )}
    </div>
  );
}
