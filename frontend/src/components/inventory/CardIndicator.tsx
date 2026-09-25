"use client";

import Icon from "@/components/ui/Icon";
import { useLocale } from "@/contexts/LocaleContext";

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
  disabled,
}: {
  icon: string;
  count?: number;
  color: string;
  title: string;
  /** Flag, not a count: coloured when on, faint when off, never a number. */
  hideCount?: boolean;
  /** The data isn't available (e.g. the API doesn't send it yet): dimmed, no number. */
  disabled?: boolean;
}) {
  const { t } = useLocale();
  const active = !disabled && (count ?? 0) > 0;
  // Fixed anatomy: every indicator always renders in its place, so position
  // carries meaning. Zero is a faint 0, unavailable is dimmed.
  const colorClass = active ? colors[color] ?? "text-[var(--accent)]" : "text-[var(--text-faint)]";
  const label = disabled ? `${title}: ${t("common.notAvailable")}` : title;

  return (
    <div className={`flex items-center gap-1 ${disabled ? "opacity-40" : ""}`} title={label} role="img" aria-label={label}>
      <Icon path={icon} className={`w-3.5 h-3.5 ${colorClass}`} />
      {!hideCount && !disabled && <span className={`text-xs font-semibold font-mono ${active ? colorClass : "text-[var(--text-muted)]"}`}>{count ?? 0}</span>}
    </div>
  );
}
