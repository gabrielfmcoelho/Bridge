"use client";

import type { ReactNode } from "react";
import Card, { type CardAccent } from "./Card";
import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface SectionCardProps {
  title: string;
  /** Leading icon path (ICON_PATHS), muted. */
  icon?: string;
  /** Mono count after the title (result totals). */
  count?: number;
  /** One muted line under the title: what the section shows or is for. */
  description?: string;
  /** Minimal tools on the right of the header: IconButton, ghost Button, ToolbarSelect, Toggle (h-8). */
  controls?: ReactNode;
  /** Bar under the body: save bar, pagination, comment box, "ver todos". */
  footer?: ReactNode;
  /** "flush": tables and row lists run edge to edge (rows bring their own px-5). */
  body?: "padded" | "flush";
  /** "plain": the same header without card chrome, for a body that is itself cards. */
  variant?: "card" | "plain";
  /** Left stripe, as on inventory cards. */
  accent?: CardAccent;
  /** Section with nothing to show: the body becomes this one muted line (empty-state level 2). */
  empty?: string;
  /** Optional action beside the empty line (e.g. "Executar scan"). */
  emptyAction?: ReactNode;
  /** Folds under its header (native <details>). */
  collapsible?: boolean;
  defaultOpen?: boolean;
  as?: "h2" | "h3";
  id?: string;
  className?: string;
  children?: ReactNode;
}

/**
 * The one shell for a block of content. Fixed anatomy, top to bottom:
 *   header  — title (+ icon, count) · controls on the right
 *             description (one muted line)
 *   ─────── hairline
 *   body    — table, list, chart, key-value info or form (padded or flush)
 *   footer  — optional bar
 * Replaces SectionHeading-above-a-Card and hand-rolled <h2>/<h3> card rows.
 */
export default function SectionCard({
  title, icon, count, description, controls, footer,
  body = "padded", variant = "card", accent, empty, emptyAction, collapsible = false, defaultOpen = true,
  as: Tag = "h2", id, className = "", children,
}: SectionCardProps) {
  const plain = variant === "plain";
  const titleRow = (
    <div className="flex items-center justify-between gap-3">
      <div className="flex items-center gap-2 min-w-0">
        {collapsible && <Icon path={ICON_PATHS.chevronRight} className="w-3.5 h-3.5 text-[var(--text-muted)] transition-transform duration-150 group-open/section:rotate-90" />}
        {icon && <Icon path={icon} className="w-4 h-4 text-[var(--text-muted)]" />}
        <Tag id={id} className="text-heading-xs font-semibold font-display text-[var(--text-primary)] truncate">{title}</Tag>
        {count !== undefined && <span className="text-xs font-mono tabular-nums text-[var(--text-muted)]">{count}</span>}
      </div>
      {controls && <div className="flex items-center gap-1.5 shrink-0">{controls}</div>}
    </div>
  );
  const header = (
    <div className={plain ? "pb-3" : "px-5 pt-4 pb-3"}>
      {titleRow}
      {description && <p className="mt-1 text-xs leading-relaxed text-[var(--text-muted)]">{description}</p>}
    </div>
  );
  const content = (
    <>
      {empty ? (
        <div className={`flex flex-wrap items-center justify-between gap-3 ${plain ? "pt-4" : "px-5 py-4"}`}>
          <p className="text-sm text-[var(--text-muted)]">{empty}</p>
          {emptyAction}
        </div>
      ) : (
        <div className={plain ? "pt-4" : body === "padded" ? "p-5" : ""}>{children}</div>
      )}
      {footer && <div className={`border-t border-[var(--border-subtle)] ${plain ? "pt-3 mt-4" : "px-5 py-3"}`}>{footer}</div>}
    </>
  );
  const rule = <div className="border-t border-[var(--border-subtle)]" />;

  const inner = collapsible ? (
    <details open={defaultOpen} className="group/section">
      <summary className="list-none cursor-pointer [&::-webkit-details-marker]:hidden">{header}</summary>
      {rule}
      {content}
    </details>
  ) : (
    <>
      {header}
      {rule}
      {content}
    </>
  );

  if (plain) return <section className={className} aria-labelledby={id}>{inner}</section>;
  return (
    <Card hover={false} padding="none" accent={accent} className={`overflow-hidden ${className}`}>
      <section aria-labelledby={id}>{inner}</section>
    </Card>
  );
}
