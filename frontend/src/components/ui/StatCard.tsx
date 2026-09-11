import type { CSSProperties } from "react";
import Card, { type CardAccent } from "./Card";

interface StatCardProps {
  label: string;
  value: string | number;
  icon: string;
  /** Card accent: `accent`, `cyan`, `rose` or a semantic name (`success`, `warning`, `danger`, `info`). */
  color: CardAccent;
  className?: string;
  style?: CSSProperties;
}

// KPI tile = Card with the `tint` decorator; every colour below reads --card-accent.
export default function StatCard({ label, value, icon, color, className = "", style }: StatCardProps) {
  return (
    <Card accent={color} decorator="tint" padding="sm" className={className} style={style}>
      <svg
        className="absolute right-2.5 top-2.5 w-8 h-8 text-[var(--card-accent)] opacity-[0.08]"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={1}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d={icon} />
      </svg>
      <div className="relative">
        <p className="text-2xs text-[var(--text-muted)] font-medium">{label}</p>
        <p className="text-2xl font-bold mt-0.5 text-[var(--card-accent)] font-display">{value}</p>
      </div>
    </Card>
  );
}
