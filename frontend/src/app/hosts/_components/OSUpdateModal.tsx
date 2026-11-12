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
import { osUpdateAPI } from "@/lib/api";
import type { DependencyGroup, OSUpdateKind, OSUpdateRun } from "@/lib/types";
import { osKey } from "./osView";

/** Starts an OS update run on the hosts of the chosen distro versions. */
export default function OSUpdateModal({ groups, hostIds, onClose, onStarted }: {
  groups: DependencyGroup[];
  /** Host ids per osKey. */
  hostIds: Map<string, number[]>;
  onClose: () => void;
  onStarted: (run: OSUpdateRun) => void;
}) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const [kind, setKind] = useState<OSUpdateKind>("check");
  const [selected, setSelected] = useState<number[]>(() => groups.map((_, i) => i));
  const [reboot, setReboot] = useState(false);
  const [concurrency, setConcurrency] = useState("3");
  const { data: config } = useQuery({ queryKey: ["os-updates", "config"], queryFn: osUpdateAPI.config });
  const backupAvailable = !!config?.backup_available;
  const [backup, setBackup] = useState(true);
  const [keepBackup, setKeepBackup] = useState(false);
  const withBackup = kind !== "check" && backupAvailable && backup;

  const targets = selected.flatMap((i) => hostIds.get(osKey(groups[i].name, groups[i].version)) ?? []);
  const conc = Number(concurrency);
  const concError = Number.isInteger(conc) && conc >= 1 && conc <= 10 ? "" : t("host.osUpdate.concurrencyRange");

  const start = useMutation({
    mutationFn: () => osUpdateAPI.start({ kind, host_ids: targets, reboot: kind !== "check" && reboot, concurrency: conc, backup: withBackup, keep_backup: withBackup && keepBackup }),
    onSuccess: (run) => {
      queryClient.invalidateQueries({ queryKey: ["os-updates"] });
      flag({ appearance: "info", title: t("host.osUpdate.started", { id: String(run.id), count: String(targets.length) }) });
      onStarted(run);
    },
  });

  const submit = async () => {
    if (kind !== "check") {
      const word = t("host.osUpdate.confirmWord");
      const ok = await confirm({
        title: t(`host.osUpdate.confirmTitle.${kind}`, { count: String(targets.length) }),
        message: `${reboot ? t("host.osUpdate.confirmReboot") : t("host.osUpdate.confirmNoReboot")} ${withBackup ? t("host.osUpdate.confirmBackup") : t("host.osUpdate.confirmNoBackup")}`,
        confirmLabel: t("host.osUpdate.start"),
        danger: reboot,
        requireText: word,
      });
      if (!ok) return;
    }
    start.mutate();
  };

  return (
    <ResponsiveModal open onClose={onClose} title={t("host.osUpdate.title")}
      footer={<FormFooter onCancel={onClose} submitLabel={t("host.osUpdate.start")} onSubmit={submit}
        loading={start.isPending} disabled={targets.length === 0 || !!concError} />}>
      <div className="space-y-4">
        <RadioGroup label={t("host.osUpdate.kind")} value={kind} onChange={(v) => setKind(v as OSUpdateKind)} options={[
          { value: "check", label: t("host.osUpdate.kindCheck") },
          { value: "security", label: t("host.osUpdate.kindSecurity") },
          { value: "packages", label: t("host.osUpdate.kindPackages") },
        ]} />
        <p className="text-xs text-[var(--text-muted)]">{t(`host.osUpdate.kindHint.${kind}`)}</p>
        <CheckboxList label={t("host.osUpdate.versions", { count: String(targets.length) })}
          items={groups.map((g, i) => ({ id: i, name: `${g.name} ${g.version} (${hostIds.get(osKey(g.name, g.version))?.length ?? 0})` }))}
          selected={selected} onChange={setSelected} />
        {kind !== "check" && (
          <FormField label={t("host.osUpdate.reboot")} hint={t("host.osUpdate.rebootHint")}>
            <Toggle checked={reboot} onChange={setReboot} ariaLabel={t("host.osUpdate.reboot")} />
          </FormField>
        )}
        {kind !== "check" && (backupAvailable ? (
          <>
            <FormField label={t("host.osUpdate.backup")} hint={t("host.osUpdate.backupHint", { paths: (config?.backup_include ?? []).join(" ") })}>
              <Toggle checked={backup} onChange={setBackup} ariaLabel={t("host.osUpdate.backup")} />
            </FormField>
            {backup && (
              <FormField label={t("host.osUpdate.keepBackup")} hint={`${t("host.osUpdate.keepBackupHint")} ${config?.retention_days
                ? t("host.osUpdate.retention", { days: String(config.retention_days) }) : t("host.osUpdate.retentionOff")}`}>
                <Toggle checked={keepBackup} onChange={setKeepBackup} ariaLabel={t("host.osUpdate.keepBackup")} />
              </FormField>
            )}
          </>
        ) : (
          <p className="text-xs text-[var(--text-muted)]">{t("host.osUpdate.backupUnavailable")}</p>
        ))}
        <Input label={t("host.osUpdate.concurrency")} type="number" min={1} max={10} value={concurrency}
          onChange={(e) => setConcurrency(e.target.value)} error={concError || undefined} hint={t("host.osUpdate.concurrencyHint")} />
        {kind !== "check" && <StatusAlert variant="warning">{t("host.osUpdate.privilegeHint")}</StatusAlert>}
        <FormError message={start.error ? (start.error as Error).message : ""} />
      </div>
    </ResponsiveModal>
  );
}
