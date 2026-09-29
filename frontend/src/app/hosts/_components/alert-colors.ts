import type { AlertLevel } from "@/lib/types";

export const ALERT_DOT_COLOR: Record<AlertLevel, string> = {
  critical: "bg-[var(--danger)]",
  warning: "bg-[var(--warning)]",
  info: "bg-[var(--info)]",
};

export const ALERT_TEXT_COLOR: Record<AlertLevel, string> = {
  critical: "text-[var(--danger)]",
  warning: "text-[var(--warning)]",
  info: "text-[var(--info)]",
};

export const PRIORITY_DOT_COLOR: Record<string, string> = {
  critical: "bg-[var(--danger)]",
  high: "bg-[var(--warning)]",
  medium: "bg-[var(--text-faint)]",
  low: "bg-[var(--border-default)]",
};

export const LEVEL_ORDER: Record<string, number> = { critical: 0, warning: 1, info: 2 };
