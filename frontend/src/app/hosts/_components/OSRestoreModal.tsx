"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import RadioGroup from "@/components/ui/RadioGroup";
import Textarea from "@/components/ui/Textarea";
import FormFooter from "@/components/ui/FormFooter";
import FormError from "@/components/ui/FormError";
import StatusAlert from "@/components/ui/StatusAlert";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { osUpdateAPI } from "@/lib/api";
import type { OSUpdateResult, OSUpdateRun } from "@/lib/types";

type Mode = "staging" | "paths";

/** Restores a kept backup onto its host: into a review folder, or the chosen
 *  paths in place (those paths are backed up first: the undo). */
export default function OSRestoreModal({ backup, onClose, onStarted }: {
  backup: OSUpdateResult;
  onClose: () => void;
  onStarted: (run: OSUpdateRun) => void;
}) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<Mode>("staging");
  const [paths, setPaths] = useState("");
  const list = paths.split(/\s+/).filter(Boolean);
  const pathError = list.some((p) => !p.startsWith("/")) ? t("host.osRestore.absolute")
    : mode === "paths" && list.some((p) => p.replace(/\/+/g, "/") === "/") ? t("host.osRestore.noRoot")
    : mode === "paths" && list.length === 0 ? t("host.osRestore.pathsRequired") : "";

  const start = useMutation({
    mutationFn: () => osUpdateAPI.restore({ run_id: backup.run_id, host_id: backup.host_id, mode, paths: list }),
    onSuccess: (run) => {
      queryClient.invalidateQueries({ queryKey: ["os-updates"] });
      onStarted(run);
    },
  });

  const submit = async () => {
    if (mode === "paths") {
      const ok = await confirm({
        title: t("host.osRestore.confirmTitle", { host: backup.host_nickname }),
        message: t("host.osRestore.confirmMessage", { paths: list.join(" ") }),
        confirmLabel: t("host.osRestore.start"),
        danger: true,
      });
      if (!ok) return;
    }
    start.mutate();
  };

  return (
    <ResponsiveModal open onClose={onClose} title={t("host.osRestore.title", { host: backup.host_nickname })}
      footer={<FormFooter onCancel={onClose} submitLabel={t("host.osRestore.start")} onSubmit={submit}
        loading={start.isPending} disabled={!!pathError} variant={mode === "paths" ? "danger" : "primary"} />}>
      <div className="space-y-4">
        <p className="text-xs text-[var(--text-muted)] font-mono break-all">{backup.backup_key}</p>
        <RadioGroup label={t("host.osRestore.mode")} value={mode} onChange={(v) => setMode(v as Mode)} options={[
          { value: "staging", label: t("host.osRestore.modeStaging") },
          { value: "paths", label: t("host.osRestore.modePaths") },
        ]} />
        <p className="text-xs text-[var(--text-muted)]">{t(`host.osRestore.modeHint.${mode}`)}</p>
        <Textarea label={mode === "paths" ? t("host.osRestore.paths") : t("host.osRestore.pathsOptional")} rows={3}
          value={paths} onChange={(e) => setPaths(e.target.value)} placeholder="/etc/nginx /opt/app"
          error={paths ? pathError || undefined : undefined} hint={t("host.osRestore.pathsHint")} />
        {mode === "paths" && <StatusAlert variant="warning">{t("host.osRestore.warning")}</StatusAlert>}
        <FormError message={start.error ? (start.error as Error).message : ""} />
      </div>
    </ResponsiveModal>
  );
}
