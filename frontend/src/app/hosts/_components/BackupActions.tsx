"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Button from "@/components/ui/Button";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { osUpdateAPI } from "@/lib/api";
import { formatBytes } from "@/lib/backupPreview";
import type { OSUpdateResult } from "@/lib/types";

/** A kept backup's size and actions: download (a link straight to MinIO, no
 *  Bridge proxy in the way of a big file), restore, delete. */
export default function BackupActions({ backup: x, onRestore }: { backup: OSUpdateResult; onRestore: (x: OSUpdateResult) => void }) {
  const { t } = useLocale();
  const flag = useFlag();
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const download = useMutation({
    mutationFn: () => osUpdateAPI.backupLink(x.run_id, x.host_id),
    onSuccess: (link) => { window.location.href = link.url; },
    onError: (err) => flag({ appearance: "error", title: t("host.osUpdate.backupLinkFailed"), description: (err as Error).message }),
  });
  const remove = useMutation({
    mutationFn: () => osUpdateAPI.deleteBackup(x.run_id, x.host_id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["os-updates"] }),
    onError: (err) => flag({ appearance: "error", title: t("host.osUpdate.backupDeleteFailed"), description: (err as Error).message }),
  });
  const askRemove = async () => {
    if (await confirm({ title: t("host.osUpdate.backupDeleteTitle", { host: x.host_nickname }), confirmLabel: t("common.delete"), danger: true })) remove.mutate();
  };
  return (
    <span className="flex flex-wrap items-center gap-1.5 min-w-[9rem]">
      <span className="tabular-nums text-[var(--text-secondary)]">{x.backup_size != null ? formatBytes(x.backup_size) : ""}</span>
      <Button size="sm" variant="secondary" onClick={() => download.mutate()} disabled={download.isPending}>{t("host.osUpdate.backupDownload")}</Button>
      <Button size="sm" variant="secondary" onClick={() => onRestore(x)}>{t("host.osRestore.open")}</Button>
      <Button size="sm" variant="ghost" onClick={askRemove} disabled={remove.isPending}>{t("common.delete")}</Button>
    </span>
  );
}
