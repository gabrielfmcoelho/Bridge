import Button from "@/components/ui/Button";
import type { Host } from "@/lib/types";

export default function BatchScanModal({
  scanning,
  scanProgress,
  scannableHosts,
  scannedCount,
  successCount,
  failedCount,
  onStart,
  onStop,
  onClose,
  t,
}: {
  scanning: boolean;
  scanProgress: Record<string, { status: "pending" | "scanning" | "success" | "failed" | "skipped"; error?: string }>;
  scannableHosts: Host[];
  scannedCount: number;
  successCount: number;
  failedCount: number;
  onStart: () => void;
  onStop: () => void;
  onClose: () => void;
  t: (key: string) => string;
}) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-[var(--text-secondary)]">
        {t("host.scanAllDescription")} ({scannableHosts.length} {t("host.title").toLowerCase()})
      </p>

      {scanning && (
        <div>
          <div className="flex items-center justify-between text-xs text-[var(--text-muted)] mb-1.5">
            <span>{t("host.scanProgress")}</span>
            <span style={{ fontFamily: "var(--font-mono)" }}>{scannedCount}/{scannableHosts.length}</span>
          </div>
          <div className="h-2 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
            <div
              className="h-full rounded-full bg-[var(--accent)] transition-all duration-300"
              style={{ width: `${scannableHosts.length ? (scannedCount / scannableHosts.length) * 100 : 0}%` }}
            />
          </div>
          {(successCount > 0 || failedCount > 0) && (
            <div className="flex gap-3 mt-1.5 text-xs">
              <span className="text-emerald-400">{successCount} OK</span>
              {failedCount > 0 && <span className="text-red-400">{failedCount} failed</span>}
            </div>
          )}
        </div>
      )}

      {Object.keys(scanProgress).length > 0 && (
        <div className="max-h-64 overflow-y-auto space-y-1 border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-2">
          {scannableHosts.map(host => {
            const s = scanProgress[host.oficial_slug];
            if (!s) return null;
            return (
              <div key={host.oficial_slug} className="flex items-center gap-2 py-1.5 px-2 rounded text-xs">
                {s.status === "pending" && <span className="w-2 h-2 rounded-full bg-[var(--text-faint)]" />}
                {s.status === "scanning" && <span className="w-2 h-2 rounded-full bg-[var(--accent)] animate-pulse" />}
                {s.status === "success" && <span className="w-2 h-2 rounded-full bg-emerald-400" />}
                {s.status === "failed" && <span className="w-2 h-2 rounded-full bg-red-400" />}
                <span className="text-[var(--text-primary)] font-medium" style={{ fontFamily: "var(--font-mono)" }}>{host.nickname}</span>
                {s.status === "scanning" && <span className="text-[var(--accent)] ml-auto">{t("host.scanning")}</span>}
                {s.status === "success" && <span className="text-emerald-400 ml-auto">OK</span>}
                {s.error && <span className="text-red-400 ml-auto truncate max-w-[200px]" title={s.error}>{s.error}</span>}
              </div>
            );
          })}
        </div>
      )}

      <div className="flex gap-2 justify-end">
        {scanning ? (
          <Button variant="danger" size="sm" onClick={onStop}>{t("host.stopScan")}</Button>
        ) : (
          <>
            <Button variant="secondary" size="sm" onClick={onClose}>{t("common.cancel")}</Button>
            <Button size="sm" onClick={onStart}>
              <svg className="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
              </svg>
              {scannedCount > 0 ? t("host.rescan") : t("host.startScan")}
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
