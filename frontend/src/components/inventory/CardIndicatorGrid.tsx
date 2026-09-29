import type { ReactNode } from "react";

/** The bottom indicator row of an inventory card: a fixed 6-column grid so
 *  every icon keeps the same cell on every card (DS rule 18). */
export default function CardIndicatorGrid({ children }: { children: ReactNode }) {
  return (
    <div className="grid grid-cols-6 gap-x-2 gap-y-2 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4 [&>*]:h-5 [&>*]:flex [&>*]:items-center">
      {children}
    </div>
  );
}
