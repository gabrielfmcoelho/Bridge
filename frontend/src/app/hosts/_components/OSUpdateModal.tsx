"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import RadioGroup from "@/components/ui/RadioGroup";
import CheckboxList from "@/components/ui/CheckboxList";
import Toggle from "@/components/ui/Toggle";
import Input from "@/components/ui/Input";
import FormField from "@/components/ui/FormField";
import FormFooter from "@/components/ui/FormFooter";
import FormError from "@/components/ui/FormError";
import StatusAlert from "@/components/ui/StatusAlert";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import { osUpdateAPI, type VMInfoType } from "@/lib/api";
import { parsePaths } from "@/lib/backupPreview";
import type { DependencyGroup, OSUpdateKind, OSUpdateRun } from "@/lib/types";
import { osKey } from "./osView";
import BackupPathsEditor from "./BackupPathsEditor";

/** A host an update can target. */
export interface UpdateHost { id: number; nickname: string; slug: string; os?: string }

/** Who a run targets: distro versions (the OS tab) or given hosts (a host's
 *  page, the hosts list, Vulnerabilidades). */
export type UpdateTarget =
  | { by: "groups"; groups: DependencyGroup[]; hostIds: Map<string, number[]> }
  | { by: "hosts"; hosts: UpdateHost[] };

/** The one modal that starts an OS update or backup run, from every entry
 *  point. Release (next version) only on one host; backup-only keeps it. */
export default function OSUpdateModal({ target, preset = "check", scan, onClose, onStarted }: {
  target: UpdateTarget;
  preset?: OSUpdateKind;
  /** The single host's latest scan, for the backup preview. */
  scan?: Partial<VMInfoType>;
  onClose: () => void;
  onStarted: (run: OSUpdateRun) => void;
}) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const groups = target.by === "groups" ? target.groups : [];
  const [selected, setSelected] = useState<number[]>(() => groups.map((_, i) => i));
  const targets = target.by === "groups"
    ? selected.flatMap((i) => target.hostIds.get(osKey(groups[i].name, groups[i].version)) ?? [])
    : target.hosts.map((h) => h.id);
  const single = target.by === "hosts" && target.hosts.length === 1 ? target.hosts[0] : null;

  const [kind, setKind] = useState<OSUpdateKind>(preset === "release" && !single ? "check" : preset);
  const release = kind === "release";
  const backupOnly = kind === "backup";
  const [reboot, setReboot] = useState(false);
  const [concurrency, setConcurrency] = useState("3");
  const { data: config } = useQuery({ queryKey: ["os-updates", "config"], queryFn: osUpdateAPI.config });
  const backupAvailable = !!config?.backup_available;
  const [backup, setBackup] = useState(true);
  const [keepBackup, setKeepBackup] = useState(false);
  // A release always backs up when it can; backup-only is nothing but that.
  const withBackup = kind !== "check" && backupAvailable && (release || backupOnly || backup);
  // One host: its own paths can be overridden for this run.
  const { data: profile } = useQuery({ queryKey: ["backup-profile", single?.slug], queryFn: () => osUpdateAPI.backupProfile(single!.slug), enabled: !!single && withBackup });
  const [paths, setPaths] = useState({ include: "", exclude: "" });
  const pathsInvalid = [...parsePaths(paths.include), ...parsePaths(paths.exclude)].some((p) => !p.startsWith("/"));

  const conc = Number(concurrency);
  const concError = targets.length < 2 || (Number.isInteger(conc) && conc >= 1 && conc <= 10) ? "" : t("host.osUpdate.concurrencyRange");

  const start = useMutation({
    mutationFn: () => osUpdateAPI.start({
      kind, host_ids: targets, reboot: release || (!backupOnly && kind !== "check" && reboot), concurrency: release ? 1 : Math.min(conc || 1, targets.length || 1),
      backup: withBackup, keep_backup: withBackup && (backupOnly || keepBackup),
      ...(withBackup && single ? { backup_include: parsePaths(paths.include), backup_exclude: parsePaths(paths.exclude) } : {}),
    }),
    onSuccess: (run) => {
      queryClient.invalidateQueries({ queryKey: ["os-updates"] });
      flag({ appearance: "info", title: t("host.osUpdate.started", { id: String(run.id), count: String(targets.length) }) });
      onStarted(run);
    },
  });

  const submit = async () => {
    if (release && single) {
      // One host, typed name: a release upgrade can leave a VM unbootable.
      const ok = await confirm({
        title: t("host.osRelease.confirmTitle", { host: single.nickname, os: single.os || "?" }),
        message: `${t("host.osRelease.confirmMessage")} ${withBackup ? t("host.osRelease.withBackup") : t("host.osRelease.noBackup")}`,
        confirmLabel: t("host.osRelease.start"), danger: true, requireText: single.nickname,
      });
      if (!ok) return;
    } else if (kind !== "check" && !backupOnly) {
      const ok = await confirm({
        title: t(`host.osUpdate.confirmTitle.${kind}`, { count: String(targets.length) }),
        message: `${reboot ? t("host.osUpdate.confirmReboot") : t("host.osUpdate.confirmNoReboot")} ${withBackup ? t("host.osUpdate.confirmBackup") : t("host.osUpdate.confirmNoBackup")}`,
        confirmLabel: t("host.osUpdate.start"), danger: reboot, requireText: t("host.osUpdate.confirmWord"),
      });
      if (!ok) return;
    }
    start.mutate();
  };

  const kinds: OSUpdateKind[] = ["check", "security", "packages", ...(single ? ["release" as const] : []), "backup"];
  return (
    <ResponsiveModal open onClose={onClose} title={single ? t("host.osUpdate.titleHost", { host: single.nickname }) : t("host.osUpdate.title")}
      footer={<FormFooter onCancel={onClose} submitLabel={t(backupOnly ? "host.backup.run" : "host.osUpdate.start")} onSubmit={submit}
        loading={start.isPending} disabled={targets.length === 0 || !!concError || (backupOnly && !backupAvailable) || (withBackup && pathsInvalid)}
        variant={release ? "danger" : "primary"} />}>
      <div className="space-y-4">
        <RadioGroup label={t("host.osUpdate.kind")} value={kind} onChange={(v) => setKind(v as OSUpdateKind)}
          options={kinds.map((k) => ({ value: k, label: t(`host.osUpdate.kindOption.${k}`) }))} />
        <p className="text-xs text-[var(--text-muted)]">{t(`host.osUpdate.kindHint.${kind}`)}</p>

        {target.by === "groups" ? (
          <CheckboxList label={t("host.osUpdate.versions", { count: String(targets.length) })}
            items={groups.map((g, i) => ({ id: i, name: `${g.name} ${g.version} (${target.hostIds.get(osKey(g.name, g.version))?.length ?? 0})` }))}
            selected={selected} onChange={setSelected} />
        ) : !single && (
          <FormField label={t("host.osUpdate.hostsCount", { count: String(targets.length) })}>
            <p className="text-xs font-mono text-[var(--text-secondary)] max-h-20 overflow-auto">{target.hosts.map((h) => h.nickname).join(", ")}</p>
          </FormField>
        )}

        {release && <StatusAlert variant="warning">{t("host.osRelease.modalHint")}</StatusAlert>}
        {kind !== "check" && !backupOnly && !release && (
          <FormField label={t("host.osUpdate.reboot")} hint={t("host.osUpdate.rebootHint")}>
            <Toggle checked={reboot} onChange={setReboot} ariaLabel={t("host.osUpdate.reboot")} />
          </FormField>
        )}

        {kind !== "check" && (backupAvailable ? (
          <>
            {!release && !backupOnly && (
              <FormField label={t("host.osUpdate.backup")} hint={t("host.osUpdate.backupHintProfile")}>
                <Toggle checked={backup} onChange={setBackup} ariaLabel={t("host.osUpdate.backup")} />
              </FormField>
            )}
            {withBackup && !backupOnly && (
              <FormField label={t("host.osUpdate.keepBackup")} hint={`${t("host.osUpdate.keepBackupHint")} ${config?.retention_days
                ? t("host.osUpdate.retention", { days: String(config.retention_days) }) : t("host.osUpdate.retentionOff")}`}>
                <Toggle checked={keepBackup} onChange={setKeepBackup} ariaLabel={t("host.osUpdate.keepBackup")} />
              </FormField>
            )}
            {withBackup && (single && profile ? (
              <BackupPathsEditor include={paths.include} exclude={paths.exclude} onChange={(include, exclude) => setPaths({ include, exclude })}
                inherited={profile.effective} from={profile.profile.include || profile.profile.exclude ? "host" : "defaults"} scan={scan} />
            ) : !single && <p className="text-xs text-[var(--text-muted)]">{t("host.backup.eachProfile")}</p>)}
          </>
        ) : (
          <p className="text-xs text-[var(--text-muted)]">{t("host.osUpdate.backupUnavailable")}</p>
        ))}

        {targets.length > 1 && !release && (
          <Input label={t("host.osUpdate.concurrency")} type="number" min={1} max={10} value={concurrency}
            onChange={(e) => setConcurrency(e.target.value)} error={concError || undefined} hint={t("host.osUpdate.concurrencyHint")} />
        )}
        {kind !== "check" && <StatusAlert variant="warning">{t("host.osUpdate.privilegeHint")}</StatusAlert>}
        <FormError message={start.error ? (start.error as Error).message : ""} />
      </div>
    </ResponsiveModal>
  );
}
