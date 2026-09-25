"use client";

import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { situacaoColorOf } from "@/lib/constants";

/**
 * Situação as a line of text (no badge): a dot and the label in the state's
 * colour. The dot is filled only for "active" and a ring otherwise, so the
 * state never rides on colour alone (rule 17); the label is mixed toward the
 * primary text colour so a backend enum hex can't fail text contrast. Always
 * renders — "–" when there is no situação — so the slot stays in place.
 */
export default function SituacaoText({ situacao, className = "" }: { situacao?: string; className?: string }) {
  const { data: situacoes = [] } = useQuery({ queryKey: ["enums", "situacao"], queryFn: () => enumsAPI.list("situacao") });
  if (!situacao) return <span className={`text-xs text-[var(--text-muted)] ${className}`}>–</span>;
  const color = situacaoColorOf(situacao, situacoes.find((s) => s.value === situacao)?.color);
  return <StatusText color={color} on={situacao === "active"} label={situacao} className={className} />;
}

/** The status line's look for any state: a dot (filled when `on`, ring
 *  otherwise) and the label in the state's colour, contrast-safe. */
export function StatusText({ color, on, label, className = "" }: { color: string; on: boolean; label: string; className?: string }) {
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-medium ${className}`} style={{ color: `color-mix(in oklab, ${color} 65%, var(--text-primary))` }}>
      <span aria-hidden className={`w-2 h-2 rounded-full shrink-0 ${on ? "" : "border-2"}`} style={on ? { backgroundColor: color } : { borderColor: color }} />
      {label}
    </span>
  );
}
