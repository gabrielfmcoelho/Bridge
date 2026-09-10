import { getTimeAgo } from "@/lib/utils";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function ScanIndicator({ hasScan, lastScanAt }: { hasScan?: boolean; lastScanAt?: string }) {
  const timeAgo = lastScanAt ? getTimeAgo(lastScanAt) : null;

  return (
    <span
      className="inline-flex items-center gap-1"
      title={hasScan ? `Last scan: ${timeAgo}` : "No scan data"}
    >
      <Icon path={ICON_PATHS.scan} className={`w-3.5 h-3.5 ${hasScan ? "text-[var(--success)]" : "text-[var(--text-faint)]"}`} />
    </span>
  );
}
