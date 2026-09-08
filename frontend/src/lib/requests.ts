import type { RequestStatus, RequestAbilities, FormField } from "./types";

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

// ── Transition gating ───────────────────────────────────────────────────────

type AbilityKey = keyof RequestAbilities;

/** Which of the server's four abilities gates a given move. Cancelling is
 *  always the requester's (or an admin's) call; leaving `needs_info` belongs
 *  to the requester who has to answer; every other move out of a pre-decision
 *  status is the approver's and out of a post-decision status the fulfiller's.
 *  This is a naming map, not a re-derivation of authority — the booleans still
 *  come from the server (see RequestAbilities in types.ts), and the status
 *  windows here line up with the ones that produce them. */
function gate(from: RequestStatus, to: RequestStatus): AbilityKey {
  if (to === "cancelled") return "cancel";
  if (from === "needs_info") return "edit";
  if (from === "approved" || from === "in_progress") return "fulfill";
  return "approve";
}

/** Legal targets from `from` that the current actor is also allowed to take. */
export function availableTransitions(from: RequestStatus, can: RequestAbilities): RequestStatus[] {
  return TRANSITIONS[from].filter((to) => can[gate(from, to)]);
}

/** Label for the button that performs a move *into* each status. */
export const TRANSITION_ACTION_KEY: Record<RequestStatus, string> = {
  under_review: "requests.actions.startReview",
  approved: "requests.actions.approve",
  rejected: "requests.actions.reject",
  needs_info: "requests.actions.requestInfo",
  submitted: "requests.actions.resubmit",
  in_progress: "requests.actions.startWork",
  delivered: "requests.actions.markDelivered",
  cancelled: "requests.actions.cancel",
};

export function transitionVariant(to: RequestStatus): "primary" | "secondary" | "danger" {
  if (to === "rejected" || to === "cancelled") return "danger";
  if (to === "approved" || to === "delivered") return "primary";
  return "secondary";
}

/** Moves that must carry an explanation. A bare rejection or a bare "needs
 *  info" leaves the requester with nothing to act on.
 *  ponytail: enforced client-side only — the mock (and B5's service layer)
 *  accepts `note` as optional. Move it server-side when Go lands. */
export function transitionNeedsNote(to: RequestStatus): boolean {
  return to === "rejected" || to === "needs_info";
}

// ── Asset display ───────────────────────────────────────────────────────────

/** asset_type -> the nav label already used for that inventory section, so a
 *  type badge reads consistently with the sidebar entry its link opens. */
const ASSET_TYPE_LABEL_KEY: Record<string, string> = {
  host: "nav.hosts",
  service: "nav.services",
  dns: "nav.dns",
  project: "nav.projects",
  api_catalog: "nav.apis",
  tool: "nav.tools",
};

export function assetTypeLabelKey(assetType: string): string | undefined {
  return ASSET_TYPE_LABEL_KEY[assetType];
}

// ponytail: only the asset types whose detail route takes a numeric id. Hosts
// route by slug (`/hosts/[slug]`) and a request row carries only
// fulfilled_asset_id, so a host link cannot be built from the record alone —
// it renders as plain text instead of a dead link. Upgrade path: have the
// server return a ready `href` the way CatalogHit already does (see the
// catalog/search shape), rather than resolving slugs in the client.
const ASSET_ID_ROUTES: Record<string, string> = {
  dns: "/dns",
  service: "/services",
  project: "/projects",
};

export function assetHref(assetType: string, id: number): string | null {
  const base = ASSET_ID_ROUTES[assetType];
  return base ? `${base}/${id}` : null;
}

// ── Offering form_schema: defaults + validation ─────────────────────────────
// Shared by the create modal (catalog) and the edit modal (request detail) —
// two callers must agree on what "empty" and "invalid" mean, or a request
// could be created valid and then fail to save unchanged.

/** Defaults each schema field to a type-appropriate empty value so form_data
 *  always carries every field's key, not just the ones the user touched. */
export function emptyFormValues(fields: FormField[]): Record<string, unknown> {
  const values: Record<string, unknown> = {};
  for (const f of fields) {
    values[f.key] = f.type === "checkbox" ? false : f.type === "tags" ? [] : f.type === "asset_ref" ? null : "";
  }
  return values;
}

/** Fields whose `depends_on` condition currently holds. Everything downstream —
 *  rendering, validation, and the submitted form_data — works off this list, so
 *  a hidden field can never be required, never blocks submit, and never leaves
 *  a stale answer behind. */
export function visibleFields(fields: FormField[], values: Record<string, unknown>): FormField[] {
  return fields.filter((f) => {
    if (!f.depends_on) return true;
    return f.depends_on.equals.includes(String(values[f.depends_on.key] ?? ""));
  });
}

/** form_data limited to the fields the user could actually see. */
export function visibleValues(fields: FormField[], values: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const f of visibleFields(fields, values)) out[f.key] = values[f.key];
  return out;
}

/** Compiles an authored pattern, treating a bad regex as "no rule" rather than
 *  letting one typo in an offering take the whole form down. */
function compilePattern(src: string): RegExp | null {
  try {
    return new RegExp(src);
  } catch {
    return null;
  }
}

export function isFieldEmpty(field: FormField, v: unknown): boolean {
  if (field.type === "tags") return !Array.isArray(v) || v.length === 0;
  if (field.type === "checkbox") return false; // false is a real answer, not a blank
  return v === undefined || v === null || v === "";
}

/** Hand-rolled per C2 — no zod, no react-hook-form. `t` is passed in rather
 *  than imported so this file stays free of React context. */
export function validateFormData(
  fields: FormField[],
  values: Record<string, unknown>,
  t: (key: string, vars?: Record<string, string>) => string,
): Record<string, string> {
  const errs: Record<string, string> = {};
  // Only what the user can see is judged — see visibleFields.
  for (const field of visibleFields(fields, values)) {
    const v = values[field.key];
    const empty = isFieldEmpty(field, v);

    if (field.required && empty) {
      errs[field.key] = t("requests.form.required");
      continue;
    }
    if (empty) continue;

    if (field.type === "number") {
      if (typeof v !== "number" || Number.isNaN(v)) {
        errs[field.key] = t("requests.form.invalidNumber");
      } else if (field.min != null && v < field.min) {
        errs[field.key] = t("requests.form.min", { min: String(field.min) });
      } else if (field.max != null && v > field.max) {
        errs[field.key] = t("requests.form.max", { max: String(field.max) });
      }
    } else if (field.type === "text" || field.type === "textarea") {
      const str = String(v);
      const re = field.pattern ? compilePattern(field.pattern) : null;
      if (field.max_length && str.length > field.max_length) {
        errs[field.key] = t("requests.form.maxLength", { max: String(field.max_length) });
      } else if (re && !re.test(str)) {
        errs[field.key] = t("requests.form.invalidFormat");
      }
    } else if (field.type === "select" && field.options && !field.options.includes(String(v))) {
      errs[field.key] = t("requests.form.invalidOption");
    }
  }
  return errs;
}
