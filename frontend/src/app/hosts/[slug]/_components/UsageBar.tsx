import { pctTextColor, pctBarColor } from "@/lib/utils";

export function UsageBar({ label, total, used, percent }: { label: string; total: string; used: string; percent: string }) {
  const pctNum = parseInt(percent) || 0;
  const color = pctBarColor(pctNum);
  const textColor = pctNum >= 80 ? "text-red-400" : pctNum >= 50 ? "text-amber-400" : "text-[var(--text-secondary)]";
  return (
    <div className="space-y-1.5">
      <div className="flex justify-between text-sm">
        <span className="text-[var(--text-muted)] font-medium">{label}</span>
        <span className={textColor} style={{ fontFamily: "var(--font-mono)" }}>{used} / {total} ({percent})</span>
      </div>
      <div className="w-full h-2 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
        <div className={`h-full rounded-full ${color} transition-all`} style={{ width: `${Math.min(pctNum, 100)}%` }} />
      </div>
    </div>
  );
}

export function ResourceCard({ label, value, icon }: { label: string; value: string; icon: string }) {
  return (
    <div className="bg-[var(--bg-elevated)] rounded-[var(--radius-md)] p-3 border border-[var(--border-subtle)]">
      <div className="flex items-center gap-2 mb-1.5">
        <svg className="w-4 h-4 text-[var(--text-faint)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d={icon} />
        </svg>
        <span className="text-xs text-[var(--text-muted)]">{label}</span>
      </div>
      <p className="text-sm font-semibold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{value || "-"}</p>
    </div>
  );
}
