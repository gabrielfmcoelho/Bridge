import type { ReactNode } from "react";
import Icon from "./Icon";

// The three heading recipes the app actually uses, in one place:
//   label   - small heading over a group (listing labels, card sections)
//   section - sub-heading inside a page or tab
//   rule    - heading on a hairline, optional count/icon/hint (catalog results, form steps)
//
// None of them is uppercase. Uppercase letterspaced 10-12px text is the
// slowest text on a page to read (no word shape, tracking defeats saccades),
// and the app was spending it on its least important strings — at which point
// the hierarchy it was meant to create is gone. Uppercase is reserved for
// table column headers, where it is a real convention and the row below it is
// data. Differentiate by weight and colour instead.
const variants = {
  label: "text-xs font-semibold text-[var(--text-muted)]",
  section: "text-sm font-semibold text-[var(--text-secondary)] font-display",
  rule: "text-xs font-semibold text-[var(--text-secondary)]",
};

interface SectionHeadingProps {
  children: ReactNode;
  variant?: keyof typeof variants;
  as?: "h2" | "h3";
  /** Mono count next to the label (result totals). */
  count?: number;
  /** SVG path; drawn faint before the label. */
  icon?: string;
  /** One-line explainer under the heading. */
  hint?: string;
  actions?: ReactNode;
  className?: string;
}

export default function SectionHeading({
  children,
  variant = "label",
  as: Tag = "h2",
  count,
  icon,
  hint,
  actions,
  className = "",
}: SectionHeadingProps) {
  const row = (
    <div className="flex items-center justify-between gap-2">
      <div className="flex items-baseline gap-2 min-w-0">
        {icon && <Icon path={icon} className="h-3.5 w-3.5 text-[var(--text-muted)] self-center shrink-0" />}
        <Tag className={`${variants[variant]} flex items-center gap-1.5`}>{children}</Tag>
        {count !== undefined && (
          <span className="text-xs text-[var(--text-muted)] font-mono tabular-nums">{count}</span>
        )}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </div>
  );

  if (variant === "rule") {
    return (
      <div className={`border-b border-[var(--border-subtle)] pb-2.5 mb-4 ${hint ? "space-y-1" : ""} ${className}`}>
        {row}
        {hint && <p className="text-xs leading-relaxed text-[var(--text-muted)]">{hint}</p>}
      </div>
    );
  }
  return <div className={`mb-3 ${className}`}>{row}</div>;
}
