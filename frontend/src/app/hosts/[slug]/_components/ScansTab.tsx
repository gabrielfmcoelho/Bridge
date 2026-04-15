import VMInfoDisplay from "./VMInfoDisplay";
import EmptyState from "@/components/ui/EmptyState";

interface ScansTabProps {
  lastScan?: { data: string; scanned_at: string };
  formatDateTime: (date: string) => string;
  locale: string;
  t: (key: string) => string;
}

export default function ScansTab({ lastScan, formatDateTime, locale, t }: ScansTabProps) {
  return (
    <div className="space-y-5 animate-fade-in">
      {lastScan?.data && (() => {
        try {
          const scanData = JSON.parse(lastScan.data);
          return (
            <>
              <p className="text-xs text-[var(--text-faint)]">{t("scan.lastScan")} — <span className="text-[var(--text-muted)]">{formatDateTime(lastScan.scanned_at)}</span></p>
              <VMInfoDisplay info={scanData} locale={locale} />
            </>
          );
        } catch { return null; }
      })()}
      {!lastScan?.data && (
        <EmptyState
          icon="search"
          title={t("scan.noData")}
          description={t("scan.noDataDesc")}
        />
      )}
    </div>
  );
}
