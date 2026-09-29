import Link from "next/link";
import type { ReactNode } from "react";

/**
 * Flush row lists inside a `SectionCard body="flush"` — the shape detail tabs
 * use for an asset's collections (alerts, chamados, logs, relations):
 * dot or icon · title (+ muted meta line) · trailing meta on the right.
 */
const rowBase = "flex w-full items-center gap-3 px-5 py-2.5 text-left";
const rowHover = "hover:bg-[var(--bg-elevated)] transition-colors duration-100";

export function RowList({ children }: { children: ReactNode }) {
  return <div className="divide-y divide-[var(--border-subtle)]">{children}</div>;
}

/** Muted sub-header over a group of rows (sentence case, never uppercase). */
export function RowGroupTitle({ children }: { children: ReactNode }) {
  return <h4 className="px-5 pt-4 pb-1 text-xs font-medium text-[var(--text-muted)]">{children}</h4>;
}

export function RowGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <RowGroupTitle>{title}</RowGroupTitle>
      <RowList>{children}</RowList>
    </div>
  );
}

/** One row: a link, a button, or static — decided by href/onClick. */
export function ListRow({ href, onClick, className = "", children }: {
  href?: string;
  onClick?: () => void;
  className?: string;
  children: ReactNode;
}) {
  if (href) return <Link href={href} className={`${rowBase} ${rowHover} ${className}`}>{children}</Link>;
  if (onClick) return <button type="button" onClick={onClick} className={`${rowBase} ${rowHover} ${className}`}>{children}</button>;
  return <div className={`${rowBase} ${className}`}>{children}</div>;
}

/** Title with an optional second muted line (mono id, owner, source…). */
export function RowText({ title, meta, mono, muted }: { title: ReactNode; meta?: ReactNode; mono?: boolean; muted?: boolean }) {
  return (
    <span className="min-w-0 flex-1">
      <span className={`block text-sm truncate ${mono ? "font-mono" : ""} ${muted ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]"}`}>{title}</span>
      {meta && <span className="flex gap-3 text-2xs text-[var(--text-muted)] truncate">{meta}</span>}
    </span>
  );
}
