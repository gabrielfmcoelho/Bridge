"use client";

import type { ReactNode } from "react";

export interface ViewModeOption<T extends string> {
  value: T;
  label: string;
  icon: ReactNode;
}

interface Props<T extends string> {
  value: T;
  onChange: (next: T) => void;
  options: Array<ViewModeOption<T>>;
  ariaLabel?: string;
}

/**
 * Segmented control for view-mode toggling. Mirrors the styling vocabulary of
 * the existing ViewToggle but accepts richer icons + labels.
 */
export default function ViewModeToggle<T extends string>({ value, onChange, options, ariaLabel }: Props<T>) {
  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className="inline-flex h-8 p-0.5 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] gap-0.5"
    >
      {options.map(opt => {
        const active = value === opt.value;
        return (
          <button
            key={opt.value}
            role="tab"
            aria-selected={active}
            onClick={() => onChange(opt.value)}
            className={`inline-flex items-center gap-1.5 px-2.5 rounded-[calc(var(--radius-md)-2px)] text-[11px] font-medium transition-all ${
              active
                ? "bg-[var(--accent-muted)] text-[var(--accent)] shadow-[inset_0_0_0_1px_var(--accent-muted-strong,transparent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            }`}
            title={opt.label}
          >
            <span className={`shrink-0 ${active ? "" : "opacity-70"}`}>{opt.icon}</span>
            <span className="hidden sm:inline">{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
