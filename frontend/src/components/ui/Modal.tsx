"use client";

import { useEffect, type ReactNode } from "react";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  subHeader?: ReactNode;
  children: ReactNode;
}

export default function Modal({ open, onClose, title, subHeader, children }: ModalProps) {
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
        className="relative glass border border-[var(--border-default)] md:rounded-[var(--radius-xl)] rounded-t-[var(--radius-xl)] max-w-2xl w-full md:mx-4 max-h-[95vh] md:max-h-[90vh] overflow-y-auto shadow-[var(--shadow-lg)] animate-scale-in md:animate-scale-in"
        onClick={(e) => e.stopPropagation()}
      >
        {title && (
          <div className="border-b border-[var(--border-subtle)] sticky top-0 glass z-10">
            <div className="flex items-center justify-between p-4 md:p-5">
              <h2 className="text-lg font-semibold" style={{ fontFamily: "var(--font-display)" }}>{title}</h2>
              <button
                onClick={onClose}
                className="w-8 h-8 flex items-center justify-center rounded-[var(--radius-md)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-overlay)] transition-all duration-150"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            {subHeader && <div className="px-4 md:px-5 pb-3">{subHeader}</div>}
          </div>
        )}
        <div className="p-4 md:p-5">{children}</div>
      </div>
    </div>
  );
}
