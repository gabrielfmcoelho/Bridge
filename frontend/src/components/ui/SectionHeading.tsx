import type { ReactNode } from "react";
import Icon from "./Icon";

// The three heading recipes the app actually uses, in one place:
//   label   - small uppercase eyebrow over a group (listing labels, card sections)
//   section - sentence-case sub-heading inside a page or tab
//   rule    - eyebrow on a hairline, optional count/icon/hint (catalog results, form steps)
const variants = {
  label: "text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider",
  section: "text-sm font-semibold text-[var(--text-secondary)] font-display",
  rule: "text-xs font-semibold text-[var(--text-secondary)] uppercase tracking-wider",
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
        {icon && <Icon path={icon} className="h-3.5 w-3.5 text-[var(--text-faint)] self-center shrink-0" />}
        <Tag className={`${variants[variant]} flex items-center gap-1.5`}>{children}</Tag>
        {count !== undefined && (
          <span className="text-xs text-[var(--text-faint)] font-mono tabular-nums">{count}</span>
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
