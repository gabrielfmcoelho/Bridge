import type { AlertLevel, VulnSeverity, VulnStatus } from "@/lib/types";
import type { LozengeAppearance } from "@/components/ui/Lozenge";

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

// Vulnerability severity is a Badge (a category), its status a Lozenge
// (workflow state) — DESIGN_SYSTEM.md.
export const VULN_SEVERITY_BADGE: Record<VulnSeverity, "danger" | "warning" | "info" | "default"> = {
  critical: "danger",
  high: "warning",
  medium: "info",
  low: "default",
  info: "default",
};

export const VULN_STATUS_LOZENGE: Record<VulnStatus, LozengeAppearance> = {
  open: "removed",
  accepted: "moved",
  false_positive: "default",
  resolved: "success",
};
