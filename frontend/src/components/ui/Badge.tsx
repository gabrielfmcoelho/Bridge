"use client";

import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { SITUACAO_COLORS, SITUACAO_DOT_COLORS } from "@/lib/constants";

// Full literals on purpose: Tailwind's scanner only generates utilities it can
// read verbatim from source, so these cannot be built from a template string.
const success = "bg-[var(--success)]/15 text-[var(--success)] border-[var(--success)]/30";
const warning = "bg-[var(--warning)]/15 text-[var(--warning)] border-[var(--warning)]/30";
const danger = "bg-[var(--danger)]/15 text-[var(--danger)] border-[var(--danger)]/30";
const info = "bg-[var(--info)]/15 text-[var(--info)] border-[var(--info)]/30";
const neutral = "bg-[var(--bg-overlay)] text-[var(--text-secondary)] border-[var(--border-default)]";

// Semantic names are the vocabulary; the hue names are the legacy keys 52 call
// sites still pass. Both resolve to the same theme-aware tokens.
const colorVariants: Record<string, string> = {
  default: neutral,
  success,
  warning,
  danger,
  info,
  emerald: success,
  amber: warning,
  red: danger,
  sky: info,
  cyan: "bg-[var(--cyan)]/15 text-[var(--cyan)] border-[var(--cyan)]/30",
  purple: "bg-[var(--purple)]/15 text-[var(--purple)] border-[var(--purple)]/30",
  rose: "bg-[var(--rose)]/15 text-[var(--rose)] border-[var(--rose)]/30",
  gray: neutral,
};

interface BadgeProps {
  children: React.ReactNode;
  variant?: "default" | "situacao";
  color?: keyof typeof colorVariants;
  situacao?: string;
  className?: string;
  dot?: boolean;
  /** Show only the dot; expand with label on hover */
  compact?: boolean;
}

export default function Badge({ children, variant = "default", color, situacao, className = "", dot = false, compact = false }: BadgeProps) {
  const base = "inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium border transition-colors";
  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });

  const situacaoColor = situacoes.find((s) => s.value === situacao)?.color;

  if (variant === "situacao" && situacao) {
    const dotColor =
      situacaoColor || (situacao === "active" ? "var(--success)" : situacao === "maintenance" ? "var(--warning)" : "var(--text-faint)");

    if (compact) {
      // The label expands on hover, so colour must not be the only carrier of
      // the state (WCAG 1.4.1): the dot is filled when active and a ring
      // otherwise, and the label is always in the accessible name.
      const label = typeof children === "string" ? children : situacao;
      return (
        <span
          className={`group/badge inline-flex items-center gap-0 rounded-full transition duration-300 cursor-default ${className}`}
          title={label}
          aria-label={label}
          role="img"
        >
          <span
            className={`w-2 h-2 rounded-full shrink-0 ${situacao === "active" ? "" : "border-2 bg-transparent"}`}
            style={situacao === "active" ? { backgroundColor: dotColor } : { borderColor: dotColor }}
          />
          <span
            aria-hidden
            className="max-w-0 overflow-hidden whitespace-nowrap text-xs font-medium opacity-0 group-hover/badge:max-w-[120px] group-hover/badge:opacity-100 group-hover/badge:ml-1.5 group-hover/badge:pr-1 transition-[max-width,opacity,margin,padding] duration-300"
            style={{ color: dotColor }}
          >
            {children}
          </span>
        </span>
      );
    }

    if (situacaoColor) {
      // The enum hex is the backend's identity colour and it knows nothing
      // about the theme: as label text it measured 2.15:1 on light. It stays
      // on the dot and the fill, where a graphic only needs 3:1, and the
      // label takes its contrast from the theme.
      return (
        <span
          className={`${base} text-[var(--text-secondary)] ${className}`}
          style={{
            backgroundColor: `${situacaoColor}26`,
            borderColor: `${situacaoColor}4d`,
          }}
        >
          <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: situacaoColor }} />
          {children}
        </span>
      );
    }

    return (
      <span className={`${base} ${SITUACAO_COLORS[situacao] || SITUACAO_COLORS.inactive} ${className}`}>
        {dot && (
          <span
            className={`w-2 h-2 rounded-full ${SITUACAO_DOT_COLORS[situacao] || SITUACAO_DOT_COLORS.inactive} ${situacao === "active" ? "animate-pulse-glow" : ""}`}
          />
        )}
        {children}
      </span>
    );
  }

  const colorClass = color ? colorVariants[color] : colorVariants.default;

  return (
    <span className={`${base} ${colorClass} ${className}`}>
      {dot && <span className={`w-2 h-2 rounded-full bg-current opacity-60`} />}
      {children}
    </span>
  );
}
