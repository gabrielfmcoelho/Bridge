import type { RequestStatus } from "./types";

// ── Display helpers for the Service Catalog & Requests module ──────────────
// Pure functions only — no React, no server calls. The request state machine
// mirrors the plan's diagram; the server enforces it independently (this is
// UI convenience, not the source of truth — see RequestAbilities in types.ts).

/** The color tokens Badge (src/components/ui/Badge.tsx) actually renders a
 *  class for. Badge's own `color` prop is typed as plain `string` (its
 *  colorVariants map is annotated `Record<string, string>`, so `keyof typeof`
 *  widens to `string` and doesn't constrain callers) — some existing call
 *  sites pass values outside this set and silently fall through to no color
 *  class. This narrower alias keeps statusColor honest about what actually
 *  renders. */
export type BadgeColor = "default" | "emerald" | "cyan" | "amber" | "purple" | "red" | "rose" | "gray";

const STATUS_COLORS: Record<RequestStatus, BadgeColor> = {
  submitted: "default",
  under_review: "amber",
  approved: "emerald",
  in_progress: "cyan",
  delivered: "emerald",
  rejected: "red",
  cancelled: "gray",
  needs_info: "rose",
};

export function statusColor(s: RequestStatus): BadgeColor {
  return STATUS_COLORS[s];
}

export function statusLabelKey(s: RequestStatus): string {
  return `requests.status.${s}`;
}

/** Legal target states per current status. Terminal states transition nowhere. */
export const TRANSITIONS: Record<RequestStatus, RequestStatus[]> = {
  submitted: ["under_review", "cancelled"],
  under_review: ["approved", "rejected", "needs_info", "cancelled"],
  needs_info: ["submitted", "cancelled"],
  approved: ["in_progress", "cancelled"],
  in_progress: ["delivered", "needs_info", "cancelled"],
  delivered: [],
  rejected: [],
  cancelled: [],
};

export function canTransition(from: RequestStatus, to: RequestStatus): boolean {
  return TRANSITIONS[from].includes(to);
}
