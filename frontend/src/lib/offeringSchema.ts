import type { AssetType, FormField, FormFieldType, FormSchema, OfferingTemplate } from "./types";

// Validates what an admin pastes into the offering editor. form_schema and
// templates are authored data (JSONB), so the only thing standing between a
// typo and a form nobody can submit is this file. Issues carry a code plus the
// offending position/key; the UI maps codes to i18n so the messages stay
// bilingual without dragging t() into a pure module.
//
// Mirrors the closed vocabularies DynamicField renders — an unknown `type`
// renders nothing there, so it must be rejected here.

export type SchemaIssue = { code: string; n?: number; key?: string; value?: string };

const FIELD_TYPES: readonly FormFieldType[] = ["text", "textarea", "number", "select", "checkbox", "date", "tags", "asset_ref"];
const ASSET_TYPES: readonly AssetType[] = ["host", "dns", "service", "project", "contact", "tool", "ssh_key", "api_catalog", "secret", "offering"];
/** Field keys become form_data keys, so they stay identifier-shaped. */
const FIELD_KEY_RE = /^[a-z][a-z0-9_]*$/;
/** Template keys are only labels for a preset; kebab-case is fine (and is what the seed uses). */
const TEMPLATE_KEY_RE = /^[a-z0-9][a-z0-9_-]*$/;

const isObj = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const nonEmpty = (v: unknown): v is string => typeof v === "string" && v.trim() !== "";

function parseJSON(text: string): { value: unknown; error: string | null } {
  try {
    return { value: JSON.parse(text), error: null };
  } catch (e) {
    return { value: undefined, error: e instanceof Error ? e.message : "invalid JSON" };
  }
}

/** Shared by fields and templates: a well-formed key that has not been used yet.
 *  `prefix` distinguishes the codes so the UI can say "Field 3" vs "Template 3". */
function checkKey(raw: Record<string, unknown>, n: number, seen: Set<string>, issues: SchemaIssue[], re: RegExp, prefix = ""): string {
  const key = typeof raw.key === "string" ? raw.key : "";
  const code = (c: string) => (prefix ? prefix + c[0].toUpperCase() + c.slice(1) : c);
  if (!re.test(key)) issues.push({ code: code("badKey"), n, key });
  else if (seen.has(key)) issues.push({ code: code("dupKey"), n, key });
  seen.add(key);
  return key;
}

export function parseFormSchema(text: string): { value: FormSchema | null; errors: SchemaIssue[] } {
  // Blank is "not written yet", not a malformed document — the submit guard
  // still refuses a null value, so nothing unvalidated gets through.
  if (text.trim() === "") return { value: null, errors: [] };
  const { value: json, error } = parseJSON(text);
  if (error) return { value: null, errors: [{ code: "invalidJson", value: error }] };

  const fields = Array.isArray(json) ? json : isObj(json) ? json.fields : undefined;
  if (!Array.isArray(fields)) return { value: null, errors: [{ code: "notArray" }] };
  if (fields.length === 0) return { value: null, errors: [{ code: "empty" }] };

  const issues: SchemaIssue[] = [];
  const seen = new Set<string>();
  const out: FormField[] = [];

  fields.forEach((raw, i) => {
    const n = i + 1;
    if (!isObj(raw)) return issues.push({ code: "badKey", n, key: "" });
    const key = checkKey(raw, n, seen, issues, FIELD_KEY_RE);
    if (!FIELD_TYPES.includes(raw.type as FormFieldType)) issues.push({ code: "badType", n, key, value: String(raw.type) });
    if (!nonEmpty(raw.label_en) || !nonEmpty(raw.label_pt)) issues.push({ code: "missingLabel", n, key });

    if (raw.type === "select" && !(Array.isArray(raw.options) && raw.options.length > 0 && raw.options.every(nonEmpty)))
      issues.push({ code: "noOptions", n, key });
    if (raw.type === "asset_ref" && !ASSET_TYPES.includes(raw.asset_type as AssetType))
      issues.push({ code: "badAssetType", n, key, value: String(raw.asset_type) });
    if (raw.type === "number" && typeof raw.min === "number" && typeof raw.max === "number" && raw.min > raw.max)
      issues.push({ code: "minMax", n, key });
    if (typeof raw.pattern === "string") {
      try {
        new RegExp(raw.pattern);
      } catch {
        issues.push({ code: "badPattern", n, key });
      }
    }
    out.push({ ...(raw as unknown as FormField), required: raw.required === true });
  });

  // Second pass: depends_on may legitimately point forward, so resolve against
  // the full key set rather than "keys seen so far".
  fields.forEach((raw, i) => {
    if (!isObj(raw) || raw.depends_on === undefined) return;
    const dep = raw.depends_on;
    const ok = isObj(dep) && typeof dep.key === "string" && dep.key !== raw.key && seen.has(dep.key) && Array.isArray(dep.equals) && dep.equals.length > 0;
    if (!ok) issues.push({ code: "badDependsOn", n: i + 1, key: String(raw.key ?? "") });
  });

  return issues.length ? { value: null, errors: issues } : { value: { fields: out }, errors: [] };
}

export function parseTemplates(text: string, fields: FormField[]): { value: OfferingTemplate[]; errors: SchemaIssue[] } {
  if (text.trim() === "") return { value: [], errors: [] };
  const { value: json, error } = parseJSON(text);
  if (error) return { value: [], errors: [{ code: "invalidJson", value: error }] };
  if (!Array.isArray(json)) return { value: [], errors: [{ code: "notArray" }] };

  const known = new Set(fields.map((f) => f.key));
  const issues: SchemaIssue[] = [];
  const seen = new Set<string>();
  const out: OfferingTemplate[] = [];

  json.forEach((raw, i) => {
    const n = i + 1;
    if (!isObj(raw)) return issues.push({ code: "tplBadKey", n, key: "" });
    const key = checkKey(raw, n, seen, issues, TEMPLATE_KEY_RE, "tpl");
    if (!nonEmpty(raw.name_en) || !nonEmpty(raw.name_pt)) issues.push({ code: "tplMissingLabel", n, key });
    const values = isObj(raw.values) ? raw.values : {};
    for (const k of Object.keys(values)) if (!known.has(k)) issues.push({ code: "unknownValueKey", n, key: k });
    out.push({
      key,
      name_en: String(raw.name_en ?? ""),
      name_pt: String(raw.name_pt ?? ""),
      summary_en: typeof raw.summary_en === "string" ? raw.summary_en : "",
      summary_pt: typeof raw.summary_pt === "string" ? raw.summary_pt : "",
      values,
    });
  });

  return issues.length ? { value: [], errors: issues } : { value: out, errors: [] };
}
