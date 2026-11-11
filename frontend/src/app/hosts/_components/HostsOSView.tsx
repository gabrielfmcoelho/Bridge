"use client";

import { Fragment, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import { vulnAPI } from "@/lib/api";
import { ICON_PATHS } from "@/lib/icon-paths";
import { osFamilies, sortOSGroups } from "./osView";

const params = { kind: "os", scope: "host", per_page: "200" };

/** The hosts page's "Sistemas operacionais" tab: every distro version the
 *  scans found on the hosts themselves (containers excluded), with how many
 *  hosts run it and its end of life. A row expands to its hosts. */
export default function HostsOSView() {
  const { t } = useLocale();
  const [expanded, setExpanded] = useState<string | null>(null);
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "dependency-groups", params],
    queryFn: () => vulnAPI.dependencyGroups(params),
  });
  const groups = sortOSGroups(data?.data ?? []);
  const families = osFamilies(groups);

  if (isLoading) return <SkeletonTable rows={8} />;
  if (isError) return <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>;
  if (groups.length === 0) return <EmptyState icon="server" title={t("host.osView.empty")} description={t("host.osView.emptyHint")} />;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        {families.map((f) => (
          <span key={f.name} className="inline-flex items-center gap-2 px-3 py-1.5 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-surface)] text-sm">
            <span className="font-medium text-[var(--text-primary)]">{f.name}</span>
            <span className="tabular-nums text-[var(--text-secondary)]">{t("host.osView.hostsCount", { count: String(f.hosts) })}</span>
            {f.eolHosts > 0 && <Badge color="danger">{t("host.osView.eolCount", { count: String(f.eolHosts) })}</Badge>}
          </span>
        ))}
      </div>
      <div className={tableClasses.wrapper}>
        <table className={tableClasses.table}>
          <thead>
            <tr className={tableClasses.headRow}>
              <th className={tableClasses.th}>{t("host.osView.distro")}</th>
              <th className={tableClasses.th}>{t("host.osView.version")}</th>
              <th className={tableClasses.th}>{t("host.osView.hosts")}</th>
              <th className={tableClasses.th}>{t("vuln.deps.eol")}</th>
            </tr>
          </thead>
          <tbody>
            {groups.map((g, i) => {
              const gk = `${g.name}|${g.version}`;
              const open = expanded === gk;
              return (
                <Fragment key={gk}>
                  <tr className={`${tableClasses.row} cursor-pointer ${i % 2 === 1 ? tableClasses.rowAlt : ""}`}
                    onClick={() => setExpanded(open ? null : gk)} aria-expanded={open}>
                    <td className={tableClasses.td}>
                      <span className="inline-flex items-center gap-2">
                        <Icon path={ICON_PATHS.chevronRight} className={`w-3.5 h-3.5 shrink-0 text-[var(--text-muted)] transition-transform ${open ? "rotate-90" : ""}`} />
                        <span className="font-medium text-[var(--text-primary)]">{g.name}</span>
                      </span>
                    </td>
                    <td className={`${tableClasses.td} font-mono text-xs`}>{g.version}</td>
                    <td className={`${tableClasses.td} tabular-nums`}>{g.hosts}</td>
                    <td className={tableClasses.td}>
                      {g.eol ? <Badge color="danger">{g.eol_date || t("vuln.deps.eolYes")}</Badge>
                        : g.eol_date ? <span className="text-[var(--text-secondary)]">{t("vuln.deps.eolUntil", { date: g.eol_date })}</span>
                        : <span className="text-[var(--text-muted)]">—</span>}
                    </td>
                  </tr>
                  {open && (
                    <tr className={`${tableClasses.row} bg-[var(--bg-elevated)]/40`}>
                      <td className={tableClasses.td} />
                      <td className={tableClasses.td} colSpan={3}>
                        <div className="flex flex-wrap gap-x-4 gap-y-1">
                          {g.instances.map((inst) => (
                            <Link key={inst.id} href={`/hosts/${inst.host_slug}`} className="font-mono text-xs text-[var(--accent)] hover:underline">
                              {inst.host_nickname}
                            </Link>
                          ))}
                          {g.hosts > g.instances.length && (
                            <span className="text-xs text-[var(--text-muted)]">{t("host.osView.more", { count: String(g.hosts - g.instances.length) })}</span>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
