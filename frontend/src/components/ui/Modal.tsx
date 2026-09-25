"use client";

import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useEffect, useId, type ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";

/** Widths after the ADS modal steps (400/600/800/968px); md keeps the
 *  app's long-standing 42rem so existing dialogs don't change. */
export const MODAL_SIZES = {
  sm: "max-w-[25rem]",
  md: "max-w-2xl",
  lg: "max-w-[50rem]",
  xl: "max-w-[60.5rem]",
} as const;
export type ModalSize = keyof typeof MODAL_SIZES;

interface ModalProps {
  size?: ModalSize;
  open: boolean;
  onClose: () => void;
  title?: string;
  subHeader?: ReactNode;
  /** Pinned action bar. Mirrors Drawer's footer slot so a ResponsiveModal keeps
   *  its actions in the same place on both breakpoints — previously the desktop
   *  half rendered them inline and they scrolled away with the content. */
  footer?: ReactNode;
  children: ReactNode;
}

export default function Modal({ open, onClose, title, subHeader, footer, children, size = "md" }: ModalProps) {
  const { t } = useLocale();
  const titleId = useId();

  useEffect(() => {
    if (open) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [open]);

  // Escape closes the dialog. The vaul half of ResponsiveModal has always had
  // this; the desktop half did not, so the same modal behaved differently
  // depending on viewport width.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-end md:items-center justify-center animate-fade-in"
      onClick={onClose}
    >
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        aria-label={title ? undefined : t("common.dialog")}
        className={`relative glass border border-[var(--border-default)] md:rounded-[var(--radius-xl)] rounded-t-[var(--radius-xl)] ${MODAL_SIZES[size]} w-full md:mx-4 max-h-[95vh] md:max-h-[90vh] flex flex-col overflow-hidden shadow-[var(--shadow-lg)] animate-scale-in md:animate-scale-in`}
        onClick={(e) => e.stopPropagation()}
      >
        {title && (
          <div className="border-b border-[var(--border-subtle)] shrink-0 glass">
            <div className="flex items-center justify-between p-4 md:p-5">
              <h2 id={titleId} className="text-lg font-semibold font-display">{title}</h2>
              <button
                type="button"
                onClick={onClose}
                aria-label={t("common.close")}
                className="w-8 h-8 flex items-center justify-center rounded-[var(--radius-md)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-overlay)] transition duration-150"
              >
                <Icon path={ICON_PATHS.close} />
              </button>
            </div>
            {subHeader && <div className="px-4 md:px-5 pb-3">{subHeader}</div>}
          </div>
        )}
        <div className="flex-1 overflow-y-auto p-4 md:p-5">{children}</div>
        {footer && (
          <div className="shrink-0 border-t border-[var(--border-subtle)] p-4 md:p-5 glass">{footer}</div>
        )}
      </div>
    </div>
  );
}
