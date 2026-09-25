import type { CSSProperties } from "react";
import Card, { type CardAccent } from "./Card";
import Icon from "./Icon";

interface StatCardProps {
  label: string;
  value: string | number;
  icon: string;
  /** Card accent: `accent`, `cyan`, `rose`, a semantic name (`success`, `warning`, `danger`, `info`) or a CSS colour. */
  color: CardAccent;
  /** One line of context under the value ("84 ativos · 12 em manutenção"). */
  hint?: string;
  /** Makes the tile a button (e.g. apply the filter it counts). */
  onClick?: () => void;
  /** The tile's filter is the one applied. */
  active?: boolean;
  className?: string;
  style?: CSSProperties;
}

// KPI tile. One recipe in both themes: the accent is the left stripe every
// other card uses, plus the icon; label, value and hint stay on the text
// tokens, so the number reads the same on dark and light.
export default function StatCard({ label, value, icon, color, hint, onClick, active, className = "", style }: StatCardProps) {
  return (
    <Card
      accent={color}
      padding="sm"
      hover={!!onClick}
      selected={active}
      as={onClick ? "button" : "div"}
      onClick={onClick}
      className={className}
      style={style}
    >
      <div className="flex items-start justify-between gap-2">
        <p className="text-xs text-[var(--text-muted)] font-medium truncate">{label}</p>
        <Icon path={icon} className="w-4 h-4 shrink-0 text-[var(--card-accent)]" />
      </div>
      <p className="text-2xl font-bold mt-1 text-[var(--text-primary)] font-display tabular-nums">{value}</p>
      {hint && <p className="text-xs text-[var(--text-muted)] mt-0.5 truncate">{hint}</p>}
    </Card>
  );
}
