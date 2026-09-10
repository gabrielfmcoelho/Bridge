import type { ReactNode } from "react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import StatusDot from "@/components/ui/StatusDot";

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
        } font-display`}
      >
        <span className="flex items-center gap-2">
          {title}
          {active && !open && (
            <StatusDot size="xs" color="accent" />
          )}
        </span>
        <Icon path={ICON_PATHS.chevronDown} className={`w-4 h-4 text-[var(--text-faint)] transition-transform duration-200 ${open ? "rotate-180" : ""}`} />
      </button>
      {open && <div className="pb-4 space-y-3 animate-fade-in">{children}</div>}
    </div>
  );
}
