import type { ReactNode } from "react";

// ADS lozenge appearances on our semantic tokens.
const appearances = {
  default: "bg-[var(--bg-overlay)] text-[var(--text-secondary)] border-[var(--border-default)]",
  success: "bg-[var(--success)]/15 text-[var(--success)] border-[var(--success)]/30",
  removed: "bg-[var(--danger)]/15 text-[var(--danger)] border-[var(--danger)]/30",
  inprogress: "bg-[var(--info)]/15 text-[var(--info)] border-[var(--info)]/30",
  new: "bg-[var(--accent)]/15 text-[var(--accent)] border-[var(--accent)]/30",
  moved: "bg-[var(--warning)]/15 text-[var(--warning)] border-[var(--warning)]/30",
} as const;

export type LozengeAppearance = keyof typeof appearances;

/** Shape shared with Badge's situação variant, so every status reads alike. */
export const LOZENGE_SHAPE = "inline-flex items-center gap-1.5 px-1.5 py-0.5 rounded-[var(--radius-sm)] text-xs font-semibold border whitespace-nowrap max-w-[12.5rem] truncate";

/**
 * A status (ADS lozenge): workflow state, "ativo", cert state. Square-ish, so
 * it never reads as a tag (pill) or a count. Short sentence-case labels.
 */
export default function Lozenge({ appearance = "default", children, className = "" }: { appearance?: LozengeAppearance; children: ReactNode; className?: string }) {
  return <span className={`${LOZENGE_SHAPE} ${appearances[appearance]} ${className}`}>{children}</span>;
}
