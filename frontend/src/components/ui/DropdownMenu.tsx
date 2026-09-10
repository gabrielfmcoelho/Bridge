"use client";

import type { ReactNode } from "react";
import * as Popover from "@radix-ui/react-popover";

// Small action menu on Radix Popover, so outside-click, Escape and focus
// handling are not re-implemented per site. `data-portal-dropdown` keeps a
// hosting Drawer open while the menu is up.
export default function DropdownMenu({
  trigger,
  children,
  align = "end",
  className = "",
}: {
  trigger: ReactNode;
  children: ReactNode;
  align?: "start" | "end";
  className?: string;
}) {
  return (
    <Popover.Root>
      <Popover.Trigger asChild>{trigger}</Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          data-portal-dropdown
          side="bottom"
          sideOffset={4}
          align={align}
          className={`z-[100] min-w-40 bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-[var(--shadow-lg)] overflow-hidden animate-fade-in ${className}`}
        >
          {children}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

export function DropdownMenuItem({
  children,
  active = false,
  danger = false,
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { active?: boolean; danger?: boolean }) {
  const tone = danger
    ? "text-[var(--danger)] hover:bg-[var(--danger)]/10"
    : active
      ? "text-[var(--accent)]"
      : "text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]";
  return (
    <Popover.Close asChild>
      <button type="button" className={`w-full text-left px-3 py-1.5 text-sm transition-colors disabled:opacity-40 ${tone} ${className}`} {...props}>
        {children}
      </button>
    </Popover.Close>
  );
}
