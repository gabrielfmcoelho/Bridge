"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormFooter from "@/components/ui/FormFooter";
import FormError from "@/components/ui/FormError";
import Button from "@/components/ui/Button";
import StatusAlert from "@/components/ui/StatusAlert";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { osUpdateAPI, type VMInfoType } from "@/lib/api";
import { parsePaths } from "@/lib/backupPreview";
import type { OSUpdateRun } from "@/lib/types";
import BackupPathsEditor from "./BackupPathsEditor";

/** "Configurar backup…": the host's own backup paths (empty = the defaults),
 *  previewed against its latest scan; saving can start a backup right away. */
export default function HostBackupModal({ host, scan, lastSize, onClose, onStarted }: {
  host: { id: number; nickname: string; slug: string };
  scan?: Partial<VMInfoType>;
  lastSize?: number;
  onClose: () => void;
  onStarted: (run: OSUpdateRun) => void;
}) {
  const { t } = useLocale();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const { data: view, isLoading } = useQuery({ queryKey: ["backup-profile", host.slug], queryFn: () => osUpdateAPI.backupProfile(host.slug) });
  const { data: config } = useQuery({ queryKey: ["os-updates", "config"], queryFn: osUpdateAPI.config });
  // null until edited: the stored profile shows through.
  const [edit, setEdit] = useState<{ include: string; exclude: string } | null>(null);
  const include = edit?.include ?? (view?.profile.include ?? []).join(" ");
  const exclude = edit?.exclude ?? (view?.profile.exclude ?? []).join(" ");
  const invalid = [...parsePaths(include), ...parsePaths(exclude)].some((p) => !p.startsWith("/"));

  const save = useMutation({
    mutationFn: async (andBackup: boolean) => {
      // An empty field sends [], back to the default.
      await osUpdateAPI.saveBackupProfile(host.slug, { include: parsePaths(include), exclude: parsePaths(exclude) });
      return andBackup ? osUpdateAPI.start({ kind: "backup", host_ids: [host.id], reboot: false, concurrency: 1 }) : null;
    },
    onSuccess: (run) => {
      queryClient.invalidateQueries({ queryKey: ["backup-profile", host.slug] });
      queryClient.invalidateQueries({ queryKey: ["os-updates"] });
      flag({ appearance: "success", title: run ? t("host.backup.started", { host: host.nickname }) : t("host.backup.saved") });
      if (run) onStarted(run);
      else onClose();
    },
  });

  return (
    <ResponsiveModal open onClose={onClose} title={t("host.backup.configureTitle", { host: host.nickname })}
      footer={<FormFooter onCancel={onClose} submitLabel={t("common.save")} onSubmit={() => save.mutate(false)}
        loading={save.isPending && save.variables === false} disabled={invalid || !view}
        start={config?.backup_available ? (
          <Button variant="secondary" onClick={() => save.mutate(true)} disabled={invalid || !view || save.isPending}
            loading={save.isPending && save.variables === true}>{t("host.backup.saveAndRun")}</Button>
        ) : undefined} />}>
      <div className="space-y-4">
        {config && !config.backup_available && <StatusAlert variant="warning">{t("host.osUpdate.backupUnavailable")}</StatusAlert>}
        {isLoading || !view ? <SkeletonTable rows={4} /> : (
          <BackupPathsEditor include={include} exclude={exclude} onChange={(i, e) => setEdit({ include: i, exclude: e })}
            inherited={view.defaults} from="defaults" scan={scan} lastSize={lastSize} />
        )}
        {!scan && <p className="text-xs text-[var(--text-muted)]">{t("host.backup.noScan")}</p>}
        <FormError message={save.error ? (save.error as Error).message : ""} />
      </div>
    </ResponsiveModal>
  );
}
