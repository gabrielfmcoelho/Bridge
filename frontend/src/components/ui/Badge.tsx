"use client";

import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { SITUACAO_COLORS } from "@/lib/constants";

const colorVariants: Record<string, string> = {
  default: "bg-[var(--bg-overlay)] text-[var(--text-secondary)] border-[var(--border-default)]",
  emerald: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30",
  cyan: "bg-cyan-500/15 text-cyan-400 border-cyan-500/30",
  amber: "bg-amber-500/15 text-amber-400 border-amber-500/30",
  purple: "bg-purple-500/15 text-purple-400 border-purple-500/30",
  red: "bg-red-500/15 text-red-400 border-red-500/30",
  rose: "bg-rose-500/15 text-rose-400 border-rose-500/30",
  gray: "bg-gray-500/15 text-gray-400 border-gray-500/30",
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
    const dotColor = situacaoColor || (situacao === "active" ? "#10b981" : situacao === "maintenance" ? "#eab308" : "#6b7280");

    if (compact) {
      return (
        <span className={`group/badge inline-flex items-center gap-0 rounded-full transition-all duration-300 cursor-default ${className}`}>
          <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: dotColor }} />
          <span
            className="max-w-0 overflow-hidden whitespace-nowrap text-xs font-medium opacity-0 group-hover/badge:max-w-[120px] group-hover/badge:opacity-100 group-hover/badge:ml-1.5 group-hover/badge:pr-1 transition-all duration-300"
            style={{ color: dotColor }}
          >
            {children}
          </span>
        </span>
      );
    }

    if (situacaoColor) {
      return (
        <span
          className={`${base} ${className}`}
          style={{
            backgroundColor: `${situacaoColor}26`,
            color: situacaoColor,
            borderColor: `${situacaoColor}4d`,
          }}
        >
          {dot && <span className="w-2 h-2 rounded-full" style={{ backgroundColor: situacaoColor }} />}
          {children}
        </span>
      );
    }

    return (
      <span className={`${base} ${SITUACAO_COLORS[situacao] || SITUACAO_COLORS.inactive} ${className}`}>
        {dot && <span className={`w-2 h-2 rounded-full ${situacao === "active" ? "bg-emerald-400 animate-pulse-glow" : situacao === "maintenance" ? "bg-yellow-400" : "bg-gray-400"}`} />}
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
