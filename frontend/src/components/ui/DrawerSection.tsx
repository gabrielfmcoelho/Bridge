import type { ReactNode } from "react";

interface DrawerSectionProps {
  title: string;
  open: boolean;
  onToggle: () => void;
  /** Show an accent dot when the section has active values (useful for filters). */
  active?: boolean;
  children: ReactNode;
}

/**
 * Unified collapsible section for drawer content.
 * Uses flat border-b dividers — no card wrapping to avoid cards-in-cards.
 */
export default function DrawerSection({ title, open, onToggle, active, children }: DrawerSectionProps) {
  return (
    <div className="border-b border-[var(--border-subtle)]">
      <button
        type="button"
        onClick={onToggle}
        className={`w-full flex items-center justify-between py-3 text-sm font-medium transition-colors ${
          open ? "text-[var(--text-primary)]" : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
        }`}
        style={{ fontFamily: "var(--font-display)" }}
      >
        <span className="flex items-center gap-2">
          {title}
          {active && !open && (
            <span className="w-1.5 h-1.5 rounded-full bg-[var(--accent)]" />
          )}
        </span>
        <svg
          className={`w-4 h-4 text-[var(--text-faint)] transition-transform duration-200 ${open ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && <div className="pb-4 space-y-3 animate-fade-in">{children}</div>}
    </div>
  );
}
