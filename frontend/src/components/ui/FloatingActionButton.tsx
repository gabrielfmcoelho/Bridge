"use client";

import { useState, useEffect, useCallback } from "react";
import { useAppearance } from "@/contexts/AppearanceContext";

export interface FABAction {
  label: string;
  icon: string;
  onClick: () => void;
  color?: string;
}

interface FloatingActionButtonProps {
  actions: FABAction[];
}

export default function FloatingActionButton({ actions }: FloatingActionButtonProps) {
  const [open, setOpen] = useState(false);
  const { appColor } = useAppearance();

  // Close on scroll
  const close = useCallback(() => setOpen(false), []);
  useEffect(() => {
    if (!open) return;
    window.addEventListener("scroll", close, { passive: true, capture: true });
    return () => window.removeEventListener("scroll", close, true);
  }, [open, close]);

  if (actions.length === 0) return null;

  return (
    <div className="md:hidden">
      {/* Backdrop */}
      {open && (
        <div className="fixed inset-0 z-[38] bg-black/30" onClick={() => setOpen(false)} />
      )}

      {/* Action bubbles */}
      <div className={`fixed bottom-36 right-4 z-[45] flex flex-col items-end gap-2.5 transition-all duration-200 ${open ? "opacity-100 translate-y-0" : "opacity-0 translate-y-4 pointer-events-none"}`}>
        {actions.map((action, i) => (
          <button
            key={i}
            onClick={() => { action.onClick(); setOpen(false); }}
            className="flex items-center gap-2 pl-3 pr-2 py-1.5 rounded-full shadow-lg border border-[var(--border-default)] bg-[var(--bg-surface)] text-[var(--text-primary)] text-xs font-medium animate-slide-up"
            style={{ animationDelay: `${i * 40}ms` }}
          >
            <span>{action.label}</span>
            <span
              className="w-8 h-8 rounded-full flex items-center justify-center shrink-0"
              style={{ backgroundColor: action.color || appColor, color: "#fff" }}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d={action.icon} />
              </svg>
            </span>
          </button>
        ))}
      </div>

      {/* Main FAB */}
      <button
        onClick={() => setOpen(!open)}
        className="fixed bottom-20 right-4 z-[45] w-14 h-14 rounded-full flex items-center justify-center shadow-xl transition-all duration-200"
        style={{
          backgroundColor: appColor,
          boxShadow: `0 6px 20px ${appColor}50`,
        }}
      >
        {open ? (
          <svg className="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        ) : (
          <svg className="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <circle cx="12" cy="5" r="1" fill="currentColor" />
            <circle cx="12" cy="12" r="1" fill="currentColor" />
            <circle cx="12" cy="19" r="1" fill="currentColor" />
          </svg>
        )}
      </button>
    </div>
  );
}
