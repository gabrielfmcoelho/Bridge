"use client";

import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useEffect, type ReactNode } from "react";

interface ModalProps {
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

export default function Modal({ open, onClose, title, subHeader, footer, children }: ModalProps) {
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

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-end md:items-center justify-center animate-fade-in"
      onClick={onClose}
    >
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" />
      <div
        className="relative glass border border-[var(--border-default)] md:rounded-[var(--radius-xl)] rounded-t-[var(--radius-xl)] max-w-2xl w-full md:mx-4 max-h-[95vh] md:max-h-[90vh] flex flex-col overflow-hidden shadow-[var(--shadow-lg)] animate-scale-in md:animate-scale-in"
        onClick={(e) => e.stopPropagation()}
      >
        {title && (
          <div className="border-b border-[var(--border-subtle)] shrink-0 glass">
            <div className="flex items-center justify-between p-4 md:p-5">
              <h2 className="text-lg font-semibold" style={{ fontFamily: "var(--font-display)" }}>{title}</h2>
              <button
                onClick={onClose}
                className="w-8 h-8 flex items-center justify-center rounded-[var(--radius-md)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-overlay)] transition-all duration-150"
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
