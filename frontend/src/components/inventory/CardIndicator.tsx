import Icon from "@/components/ui/Icon";

// Full literals on purpose: Tailwind only generates classes it can read from
// source. Hue names are what the entity cards pass; semantic names also work.
const colors: Record<string, string> = {
  success: "text-[var(--success)]",
  warning: "text-[var(--warning)]",
  danger: "text-[var(--danger)]",
  info: "text-[var(--info)]",
  cyan: "text-[var(--cyan)]",
  purple: "text-[var(--accent)]",
  rose: "text-[var(--rose)]",
  emerald: "text-[var(--success)]",
  amber: "text-[var(--warning)]",
  orange: "text-[var(--warning)]",
  red: "text-[var(--danger)]",
  sky: "text-[var(--info)]",
  violet: "text-[var(--accent)]",
};

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
  /** Flag, not a count: render the icon either way (coloured when on, faint
   *  when off) and never print a number. */
  hideCount?: boolean;
}) {
  const active = (count ?? 0) > 0;
  // A row of faint zeroes is the loudest thing on an inventory card and says
  // nothing. Counts disappear at zero; flags keep their off state.
  if (!active && !hideCount) return null;
  const colorClass = active ? colors[color] ?? "text-[var(--accent)]" : "text-[var(--text-faint)]";

  return (
    <div className="flex items-center gap-1" title={title}>
      <Icon path={icon} className={`w-3.5 h-3.5 ${colorClass}`} />
      {active && !hideCount && <span className={`text-xs font-semibold font-mono ${colorClass}`}>{count}</span>}
    </div>
  );
}
