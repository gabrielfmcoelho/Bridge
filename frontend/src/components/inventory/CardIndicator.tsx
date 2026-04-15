/** Atomic icon + count indicator for inventory card bottom rows. */
export default function CardIndicator({
  icon,
  count,
  color,
  title,
  hideCount,
}: {
  icon: string;
  count?: number;
  color: string;
  title: string;
  /** Show icon only (colored when active, faint when not) without the numeric count. */
  hideCount?: boolean;
}) {
  const active = (count ?? 0) > 0;
  const colorClass = active ? `text-${color}-400` : "text-[var(--text-faint)]";

  return (
    <div className="flex items-center gap-1" title={title}>
      <svg
        className={`w-3.5 h-3.5 ${colorClass}`}
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d={icon} />
      </svg>
      {active && !hideCount && (
        <span
          className={`text-xs font-semibold text-${color}-400`}
          style={{ fontFamily: "var(--font-mono)" }}
        >
          {count}
        </span>
      )}
    </div>
  );
}
