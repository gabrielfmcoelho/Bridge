"use client";

import { Drawer as VaulDrawer } from "vaul";
import type { ReactNode } from "react";
import { useMediaQuery } from "@/hooks/useMediaQuery";

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  subHeader?: ReactNode;
  headerAction?: ReactNode;
  /** Optional back-arrow button rendered to the left of the title. Wire it
   *  to onClose (or a more nuanced navigation callback) when the drawer
   *  functions as a subview that the user should "back out of". */
  onBack?: () => void;
  children: ReactNode;
  footer?: ReactNode;
  /** Force a specific side. When omitted, auto-switches: right on desktop, bottom on mobile. */
  side?: "bottom" | "right";
  /** Use a wider max-width for the right drawer (default: max-w-2xl). */
  wide?: boolean;
}

export default function Drawer({ open, onClose, title, subHeader, headerAction, onBack, children, footer, side, wide }: DrawerProps) {
  const BackButton = onBack ? (
    <button
      type="button"
      onClick={onBack}
      className="w-7 h-7 -ml-1 flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] transition-colors shrink-0"
      aria-label="Back"
      title="Back"
    >
      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
      </svg>
    </button>
  ) : null;
  const isMobile = useMediaQuery("(max-width: 767px)");
  const resolvedSide = side ?? (isMobile ? "bottom" : "right");

  if (resolvedSide === "right") {
    return (
      <VaulDrawer.Root open={open} onOpenChange={(o) => !o && onClose()} direction="right">
        <VaulDrawer.Portal>
          <VaulDrawer.Overlay className="fixed inset-0 z-50 bg-black/60" />
          <VaulDrawer.Content
            aria-describedby={undefined}
            className={`fixed right-0 top-0 bottom-0 z-50 w-[90vw] ${wide ? "max-w-5xl" : "max-w-2xl"} flex flex-col bg-[var(--bg-surface)] border-l border-[var(--border-subtle)] outline-none`}
            onPointerDownOutside={(e) => {
              if ((e.target as HTMLElement)?.closest?.("[data-portal-dropdown]")) {
                e.preventDefault();
              }
            }}
          >
            {title ? (
              <div className="border-b border-[var(--border-subtle)]">
                <div className="flex items-center gap-2 p-4">
                  {BackButton}
                  <VaulDrawer.Title className="text-lg font-semibold flex-1 min-w-0 truncate" style={{ fontFamily: "var(--font-display)" }}>
                    {title}
                  </VaulDrawer.Title>
                  {headerAction}
                </div>
                {subHeader && <div className="px-4 pb-3">{subHeader}</div>}
              </div>
            ) : (
              <VaulDrawer.Title className="sr-only">Menu</VaulDrawer.Title>
            )}
            <div className="flex-1 overflow-y-auto p-4">{children}</div>
            {footer && (
              <div className="p-4 border-t border-[var(--border-subtle)]">
                {footer}
              </div>
            )}
          </VaulDrawer.Content>
        </VaulDrawer.Portal>
      </VaulDrawer.Root>
    );
  }

  return (
    <VaulDrawer.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <VaulDrawer.Portal>
        <VaulDrawer.Overlay className="fixed inset-0 z-50 bg-black/60" />
        <VaulDrawer.Content
          aria-describedby={undefined}
          className="fixed bottom-0 left-0 right-0 z-50 flex flex-col rounded-t-[var(--radius-xl)] bg-[var(--bg-surface)] border-t border-[var(--border-subtle)] max-h-[92vh] outline-none"
          onPointerDownOutside={(e) => {
            if ((e.target as HTMLElement)?.closest?.("[data-portal-dropdown]")) {
              e.preventDefault();
            }
          }}
        >
          {/* Drag handle */}
          <div className="flex justify-center pt-3 pb-1">
            <div className="w-10 h-1 rounded-full bg-[var(--border-default)]" />
          </div>
          {title ? (
            <div className="border-b border-[var(--border-subtle)]">
              <div className="flex items-center gap-2 px-4 pb-3">
                {BackButton}
                <VaulDrawer.Title className="text-lg font-semibold flex-1 min-w-0 truncate" style={{ fontFamily: "var(--font-display)" }}>
                  {title}
                </VaulDrawer.Title>
                {headerAction}
              </div>
              {subHeader && <div className="px-4 pb-3">{subHeader}</div>}
            </div>
          ) : (
            <VaulDrawer.Title className="sr-only">Menu</VaulDrawer.Title>
          )}
          <div className="flex-1 overflow-y-auto p-4">{children}</div>
          {footer && (
            <div className="p-4 border-t border-[var(--border-subtle)]">
              {footer}
            </div>
          )}
        </VaulDrawer.Content>
      </VaulDrawer.Portal>
    </VaulDrawer.Root>
  );
}
