/** A labelled meter for an inventory card's domain block (host CPU/RAM/disk,
 *  certificate validity): label and reading on top, bar, caption below.
 *  `pct` null = no reading — the track shows empty and the reading "–". */
export default function CardMeter({ label, reading, pct, tone, caption }: {
  label: string;
  reading?: string;
  pct: number | null;
  /** Token colour of the reading and bar: "success" | "warning" | "danger". */
  tone: "success" | "warning" | "danger";
  caption?: string;
}) {
  const text = { success: "text-[var(--success)]", warning: "text-[var(--warning)]", danger: "text-[var(--danger)]" }[tone];
  const bar = { success: "bg-[var(--success)]", warning: "bg-[var(--warning)]", danger: "bg-[var(--danger)]" }[tone];
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-[var(--text-muted)]">{label}</span>
        <span className={`text-xs font-semibold font-mono ${pct !== null && reading ? text : "text-[var(--text-muted)]"}`}>{pct !== null && reading ? reading : "–"}</span>
      </div>
      <div className="h-1.5 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
        {pct !== null && <div className={`h-full rounded-full ${bar} transition-[width]`} style={{ width: `${Math.max(0, Math.min(pct, 100))}%` }} />}
      </div>
      <p className="text-xs text-[var(--text-muted)] mt-0.5 text-right font-mono truncate">{caption || "–"}</p>
    </div>
  );
}
