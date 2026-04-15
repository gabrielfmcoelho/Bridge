import type { AlertLevel } from "@/lib/types";

export const ALERT_DOT_COLOR: Record<AlertLevel, string> = {
  critical: "bg-red-400",
  warning: "bg-amber-400",
  info: "bg-sky-400",
};

export const ALERT_TEXT_COLOR: Record<AlertLevel, string> = {
  critical: "text-red-400",
  warning: "text-amber-400",
  info: "text-sky-400",
};

export const PRIORITY_DOT_COLOR: Record<string, string> = {
  critical: "bg-red-400",
  high: "bg-amber-400",
  medium: "bg-[var(--text-faint)]",
  low: "bg-[var(--border-default)]",
};

export const LEVEL_ORDER: Record<string, number> = { critical: 0, warning: 1, info: 2 };
