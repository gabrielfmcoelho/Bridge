"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import Lozenge from "@/components/ui/Lozenge";
import { useLocale } from "@/contexts/LocaleContext";
import { osUpdateAPI } from "@/lib/api";
import { getTimeAgo } from "@/lib/utils";
import type { HostOS, OSUpdateResult } from "@/lib/types";

/** Each host's newest kept backup (admins only: backups are admin data). */
export function useLatestBackups(enabled: boolean): Map<number, OSUpdateResult> {
  const { data = [] } = useQuery({ queryKey: ["os-updates", "backups", "latest"], queryFn: () => osUpdateAPI.backups(), enabled });
  return useMemo(() => new Map(data.map((x) => [x.host_id, x])), [data]);
}

/** The one look of a host's OS state everywhere (table, OS tab, host page):
 *  end of life, reboot pending, and its newest kept backup. */
export default function OSBadges({ os, backup, showNoBackup }: { os?: HostOS; backup?: OSUpdateResult; showNoBackup?: boolean }) {
  const { t, locale } = useLocale();
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      {os?.eol && <span title={os.eol_date}><Lozenge appearance="removed">{t("host.osEol")}</Lozenge></span>}
      {os?.reboot_required && <Lozenge appearance="moved">{t("host.rebootPending")}</Lozenge>}
      {backup?.finished_at ? (
        <span title={backup.expires_at ? t("host.backupExpires", { date: new Date(backup.expires_at).toLocaleDateString(locale) }) : t("host.backupKept")}>
          <Lozenge appearance="success">{t("host.backupBadge", { age: getTimeAgo(backup.finished_at, locale) })}</Lozenge>
        </span>
      ) : showNoBackup && <Lozenge appearance="default">{t("host.noBackup")}</Lozenge>}
    </span>
  );
}
