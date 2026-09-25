"use client";

import type { ReactNode } from "react";
import * as Popover from "@radix-ui/react-popover";
import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

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

/** Titled group of items. More than one group in a menu → title each (ADS). */
export function DropdownMenuGroup({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <div role="group" aria-label={title} className="py-1 [&+&]:border-t [&+&]:border-[var(--border-subtle)]">
      {title && <p className="px-3 pt-1.5 pb-1 text-xs font-semibold text-[var(--text-muted)]">{title}</p>}
      {children}
    </div>
  );
}

// Shared row: icon slot, label (+ optional second line), trailing slot.
function ItemBody({ elemBefore, elemAfter, description, children }: { elemBefore?: ReactNode; elemAfter?: ReactNode; description?: ReactNode; children: ReactNode }) {
  return (
    <span className="flex items-center gap-2">
      {elemBefore && <span className="flex w-4 shrink-0 items-center justify-center">{elemBefore}</span>}
      <span className="min-w-0 flex-1">
        <span className="block truncate">{children}</span>
        {description && <span className="block text-xs text-[var(--text-muted)] truncate">{description}</span>}
      </span>
      {elemAfter && <span className="shrink-0 text-xs text-[var(--text-muted)]">{elemAfter}</span>}
    </span>
  );
}

type ItemSlots = { elemBefore?: ReactNode; elemAfter?: ReactNode; description?: ReactNode };

const itemBase = "w-full text-left px-3 py-1.5 text-sm transition-colors disabled:opacity-40";
const toneIdle = "text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]";

/** Selects one value; closes the menu. Marked with a check when chosen. */
export function DropdownMenuRadioItem({ checked, onSelect, children, ...slots }: ItemSlots & { checked: boolean; onSelect: () => void; children: ReactNode }) {
  return (
    <Popover.Close asChild>
      <button
        type="button"
        role="menuitemradio"
        aria-checked={checked}
        onClick={onSelect}
        className={`${itemBase} ${checked ? "text-[var(--accent)]" : toneIdle}`}
      >
        <ItemBody {...slots} elemAfter={checked ? <Icon path={ICON_PATHS.check} className="w-3.5 h-3.5 text-[var(--accent)]" /> : slots.elemAfter}>
          {children}
        </ItemBody>
      </button>
    </Popover.Close>
  );
}

/** Toggles; the menu stays open so several can be ticked in one go. */
export function DropdownMenuCheckboxItem({ checked, onCheckedChange, children, ...slots }: ItemSlots & { checked: boolean; onCheckedChange: (checked: boolean) => void; children: ReactNode }) {
  return (
    <button
      type="button"
      role="menuitemcheckbox"
      aria-checked={checked}
      onClick={() => onCheckedChange(!checked)}
      className={`${itemBase} ${toneIdle}`}
    >
      <ItemBody
        {...slots}
        elemBefore={
          <span className={`flex h-4 w-4 items-center justify-center rounded-[var(--radius-sm)] border ${checked ? "border-[var(--accent)] bg-[var(--accent)] text-white" : "border-[var(--border-strong)]"}`}>
            {checked && <Icon path={ICON_PATHS.check} className="w-3 h-3" />}
          </span>
        }
      >
        {children}
      </ItemBody>
    </button>
  );
}

export function DropdownMenuItem({
  children,
  active = false,
  danger = false,
  className = "",
  elemBefore,
  elemAfter,
  description,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & ItemSlots & { active?: boolean; danger?: boolean }) {
  const tone = danger
    ? "text-[var(--danger)] hover:bg-[var(--danger)]/10"
    : active
      ? "text-[var(--accent)]"
      : "text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]";
  return (
    <Popover.Close asChild>
      <button type="button" className={`${itemBase} ${tone} ${className}`} {...props}>
        {elemBefore || elemAfter || description ? (
          <ItemBody elemBefore={elemBefore} elemAfter={elemAfter} description={description}>{children}</ItemBody>
        ) : (
          children
        )}
      </button>
    </Popover.Close>
  );
}
