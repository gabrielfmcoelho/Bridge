"use client";

import { Fragment, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import Lozenge, { type LozengeAppearance } from "@/components/ui/Lozenge";
import SectionHeading from "@/components/ui/SectionHeading";
import StatusAlert from "@/components/ui/StatusAlert";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { osUpdateAPI } from "@/lib/api";
import { getTimeAgo } from "@/lib/utils";
import type { OSUpdateResult, OSUpdateRun } from "@/lib/types";
import OSRestoreModal from "./OSRestoreModal";
import BackupActions from "./BackupActions";

const RUN_TONE: Record<OSUpdateRun["status"], LozengeAppearance> = { running: "inprogress", done: "success", cancelled: "default", interrupted: "moved" };
const RESULT_TONE: Record<OSUpdateResult["status"], LozengeAppearance> = { pending: "default", running: "inprogress", success: "success", failed: "removed", skipped: "moved" };

/** Run history (the fleet's, or one host's with hostId); a row opens the
 *  run's per-host results. Polls while a run is going and says when one ends. */
export default function OSUpdateRuns({ openId, onOpen, hostId }: { openId: number | null; onOpen: (id: number | null) => void; hostId?: number }) {
  const { t, locale } = useLocale();
  const [restoring, setRestoring] = useState<OSUpdateResult | null>(null);
  const flag = useFlag();
  const queryClient = useQueryClient();
  const { data: runs = [], isError, error } = useQuery({
    queryKey: ["os-updates", "runs", hostId ?? 0],
    queryFn: () => osUpdateAPI.runs(hostId),
    refetchInterval: (q) => (q.state.data?.some((r) => r.status === "running") ? 5000 : false),
  });

  // A run that was running and isn't any more: tell the admin, refresh the map.
  const wasRunning = useRef(new Set<number>());
  useEffect(() => {
    for (const r of runs) {
      if (r.status === "running") wasRunning.current.add(r.id);
      else if (wasRunning.current.delete(r.id)) {
        flag({ appearance: r.counts.failed ? "warning" : "success", title: t("host.osUpdate.finished", { id: String(r.id) }), description: countsText(r, t) });
        queryClient.invalidateQueries({ queryKey: ["hosts"] });
        queryClient.invalidateQueries({ queryKey: ["vulnerabilities", "dependency-groups"] });
        queryClient.invalidateQueries({ queryKey: ["os-updates", "latest"] });
        queryClient.invalidateQueries({ queryKey: ["os-updates", "backups"] });
      }
    }
  }, [runs, flag, t, queryClient]);

  if (isError) return <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>;
  if (runs.length === 0) return null;
  return (
    <div className="space-y-2">
      <SectionHeading>{t("host.osUpdate.runs")}</SectionHeading>
      <div className={tableClasses.wrapper}>
        <table className={tableClasses.table}>
          <thead>
            <tr className={tableClasses.headRow}>
              <th className={tableClasses.th}>#</th>
              <th className={tableClasses.th}>{t("host.osUpdate.kind")}</th>
              <th className={tableClasses.th}>{t("host.osUpdate.status")}</th>
              <th className={tableClasses.th}>{t("host.osUpdate.hosts")}</th>
              <th className={tableClasses.th}>{t("host.osUpdate.when")}</th>
            </tr>
          </thead>
          <tbody>
            {runs.map((r, i) => (
              <tr key={r.id} className={`${tableClasses.row} cursor-pointer ${i % 2 === 1 ? tableClasses.rowAlt : ""}`} onClick={() => onOpen(r.id)}>
                <td className={`${tableClasses.td} font-mono text-xs`}>{r.id}</td>
                <td className={tableClasses.td}>{t(`host.osUpdate.kindShort.${r.kind}`)}{r.kind === "restore" ? ` · ${t(`host.osRestore.modeShort.${r.restore_mode ?? "staging"}`)}` : r.reboot ? ` · ${t("host.osUpdate.withReboot")}` : ""}</td>
                <td className={tableClasses.td}><Lozenge appearance={RUN_TONE[r.status]}>{t(`host.osUpdate.runStatus.${r.status}`)}</Lozenge></td>
                <td className={`${tableClasses.td} text-xs text-[var(--text-secondary)]`}>{countsText(r, t)}</td>
                <td className={`${tableClasses.td} text-xs text-[var(--text-muted)] whitespace-nowrap`}>
                  {getTimeAgo(r.created_at, locale)}{r.requested_name ? ` · ${r.requested_name}` : ""}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {openId !== null && <RunDrawer id={openId} onClose={() => onOpen(null)}
        onRestore={(x) => { onOpen(null); setRestoring(x); }} />}
      {/* Outside the drawer: a modal inside it would sit under it and lose focus to it. */}
      {restoring && <OSRestoreModal backup={restoring} onClose={() => setRestoring(null)}
        onStarted={(r) => { setRestoring(null); onOpen(r.id); }} />}
    </div>
  );
}

function countsText(r: OSUpdateRun, t: (k: string, v?: Record<string, string>) => string): string {
  return (["success", "failed", "skipped", "running", "pending"] as const)
    .filter((s) => r.counts[s])
    .map((s) => `${r.counts[s]} ${t(`host.osUpdate.resultStatus.${s}`).toLowerCase()}`)
    .join(" · ");
}

/** One run's per-host results (polls while it runs). */
export function RunDrawer({ id, onClose, onRestore }: { id: number; onClose: () => void; onRestore: (x: OSUpdateResult) => void }) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [openHost, setOpenHost] = useState<number | null>(null);
  const { data: run, isError, error } = useQuery({
    queryKey: ["os-updates", "run", id],
    queryFn: () => osUpdateAPI.run(id),
    refetchInterval: (q) => (q.state.data?.status === "running" ? 5000 : false),
  });
  const cancel = useMutation({
    mutationFn: () => osUpdateAPI.cancel(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["os-updates"] }),
  });

  return (
    <Drawer open onClose={onClose} wide title={t("host.osUpdate.runTitle", { id: String(id) })}
      headerAction={run?.status === "running" ? (
        <Button size="sm" variant="danger" onClick={() => cancel.mutate()} disabled={cancel.isPending}>{t("host.osUpdate.cancel")}</Button>
      ) : undefined}>
      {isError && <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>}
      {run?.status === "running" && <p className="text-xs text-[var(--text-muted)] mb-3">{t("host.osUpdate.cancelHint")}</p>}
      {run?.backup && (run.backup_include || run.backup_exclude) && (
        <p className="text-xs text-[var(--text-secondary)] mb-3">
          {t("host.backup.runPaths", { include: (run.backup_include ?? []).join(" ") || t("host.backup.inherited"), exclude: (run.backup_exclude ?? []).join(" ") || t("host.backup.inherited") })}
        </p>
      )}
      {run?.kind === "restore" && (
        <p className="text-xs text-[var(--text-secondary)] mb-3">
          {t(`host.osRestore.summary.${run.restore_mode ?? "staging"}`, { paths: (run.restore_paths ?? []).join(" ") || "/", run: String(run.source_run_id ?? "") })}
        </p>
      )}

      {run && (
        <div className={tableClasses.wrapper}>
          <table className={tableClasses.table}>
            <thead>
              <tr className={tableClasses.headRow}>
                <th className={tableClasses.th}>{t("host.osUpdate.host")}</th>
                <th className={tableClasses.th}>{t("host.osUpdate.status")}</th>
                <th className={tableClasses.th}>{t("host.osUpdate.pending")}</th>
                <th className={tableClasses.th}>{t("host.osUpdate.upgraded")}</th>
                <th className={tableClasses.th}>{t("host.os")}</th>
                {run.backup && <th className={tableClasses.th}>{t("host.osUpdate.backupCol")}</th>}
              </tr>
            </thead>
            <tbody>
              {(run.results ?? []).map((x) => (
                <Fragment key={x.host_id}>
                  <tr className={`${tableClasses.row} ${x.output ? "cursor-pointer" : ""}`} onClick={() => x.output && setOpenHost(openHost === x.host_id ? null : x.host_id)}>
                    <td className={tableClasses.td}>
                      <Link href={`/hosts/${x.host_slug}`} onClick={(e) => e.stopPropagation()} className="font-mono text-xs text-[var(--accent)] hover:underline">{x.host_nickname}</Link>
                    </td>
                    <td className={tableClasses.td}>
                      <Lozenge appearance={RESULT_TONE[x.status]}>{t(`host.osUpdate.resultStatus.${x.status}`)}</Lozenge>
                      {x.reason && <div className="text-xs text-[var(--text-muted)] mt-1 max-w-xs">{x.reason}</div>}
                    </td>
                    <td className={`${tableClasses.td} text-xs tabular-nums`}>{pendingText(x, t)}</td>
                    <td className={`${tableClasses.td} text-xs tabular-nums`}>{x.upgraded ?? "–"}</td>
                    <td className={`${tableClasses.td} text-xs`}>
                      {x.os_after && x.os_after !== x.os_before ? `${x.os_before || "?"} → ${x.os_after}` : x.os_after || x.os_before || "–"}
                      {x.rebooted && <Lozenge appearance="inprogress" className="ml-1.5">{t("host.osUpdate.rebooted")}</Lozenge>}
                      {!x.rebooted && x.reboot_required && <Lozenge appearance="moved" className="ml-1.5">{t("host.rebootPending")}</Lozenge>}
                    </td>
                    {run.backup && (
                      <td className={`${tableClasses.td} text-xs`} onClick={(e) => e.stopPropagation()}>
                        {x.backup_status === "done" ? <BackupActions backup={x} onRestore={onRestore} /> : x.backup_status ? (
                          <span className={x.backup_status === "failed" ? "text-[var(--danger)]" : "text-[var(--text-muted)]"}>{t(`host.osUpdate.backupStatus.${x.backup_status}`)}</span>
                        ) : <span className="text-[var(--text-muted)]">–</span>}
                      </td>
                    )}
                  </tr>
                  {openHost === x.host_id && (
                    <tr className={tableClasses.row}>
                      <td className={tableClasses.td} colSpan={run.backup ? 6 : 5}>
                        <pre className="text-xs font-mono whitespace-pre-wrap max-h-80 overflow-auto bg-[var(--bg-elevated)] rounded-[var(--radius-md)] p-3">{x.output}</pre>
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Drawer>
  );
}

/** "12 (3 seg.)": still upgradable, security ones in parentheses. */
export function pendingText(x: Pick<OSUpdateResult, "upgradable" | "security_upgradable">, t: (k: string, v?: Record<string, string>) => string): string {
  if (x.upgradable == null) return "–";
  return x.security_upgradable ? `${x.upgradable} (${t("host.osUpdate.securityShort", { count: String(x.security_upgradable) })})` : String(x.upgradable);
}
