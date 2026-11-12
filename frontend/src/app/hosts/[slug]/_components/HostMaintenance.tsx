"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import Field from "@/components/ui/Field";
import Lozenge from "@/components/ui/Lozenge";
import StatusAlert from "@/components/ui/StatusAlert";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { osUpdateAPI, type VMInfoType } from "@/lib/api";
import { getTimeAgo } from "@/lib/utils";
import type { Host, OSUpdateKind, OSUpdateResult } from "@/lib/types";
import { osLabel } from "../../_components/hostInsights";
import OSBadges from "../../_components/OSBadges";
import OSUpdateModal from "../../_components/OSUpdateModal";
import OSUpdateRuns, { pendingText } from "../../_components/OSUpdateRuns";
import OSRestoreModal from "../../_components/OSRestoreModal";
import HostBackupModal from "../../_components/HostBackupModal";
import BackupActions from "../../_components/BackupActions";

/** The host's "Manutenção" tab (admin): its OS and pending updates, how it is
 *  backed up, the backups kept, and every update / backup / restore run on it.
 *  Every action opens the same modals as the hosts list and the OS tab. */
export default function HostMaintenance({ host, scanData }: { host: Host; scanData?: string }) {
  const { t, locale } = useLocale();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const scan = useMemo<Partial<VMInfoType> | undefined>(() => {
    try { return scanData ? JSON.parse(scanData) : undefined; } catch { return undefined; }
  }, [scanData]);
  const target = { id: host.id, nickname: host.nickname, slug: host.oficial_slug, os: osLabel(host) };

  const [update, setUpdate] = useState<OSUpdateKind | null>(null);
  const [configuring, setConfiguring] = useState(false);
  const [restoring, setRestoring] = useState<OSUpdateResult | null>(null);
  const [openRun, setOpenRun] = useState<number | null>(null);

  const { data: config } = useQuery({ queryKey: ["os-updates", "config"], queryFn: osUpdateAPI.config });
  const { data: latest = [] } = useQuery({ queryKey: ["os-updates", "latest"], queryFn: osUpdateAPI.latest });
  const last = latest.find((x) => x.host_id === host.id);
  const { data: profile } = useQuery({ queryKey: ["backup-profile", host.oficial_slug], queryFn: () => osUpdateAPI.backupProfile(host.oficial_slug) });
  const { data: kept = [], isError: keptError, error: keptErr } = useQuery({ queryKey: ["os-updates", "backups", host.id], queryFn: () => osUpdateAPI.backups(host.id) });
  const custom = !!(profile?.profile.include || profile?.profile.exclude);

  const backupNow = useMutation({
    mutationFn: () => osUpdateAPI.start({ kind: "backup", host_ids: [host.id], reboot: false, concurrency: 1 }),
    onSuccess: (run) => {
      queryClient.invalidateQueries({ queryKey: ["os-updates"] });
      flag({ appearance: "info", title: t("host.backup.started", { host: host.nickname }) });
      setOpenRun(run.id);
    },
    onError: (err) => flag({ appearance: "error", title: t("host.backup.failed"), description: (err as Error).message }),
  });

  return (
    <div className="space-y-5 animate-fade-in">
      <SectionCard as="h3" title={t("host.maintenance.system")}
        controls={<div className="flex flex-wrap gap-2">
          <Button size="sm" variant="secondary" onClick={() => setUpdate("check")}>{t("host.maintenance.check")}</Button>
          <Button size="sm" onClick={() => setUpdate("security")}>{t("host.maintenance.update")}</Button>
          <Button size="sm" variant="secondary" onClick={() => setUpdate("release")}>{t("host.osRelease.start")}</Button>
        </div>}>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
          <Field label={t("host.os")} value={host.os?.pretty || osLabel(host) || "–"} />
          <Field label={t("vm.arch")} value={host.os?.arch || scan?.arch || "–"} />
          <Field label={t("host.maintenance.kernel")} value={scan?.kernel || "–"} mono />
          <Field label={t("vuln.deps.eol")} value={host.os?.eol ? (host.os.eol_date || t("vuln.deps.eolYes"))
            : host.os?.eol_date ? t("vuln.deps.eolUntil", { date: host.os.eol_date }) : "–"} />
          <Field label={t("host.osUpdate.pending")} value={last ? `${pendingText(last, t)} · ${getTimeAgo(last.finished_at ?? "", locale)}` : t("host.maintenance.neverChecked")} />
          <div><OSBadges os={host.os} /></div>
        </div>
      </SectionCard>

      <SectionCard as="h3" title={t("host.maintenance.backup")}
        description={config?.backup_available ? undefined : t("host.osUpdate.backupUnavailable")}
        controls={<div className="flex flex-wrap gap-2">
          <Button size="sm" variant="secondary" onClick={() => setConfiguring(true)}>{t("host.backup.configure")}</Button>
          <Button size="sm" onClick={() => backupNow.mutate()} loading={backupNow.isPending} disabled={!config?.backup_available}>{t("host.backup.run")}</Button>
        </div>}>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label={t("settings.integrations.minio.include")} value={(profile?.effective.include ?? []).join(" ")} mono wrap />
          <Field label={t("settings.integrations.minio.exclude")} value={(profile?.effective.exclude ?? []).join(" ")} mono wrap />
          <Field label={t("host.backup.profile")} value={custom ? t("host.backup.custom") : t("host.backup.usesDefaults")} />
          <div><OSBadges backup={kept[0]} showNoBackup /></div>
        </div>
      </SectionCard>

      <SectionCard as="h3" title={t("host.maintenance.kept")} count={kept.length} body="flush"
        empty={kept.length === 0 ? t("host.maintenance.noKept") : undefined}>
        {keptError && <StatusAlert variant="error">{(keptErr as Error)?.message}</StatusAlert>}
        {kept.length > 0 && (
          <div className={tableClasses.wrapper}>
            <table className={tableClasses.table}>
              <thead>
                <tr className={tableClasses.headRow}>
                  <th className={tableClasses.th}>{t("host.osUpdate.when")}</th>
                  <th className={tableClasses.th}>{t("host.osUpdate.kind")}</th>
                  <th className={tableClasses.th}>{t("host.maintenance.expires")}</th>
                  <th className={tableClasses.th}>{t("host.osUpdate.backupCol")}</th>
                </tr>
              </thead>
              <tbody>
                {kept.map((x) => (
                  <tr key={x.run_id} className={tableClasses.row}>
                    <td className={`${tableClasses.td} text-xs whitespace-nowrap`} title={(x.backup_include ?? []).join(" ")}>
                      {x.finished_at ? getTimeAgo(x.finished_at, locale) : "–"} · <button type="button" className="font-mono text-[var(--accent)] hover:underline" onClick={() => setOpenRun(x.run_id)}>#{x.run_id}</button>
                    </td>
                    <td className={tableClasses.td}><Lozenge>{t(`host.osUpdate.kindShort.${x.run_kind ?? "packages"}`)}</Lozenge></td>
                    <td className={`${tableClasses.td} text-xs text-[var(--text-muted)]`}>{x.expires_at ? new Date(x.expires_at).toLocaleDateString(locale) : t("host.maintenance.keptForever")}</td>
                    <td className={`${tableClasses.td} text-xs`}><BackupActions backup={x} onRestore={setRestoring} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </SectionCard>

      <OSUpdateRuns hostId={host.id} openId={openRun} onOpen={setOpenRun} />

      {update && (
        <OSUpdateModal target={{ by: "hosts", hosts: [target] }} preset={update} scan={scan} onClose={() => setUpdate(null)}
          onStarted={(run) => { setUpdate(null); setOpenRun(run.id); }} />
      )}
      {configuring && (
        <HostBackupModal host={target} scan={scan} lastSize={kept[0]?.backup_size} onClose={() => setConfiguring(false)}
          onStarted={(run) => { setConfiguring(false); setOpenRun(run.id); }} />
      )}
      {restoring && <OSRestoreModal backup={restoring} onClose={() => setRestoring(null)}
        onStarted={(r) => { setRestoring(null); setOpenRun(r.id); }} />}
    </div>
  );
}
