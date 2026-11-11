"use client";

import { Fragment, useMemo, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import { osUpdateAPI, vulnAPI } from "@/lib/api";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Host } from "@/lib/types";
import { hostIdsByOS, osFamilies, osKey, sortOSGroups } from "./osView";
import { osLabel } from "./hostInsights";
import OSUpdateModal from "./OSUpdateModal";
import OSUpdateRuns, { pendingText } from "./OSUpdateRuns";

const params = { kind: "os", scope: "host", per_page: "200" };

/** The hosts page's "Sistemas operacionais" tab: every distro version the
 *  scans found on the hosts themselves (containers excluded), with how many
 *  hosts run it and its end of life. A row expands to its hosts and their
 *  pending updates (last run). Admins start OS update runs from here. */
export default function HostsOSView({ hosts, isAdmin }: { hosts: Host[]; isAdmin: boolean }) {
  const { t } = useLocale();
  const [expanded, setExpanded] = useState<string | null>(null);
  const [showUpdate, setShowUpdate] = useState(false);
  const [openRun, setOpenRun] = useState<number | null>(null);
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "dependency-groups", params],
    queryFn: () => vulnAPI.dependencyGroups(params),
  });
  const { data: latest = [] } = useQuery({ queryKey: ["os-updates", "latest"], queryFn: osUpdateAPI.latest, enabled: isAdmin });
  const groups = sortOSGroups(data?.data ?? []);
  const families = osFamilies(groups);
  const idsByOS = useMemo(() => hostIdsByOS(hosts), [hosts]);
  const hostById = useMemo(() => new Map(hosts.map((h) => [h.id, h])), [hosts]);
  const latestByHost = useMemo(() => new Map(latest.map((x) => [x.host_id, x])), [latest]);
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const release = useMutation({
    mutationFn: (h: Host) => osUpdateAPI.start({ kind: "release", host_ids: [h.id], reboot: true, concurrency: 1 }),
    onSuccess: (run) => {
      queryClient.invalidateQueries({ queryKey: ["os-updates"] });
      setOpenRun(run.id);
    },
    onError: (err) => flag({ appearance: "error", title: t("host.osRelease.failed"), description: (err as Error).message }),
  });
  // One host at a time, typed name: a release upgrade can leave a VM unbootable.
  const startRelease = async (h: Host) => {
    const ok = await confirm({
      title: t("host.osRelease.confirmTitle", { host: h.nickname, os: osLabel(h) }),
      message: t("host.osRelease.confirmMessage"),
      confirmLabel: t("host.osRelease.start"),
      danger: true,
      requireText: h.nickname,
    });
    if (ok) release.mutate(h);
  };

  if (isLoading) return <SkeletonTable rows={8} />;
  if (isError) return <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>;
  if (groups.length === 0) return <EmptyState icon="server" title={t("host.osView.empty")} description={t("host.osView.emptyHint")} />;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        {families.map((f) => (
          <span key={f.name} className="inline-flex items-center gap-2 px-3 py-1.5 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-surface)] text-sm">
            <span className="font-medium text-[var(--text-primary)]">{f.name}</span>
            <span className="tabular-nums text-[var(--text-secondary)]">{t("host.osView.hostsCount", { count: String(f.hosts) })}</span>
            {f.eolHosts > 0 && <Badge color="danger">{t("host.osView.eolCount", { count: String(f.eolHosts) })}</Badge>}
          </span>
        ))}
        {isAdmin && (
          <Button size="sm" className="ml-auto" onClick={() => setShowUpdate(true)}>
            <Icon path={ICON_PATHS.refresh} className="w-3.5 h-3.5" />{t("host.osUpdate.open")}
          </Button>
        )}
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
              const gk = osKey(g.name, g.version);
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
                        <div className="flex flex-wrap gap-x-5 gap-y-1">
                          {(idsByOS.get(gk) ?? []).map((id) => {
                            const h = hostById.get(id)!;
                            const last = latestByHost.get(id);
                            return (
                              <span key={id} className="inline-flex items-center gap-1.5 text-xs">
                                <Link href={`/hosts/${h.oficial_slug}`} className="font-mono text-[var(--accent)] hover:underline">{h.nickname}</Link>
                                {last && (
                                  <span className="text-[var(--text-muted)]" title={t("host.osUpdate.pendingTitle")}>
                                    {t("host.osUpdate.pendingShort", { text: pendingText(last, t) })}
                                  </span>
                                )}
                                {h.os?.reboot_required && <span className="text-[var(--warning)]">{t("host.rebootPending")}</span>}
                                {isAdmin && (
                                  <button type="button" onClick={() => startRelease(h)} disabled={release.isPending}
                                    className="text-[var(--text-muted)] hover:text-[var(--danger)] underline-offset-2 hover:underline">
                                    {t("host.osRelease.action")}
                                  </button>
                                )}
                              </span>
                            );
                          })}
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
      {isAdmin && <OSUpdateRuns openId={openRun} onOpen={setOpenRun} />}
      {showUpdate && (
        <OSUpdateModal groups={groups} hostIds={idsByOS} onClose={() => setShowUpdate(false)}
          onStarted={(run) => { setShowUpdate(false); setOpenRun(run.id); }} />
      )}
    </div>
  );
}
