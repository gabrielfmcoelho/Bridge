import type { ButtonHTMLAttributes, ReactNode } from "react";

// Toggle chip. `rounded` is the toolbar/filter-drawer look; `pill` is the
// transparent, fully-round filter chip the catalog and atlas use.
const shapes = {
  rounded: {
    base: "rounded-[var(--radius-md)]",
    active: "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20",
    idle: "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:bg-[var(--bg-overlay)] hover:text-[var(--text-secondary)]",
  },
  pill: {
    base: "rounded-full",
    active: "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/30",
    idle: "bg-transparent text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] hover:border-[var(--border-strong)]",
  },
};

const sizes = {
  sm: "px-2.5 py-1 text-xs",
  md: "px-3 py-1.5 text-xs",
  lg: "px-4 py-2 text-sm",
};

interface PillButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "onClick"> {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
  shape?: keyof typeof shapes;
  size?: keyof typeof sizes;
  /** Mono trailing count (result totals). */
  count?: number;
  /** Leading dot or icon. */
  lead?: ReactNode;
}

export default function PillButton({
  active,
  onClick,
  children,
  shape = "rounded",
  size = "md",
  count,
  lead,
  className = "",
  ...props
}: PillButtonProps) {
  const s = shapes[shape];
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 border font-medium whitespace-nowrap transition duration-150 active:scale-[0.97] ${s.base} ${sizes[size]} ${active ? s.active : s.idle} ${className}`}
      {...props}
    >
      {lead}
      {children}
      {count !== undefined && (
        <span className={`font-mono tabular-nums text-[0.85em] ${active ? "opacity-80" : "text-[var(--text-faint)]"}`}>{count}</span>
      )}
    </button>
  );
}
