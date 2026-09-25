"use client";

import type { ReactNode } from "react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export type T = (k: string, vars?: Record<string, string>) => string;

/** One block inside a group card; blocks after the first get a hairline above. */
export function SubBlock({ title, aside, children }: { title?: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <div className="[&+&]:pt-4 [&+&]:mt-4 [&+&]:border-t [&+&]:border-[var(--border-subtle)]">
      {title && (
        <div className="flex items-center gap-2 mb-2">
          <span className="text-xs font-medium text-[var(--text-secondary)]">{title}</span>
          {aside && <span className="ml-auto text-2xs text-[var(--text-faint)] font-mono">{aside}</span>}
        </div>
      )}
      {children}
    </div>
  );
}

/** Container for flush row lists (users, processes, containers). */
export function Rows({ children }: { children: ReactNode }) {
  return <div className="divide-y divide-[var(--border-subtle)]">{children}</div>;
}

/**
 * One compact row. With `children` it expands (native <details>) to show them;
 * without, it is a plain row — so every row keeps the same left edge.
 */
export function Row({ summary, children, muted }: { summary: ReactNode; children?: ReactNode; muted?: boolean }) {
  const line = "flex items-center gap-3 px-5 py-2.5 min-w-0 text-xs";
  if (!children) {
    return (
      <div className={`${line} ${muted ? "opacity-60" : ""}`}>
        <span className="w-3.5 shrink-0" aria-hidden="true" />
        {summary}
      </div>
    );
  }
  return (
    <details className={`group/row ${muted ? "opacity-60 open:opacity-100" : ""}`}>
      <summary className={`${line} list-none cursor-pointer hover:bg-[var(--bg-elevated)]/50 [&::-webkit-details-marker]:hidden`}>
        <Icon path={ICON_PATHS.chevronRight} className="w-3.5 h-3.5 shrink-0 text-[var(--text-muted)] transition-transform duration-150 group-open/row:rotate-90" />
        {summary}
      </summary>
      <div className="px-5 pb-3 pl-[2.875rem] space-y-2 text-xs">{children}</div>
    </details>
  );
}

/** Label/value pair inside an expanded row. */
export function Detail({ label, children, mono = true }: { label: string; children: ReactNode; mono?: boolean }) {
  return (
    <div className="flex gap-2 min-w-0">
      <span className="w-24 shrink-0 text-[var(--text-muted)]">{label}</span>
      <span className={`min-w-0 break-all text-[var(--text-secondary)] ${mono ? "font-mono" : ""}`}>{children}</span>
    </div>
  );
}

/** Colour for a CPU/RAM percentage: danger ≥ 80, warning ≥ 50. */
export function pctColor(pct: number): string {
  return pct >= 80 ? "text-[var(--danger)]" : pct >= 50 ? "text-[var(--warning)]" : "text-[var(--text-secondary)]";
}
