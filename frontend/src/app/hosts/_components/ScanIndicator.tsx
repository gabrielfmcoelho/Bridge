"use client";

import { getTimeAgo } from "@/lib/utils";
import { useLocale } from "@/contexts/LocaleContext";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function ScanIndicator({ hasScan, lastScanAt }: { hasScan?: boolean; lastScanAt?: string }) {
  const { t } = useLocale();
  const timeAgo = lastScanAt ? getTimeAgo(lastScanAt) : null;

  return (
    <span
      className="inline-flex items-center gap-1"
      role="img"
      aria-label={hasScan ? `${t("scan.lastScanLabel")} ${timeAgo}` : t("scan.noScanData")}
      title={hasScan ? `${t("scan.lastScanLabel")} ${timeAgo}` : t("scan.noScanData")}
    >
      <Icon path={ICON_PATHS.scan} className={`w-3.5 h-3.5 ${hasScan ? "text-[var(--success)]" : "text-[var(--text-muted)]"}`} />
    </span>
  );
}
