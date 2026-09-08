import type { FormField } from "./types";

// The boundary between a language model's output and this app's data.
// Extracted from the mock route so it can be tested directly: a model reply is
// untrusted input, and "the model proposes, the schema decides" is only true if
// the deciding half is actually exercised. Moves to Go verbatim at cutover.

/** Pulls the first balanced JSON object out of a completion. Models wrap JSON in
 *  prose or a markdown fence often enough that a bare JSON.parse is a bug. */
export function extractJSON(text: string): Record<string, unknown> | null {
  const start = text.indexOf("{");
  const end = text.lastIndexOf("}");
  if (start === -1 || end <= start) return null;
  try {
    const parsed = JSON.parse(text.slice(start, end + 1));
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

/** Forces a model's answer through the offering's own schema: unknown keys are
 *  dropped, types coerced, selects must match a declared option, numbers must
 *  sit inside the declared range, patterns must hold. Anything that fails is
 *  omitted rather than corrected — a blank field the user fills is safe; a
 *  plausible wrong value they don't notice is not. */
export function coerceToSchema(fields: FormField[], raw: unknown): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (!raw || typeof raw !== "object") return out;
  const obj = raw as Record<string, unknown>;

  for (const f of fields) {
    const v = obj[f.key];
    if (v === undefined || v === null || v === "") continue;

    switch (f.type) {
      case "number": {
        const n = typeof v === "number" ? v : Number(v);
        if (!Number.isFinite(n)) break;
        if (f.min != null && n < f.min) break;
        if (f.max != null && n > f.max) break;
        out[f.key] = n;
        break;
      }
      case "checkbox":
        out[f.key] = Boolean(v);
        break;
      case "select":
        if (f.options?.includes(String(v))) out[f.key] = String(v);
        break;
      case "tags":
        if (Array.isArray(v)) {
          const tags = v.map(String).filter(Boolean);
          if (tags.length > 0) out[f.key] = tags;
        }
        break;
      // asset_ref is a foreign key into real inventory. A model guessing an id
      // would silently attach someone else's host to the request.
      case "asset_ref":
        break;
      default: {
        const str = String(v);
        if (f.max_length && str.length > f.max_length) break;
        if (f.pattern) {
          try {
            if (!new RegExp(f.pattern).test(str)) break;
          } catch {
            /* unparseable authored pattern — treat as no rule, same as validation */
          }
        }
        out[f.key] = str;
      }
    }
  }
  return out;
}

export const DRAFT_PRIORITIES = ["low", "medium", "high", "critical"];

export function coercePriority(v: unknown): string {
  return DRAFT_PRIORITIES.includes(String(v)) ? String(v) : "";
}
