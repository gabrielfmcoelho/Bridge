"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import IconButton from "./IconButton";
import Icon from "./Icon";
import Heading from "./Heading";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

interface PanelProps {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
}

/**
 * ADS "Panel": a non-modal side area for contextual detail next to the page.
 * No backdrop, no focus trap, the page stays usable. Escape closes it while
 * focus is inside; opening moves focus to the close button and closing
 * returns it to whatever opened it.
 */
export default function Panel({ open, onClose, title, subtitle, footer, children }: PanelProps) {
  const { t } = useLocale();
  const titleId = useId();
  const asideRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const returnTo = useRef<HTMLElement | null>(null);
  // Starts at the mount-time value, so a panel mounted already open (e.g. its
  // owner remounted) does not steal focus from whatever the user is in. Mount
  // it closed and flip `open` to get the slide-in and the focus move.
  const wasOpen = useRef(open);

  useEffect(() => {
    if (open && !wasOpen.current) {
      returnTo.current = document.activeElement as HTMLElement | null;
      closeRef.current?.focus();
    } else if (!open && wasOpen.current) {
      const el = returnTo.current;
      // Only when focus was still in the panel (or dropped to body) — don't
      // yank it back if the user closed by clicking elsewhere on the page.
      const active = document.activeElement;
      const inPanel = !active || active === document.body || asideRef.current?.contains(active);
      if (el?.isConnected && inPanel) el.focus();
      returnTo.current = null;
    }
    wasOpen.current = open;
  }, [open]);

  if (typeof document === "undefined") return null;

  return createPortal(
    <aside
      ref={asideRef}
      role="complementary"
      aria-labelledby={titleId}
      inert={!open}
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.stopPropagation();
          onClose();
        }
      }}
      className={`fixed top-13 bottom-0 right-0 z-45 w-[calc(100vw-1rem)] sm:w-96 flex flex-col bg-[var(--bg-surface)] border-l border-[var(--border-subtle)] shadow-[var(--shadow-lg)] transition duration-200 ${
        open ? "translate-x-0 visible" : "translate-x-full invisible"
      }`}
    >
      <div className="flex items-start gap-2 p-4 border-b border-[var(--border-subtle)] shrink-0">
        <div className="flex-1 min-w-0">
          <Heading as="h2" size="xs" id={titleId} className="break-words">
            {title}
          </Heading>
          {subtitle && <div className="mt-0.5 text-xs text-[var(--text-muted)]">{subtitle}</div>}
        </div>
        <IconButton ref={closeRef} label={t("common.close")} onClick={onClose} className="-mr-1 -mt-1">
          <Icon path={ICON_PATHS.close} />
        </IconButton>
      </div>
      <div className="flex-1 overflow-y-auto p-4">{children}</div>
      {footer && <div className="shrink-0 border-t border-[var(--border-subtle)] p-4">{footer}</div>}
    </aside>,
    document.body,
  );
}
