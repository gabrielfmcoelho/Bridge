"use client";

import { useState } from "react";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import Select from "@/components/ui/Select";
import CheckboxList from "@/components/ui/CheckboxList";
import RadioGroup from "@/components/ui/RadioGroup";
import DateTimeInput from "@/components/ui/DateTimeInput";
import AsyncPicker, { type AsyncPickerItem } from "@/components/ui/AsyncPicker";
import FormcreatorFileInput, { type UploadedDoc } from "@/components/glpi/FormcreatorFileInput";
import { glpiAPI, type FormcreatorQuestion as Question } from "@/lib/api";

// Local copies of the helpers in TicketDetailDrawer.tsx so this component
// doesn't depend on the drawer module. They're kept tiny on purpose — moving
// them to a shared lib is a follow-up if we need them in a third place.
function decodeGlpiHtml(s?: string): string {
  if (!s) return "";
  if (typeof document === "undefined") return s;
  const ta = document.createElement("textarea");
  ta.innerHTML = s;
  return ta.value;
}
function rewriteGlpiLinks(html: string, base: string, profileID: number | null): string {
  if (!html) return html;
  if (typeof document === "undefined") return html;
  const root = (base || "").replace(/\/+$/, "");
  const doc = new DOMParser().parseFromString(`<div>${html}</div>`, "text/html");
  const proxyDoc = (id: number) =>
    `/api/glpi/documents/${id}${profileID ? `?profile_id=${profileID}` : ""}`;
  const fix = (el: Element, attr: "href" | "src") => {
    const v = el.getAttribute(attr);
    if (!v) return;
    const docMatch = /document\.send\.php\?([^#]*)/i.exec(v);
    if (docMatch) {
      const params = new URLSearchParams(docMatch[1]);
      const id = parseInt(params.get("docid") || "", 10);
      if (Number.isFinite(id) && id > 0 && profileID) {
        el.setAttribute(attr, proxyDoc(id));
        if (attr === "href") {
          el.setAttribute("target", "_blank");
          el.setAttribute("rel", "noopener noreferrer");
        }
        return;
      }
    }
    if (/^(https?:)?\/\//i.test(v)) return;
    if (/^(mailto:|tel:|data:|javascript:|#)/i.test(v)) return;
    if (!root) return;
    el.setAttribute(attr, v.startsWith("/") ? root + v : root + "/" + v);
    if (attr === "href") {
      el.setAttribute("target", "_blank");
      el.setAttribute("rel", "noopener noreferrer");
    }
  };
  doc.querySelectorAll("a[href]").forEach((el) => fix(el, "href"));
  doc.querySelectorAll("img[src]").forEach((el) => fix(el, "src"));
  return doc.body.firstElementChild?.innerHTML ?? html;
}

// parseQuestionValues unpacks Formcreator's `values` field. Formcreator has
// evolved the shape over releases — we accept any of:
//   1. JSON array of strings: ["A","B","C"]
//   2. JSON object mapping value → label: {"a":"Apple","b":"Banana"}
//   3. Newline-separated plain text: "A\nB\nC"
// Returns {value,label} pairs ready for Select / CheckboxList / RadioGroup.
function parseQuestionValues(raw: string | undefined): { value: string; label: string }[] {
  if (!raw) return [];
  const trimmed = raw.trim();
  if (!trimmed) return [];
  // JSON forms first — if parsing fails, fall through to the text split.
  if (trimmed.startsWith("[") || trimmed.startsWith("{")) {
    try {
      const parsed = JSON.parse(trimmed);
      if (Array.isArray(parsed)) {
        return parsed.map((v) => {
          const s = String(v);
          return { value: s, label: s };
        });
      }
      if (parsed && typeof parsed === "object") {
        return Object.entries(parsed as Record<string, unknown>).map(([k, v]) => ({
          value: k,
          label: String(v ?? k),
        }));
      }
    } catch {
      // Fall through to newline split.
    }
  }
  return trimmed
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter(Boolean)
    .map((s) => ({ value: s, label: s }));
}

interface Props {
  question: Question;
  value: unknown;
  onChange: (value: unknown) => void;
  visible: boolean;
  formID: number;
  glpiBaseURL: string;
  profileID: number | null;
  error?: string;
}

const SUPPORTED = new Set([
  "text",
  "textarea",
  "description",
  "select",
  "radios",
  "checkboxes",
  "multiselect",
  "integer",
  "float",
  "number",
  "email",
  "date",
  "datetime",
  "time",
  // Phase 2
  "urgency",
  "ip",
  "file",
  "glpiselect",
  "dropdown",
  "itemtype",
  "actor",
  // Phase 3
  "tag",
  // Formcreator's "hidden" is a no-op in the UI — it carries its default value
  // straight through to submission. Render nothing.
  "hidden",
]);

// readActorConfig teases the "actor_type" hint out of the question's values
// JSON. Formcreator's actor field supports single or multi selection — the
// shape is either "requester" (single) or "requester,observer" (multi).
function readActorConfig(rawValues: string | undefined): { multi: boolean } {
  if (!rawValues) return { multi: false };
  try {
    const parsed = JSON.parse(rawValues);
    // Some versions expose an explicit boolean, others use the comma-list.
    if (typeof parsed?.multiple === "boolean") return { multi: parsed.multiple };
    if (typeof parsed?.actor_type === "string") {
      return { multi: parsed.actor_type.includes(",") };
    }
  } catch {
    // fall through
  }
  return { multi: false };
}

// readNumericRange parses the optional {min,max} Formcreator stores on
// numeric fields' `values` JSON. Used for client-side validation.
function readNumericRange(rawValues: string | undefined): { min?: number; max?: number } {
  if (!rawValues) return {};
  try {
    const parsed = JSON.parse(rawValues);
    const r = parsed?.range ?? parsed;
    const min = Number(r?.min);
    const max = Number(r?.max);
    return {
      min: Number.isFinite(min) ? min : undefined,
      max: Number.isFinite(max) ? max : undefined,
    };
  } catch {
    return {};
  }
}

// Parse the GLPI itemtype stored in a question's `values` JSON — applies to
// glpiselect / dropdown / itemtype. Formcreator has used several key names for
// this config over the years; try each before giving up.
function readItemtype(rawValues: string | undefined): string {
  if (!rawValues) return "";
  try {
    const parsed = JSON.parse(rawValues);
    const candidates = [
      parsed?.itemtype,
      parsed?.glpi_object,
      parsed?.glpiitemtype,
      parsed?.objecttype,
    ];
    for (const c of candidates) {
      if (typeof c === "string" && c) return c;
    }
  } catch {
    // non-JSON values — fall through
  }
  return "";
}

// Parse a file question's config. `max_files` and `extensions` (comma-
// separated) are the fields Formcreator exposes.
function readFileConfig(rawValues: string | undefined): { maxFiles?: number; accept?: string } {
  if (!rawValues) return {};
  try {
    const parsed = JSON.parse(rawValues);
    const maxRaw = parsed?.max_files;
    const n = typeof maxRaw === "string" ? parseInt(maxRaw, 10) : Number(maxRaw);
    const exts = typeof parsed?.extensions === "string" ? parsed.extensions : "";
    return {
      maxFiles: Number.isFinite(n) && n > 0 ? n : undefined,
      accept: exts
        ? exts
            .split(",")
            .map((s: string) => s.trim())
            .filter(Boolean)
            .map((ext: string) => (ext.startsWith(".") ? ext : "." + ext))
            .join(",")
        : undefined,
    };
  } catch {
    return {};
  }
}

// IPv4 + simple IPv6 check. Formcreator's web UI uses a looser pattern; this
// catches obvious garbage without rejecting valid inputs like "::1".
const IPV4_RE = /^(\d{1,3}\.){3}\d{1,3}$/;
const IPV6_RE = /^[0-9a-fA-F:]+$/;
function isValidIP(v: string): boolean {
  if (!v) return true; // empty is handled by "required" logic separately
  if (IPV4_RE.test(v)) {
    return v.split(".").every((n) => {
      const i = parseInt(n, 10);
      return i >= 0 && i <= 255;
    });
  }
  return IPV6_RE.test(v) && v.includes(":");
}

const URGENCY_OPTIONS = [
  { value: "1", label: "1 – Muito baixa" },
  { value: "2", label: "2 – Baixa" },
  { value: "3", label: "3 – Média" },
  { value: "4", label: "4 – Alta" },
  { value: "5", label: "5 – Muito alta" },
];

export default function FormcreatorQuestion({
  question,
  value,
  onChange,
  visible,
  formID,
  glpiBaseURL,
  profileID,
  error,
}: Props) {
  if (!visible) return null;

  const required = question.required === 1;
  const labelText = required ? `${question.name} *` : question.name;

  // Hidden fields don't render anything — their default_values were already
  // seeded into the answers map by the drawer, and that value rides with the
  // submission untouched.
  if (question.fieldtype === "hidden") {
    return null;
  }

  // Description = static HTML block, never an input.
  if (question.fieldtype === "description") {
    return (
      <div
        className="glpi-content text-sm text-[var(--text-secondary)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2.5"
        dangerouslySetInnerHTML={{
          __html: rewriteGlpiLinks(decodeGlpiHtml(question.description), glpiBaseURL, profileID),
        }}
      />
    );
  }

  if (!SUPPORTED.has(question.fieldtype)) {
    const fallbackHref = `${glpiBaseURL.replace(/\/+$/, "")}/marketplace/formcreator/front/formdisplay.php?id=${formID}`;
    return (
      <div className="space-y-1">
        <label className="block text-xs font-medium text-[var(--text-secondary)]">
          {labelText}
        </label>
        <div className="flex items-center justify-between gap-2 px-3 py-2 text-xs rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300">
          <span>
            Tipo <code className="font-mono">{question.fieldtype}</code> ainda não é suportado em sshcm.
          </span>
          <a
            href={fallbackHref}
            target="_blank"
            rel="noopener noreferrer"
            className="text-amber-200 hover:underline shrink-0"
          >
            Abrir no GLPI ↗
          </a>
        </div>
      </div>
    );
  }

  const stringValue = value == null ? "" : String(value);
  const arrayValue = Array.isArray(value) ? value.map(String) : [];
  const opts = parseQuestionValues(question.values);

  switch (question.fieldtype) {
    case "text": {
      // Apply the question's regex (if any) as a live validation hint.
      let localError = error;
      if (!localError && stringValue && question.regex) {
        try {
          if (!new RegExp(question.regex).test(stringValue)) {
            localError = "Formato inválido";
          }
        } catch {
          // invalid regex in the form definition — ignore silently
        }
      }
      return (
        <Input
          label={labelText}
          value={stringValue}
          onChange={(e) => onChange(e.target.value)}
          error={localError}
        />
      );
    }
    case "email":
      return (
        <Input
          label={labelText}
          type="email"
          value={stringValue}
          onChange={(e) => onChange(e.target.value)}
          error={error}
        />
      );
    case "integer":
    case "number":
    case "float": {
      const range = readNumericRange(question.values);
      let localError = error;
      if (!localError && stringValue) {
        const n = Number(stringValue);
        if (!Number.isFinite(n)) {
          localError = "Número inválido";
        } else if (range.min != null && n < range.min) {
          localError = `Mínimo: ${range.min}`;
        } else if (range.max != null && n > range.max) {
          localError = `Máximo: ${range.max}`;
        }
      }
      return (
        <Input
          label={labelText}
          type="number"
          step={question.fieldtype === "integer" ? 1 : "any"}
          min={range.min}
          max={range.max}
          value={stringValue}
          onChange={(e) => onChange(e.target.value)}
          error={localError}
        />
      );
    }
    case "textarea":
      return (
        <Textarea
          label={labelText}
          value={stringValue}
          onChange={(e) => onChange(e.target.value)}
          error={error}
          rows={4}
        />
      );
    case "select":
      return (
        <Select
          label={labelText}
          options={opts}
          value={stringValue}
          onChange={(e) => onChange(e.target.value)}
          error={error}
        />
      );
    case "radios":
      return (
        <RadioGroup
          label={labelText}
          options={opts}
          value={stringValue}
          onChange={(v) => onChange(v)}
          error={error}
        />
      );
    case "checkboxes":
    case "multiselect": {
      // CheckboxList expects {id:number,name:string} shape — adapt by mapping
      // string options to a stable numeric index, then convert back on change.
      const items = opts.map((o, i) => ({ id: i, name: o.label }));
      const selectedIdx = arrayValue
        .map((v) => opts.findIndex((o) => o.value === v))
        .filter((i) => i >= 0);
      return (
        <div className="space-y-1.5">
          <CheckboxList
            label={labelText}
            items={items}
            selected={selectedIdx}
            onChange={(idx) => onChange(idx.map((i) => opts[i]?.value).filter(Boolean))}
          />
          {error && <p className="text-xs text-red-400">{error}</p>}
        </div>
      );
    }
    case "date":
      return (
        <DateTimeInput
          label={labelText}
          variant="date"
          value={stringValue}
          onChange={onChange}
          error={error}
        />
      );
    case "datetime":
      return (
        <DateTimeInput
          label={labelText}
          variant="datetime"
          value={stringValue}
          onChange={onChange}
          error={error}
        />
      );
    case "time":
      return (
        <DateTimeInput
          label={labelText}
          variant="time"
          value={stringValue}
          onChange={onChange}
          error={error}
        />
      );
    case "urgency":
      return (
        <Select
          label={labelText}
          options={URGENCY_OPTIONS}
          value={stringValue}
          onChange={(e) => onChange(e.target.value)}
          error={error}
        />
      );
    case "ip": {
      const ipError = error ?? (stringValue && !isValidIP(stringValue) ? "IP inválido" : undefined);
      return (
        <Input
          label={labelText}
          value={stringValue}
          onChange={(e) => onChange(e.target.value)}
          placeholder="e.g. 10.0.0.1 or ::1"
          error={ipError}
        />
      );
    }
    case "glpiselect":
    case "dropdown":
    case "itemtype":
      return (
        <DropdownWithFallback
          label={labelText}
          itemtype={readItemtype(question.values)}
          value={value}
          onChange={onChange}
          profileID={profileID}
          error={error}
        />
      );
    case "actor": {
      const actorCfg = readActorConfig(question.values);
      const userFetcher = async (q: string) => {
        if (!profileID) return [];
        const res = await glpiAPI.searchUsers(profileID, q);
        return res.users.map((u) => ({
          id: u.id,
          label: u.display || u.login,
          secondary: u.login && u.display && u.login !== u.display ? u.login : undefined,
        } as AsyncPickerItem));
      };
      if (actorCfg.multi) {
        const selectedItems = parseLabeledIntArray(value);
        return (
          <AsyncPicker
            multi
            label={labelText}
            selectedItems={selectedItems}
            fetcher={userFetcher}
            onChange={(items) =>
              onChange(items.map((it) => `${it.id}|${it.label}`))
            }
            error={error}
            placeholder="Buscar usuários…"
            disabled={!profileID}
          />
        );
      }
      const [id, label] = splitLabeledInt(value);
      return (
        <AsyncPicker
          label={labelText}
          value={id}
          selectedLabel={label}
          fetcher={userFetcher}
          onChange={(item) => onChange(item ? `${item.id}|${item.label}` : "")}
          error={error}
          placeholder="Buscar usuário…"
          disabled={!profileID}
        />
      );
    }
    case "tag": {
      // Tags are always multi-select in Formcreator. Value is an array of
      // "<id>|<label>#<color>" strings so we preserve the tag color in the
      // closed-state chips without re-fetching. The color suffix is optional.
      const selectedItems = parseLabeledIntArray(value);
      return (
        <AsyncPicker
          multi
          label={labelText}
          selectedItems={selectedItems}
          fetcher={async (q) => {
            if (!profileID) return [];
            const res = await glpiAPI.searchFormcreatorTags(profileID, q);
            return res.tags.map((t) => ({
              id: t.id,
              label: t.name,
              color: t.color,
            } as AsyncPickerItem));
          }}
          onChange={(items) =>
            onChange(
              items.map((it) =>
                it.color
                  ? `${it.id}|${it.label}#${it.color.replace(/^#/, "")}`
                  : `${it.id}|${it.label}`
              )
            )
          }
          error={error}
          placeholder="Buscar tags…"
          disabled={!profileID}
        />
      );
    }
    case "file": {
      const cfg = readFileConfig(question.values);
      const docs = Array.isArray(value) ? (value as UploadedDoc[]) : [];
      return (
        <FormcreatorFileInput
          label={labelText}
          profileID={profileID}
          value={docs}
          onChange={(next) => onChange(next)}
          accept={cfg.accept}
          maxFiles={cfg.maxFiles}
          error={error}
        />
      );
    }
    default:
      // Already covered by SUPPORTED gate above.
      return null;
  }
}

// Field types whose answer is a GLPI item id. For these, a stored value of
// `0` is GLPI's "unset" sentinel and must NOT be submitted — GLPI rejects
// required FKs with value 0 as `ERROR_GLPI_ADD`.
export const ID_LIKE_FIELDTYPES = new Set([
  "glpiselect",
  "dropdown",
  "itemtype",
  "actor",
]);

// splitLabeledInt unpacks "<id>|<label>" encoding used by AsyncPicker answers.
// Returns [id | null, label | undefined]. An id of 0 is coerced to null so
// Formcreator defaults of "0" display as an empty selection.
function splitLabeledInt(raw: unknown): [number | null, string | undefined] {
  if (raw == null || raw === "") return [null, undefined];
  const s = String(raw);
  const pipe = s.indexOf("|");
  const idStr = pipe >= 0 ? s.slice(0, pipe) : s;
  const label = pipe >= 0 ? s.slice(pipe + 1) : undefined;
  const id = parseInt(idStr, 10);
  if (!Number.isFinite(id) || id === 0) return [null, undefined];
  return [id, label];
}

// parseLabeledIntArray decodes the multi-select encoding `["id|label"]` or
// `["id|label#color"]` into AsyncPicker items with their id/label/color
// preserved. Used by the `tag` and multi-mode `actor` question types.
function parseLabeledIntArray(raw: unknown): AsyncPickerItem[] {
  if (!Array.isArray(raw)) return [];
  const out: AsyncPickerItem[] = [];
  for (const entry of raw) {
    const s = String(entry);
    const pipe = s.indexOf("|");
    if (pipe < 0) continue;
    const id = parseInt(s.slice(0, pipe), 10);
    if (!Number.isFinite(id)) continue;
    const rest = s.slice(pipe + 1);
    const hash = rest.lastIndexOf("#");
    // Heuristic: a `#` followed by 6 hex chars is a color suffix.
    const hasColor = hash > 0 && /^#[0-9a-fA-F]{3,8}$/.test("#" + rest.slice(hash + 1));
    out.push(
      hasColor
        ? { id, label: rest.slice(0, hash), color: "#" + rest.slice(hash + 1) }
        : { id, label: rest }
    );
  }
  return out;
}

export { SUPPORTED as SUPPORTED_QUESTION_TYPES, parseQuestionValues };

// DropdownWithFallback wraps the AsyncPicker-based dropdown with an "enter
// id manually" escape hatch. Needed because some GLPI profiles can't READ
// the target itemtype via REST even though the GLPI web UI allows it via
// internal ajax endpoints we can't call from sshcm. When `itemtype` couldn't
// be parsed from the question config, we show the manual mode by default.
function DropdownWithFallback({
  label,
  itemtype,
  value,
  onChange,
  profileID,
  error,
}: {
  label: string;
  itemtype: string;
  value: unknown;
  onChange: (v: unknown) => void;
  profileID: number | null;
  error?: string;
}) {
  const hasItemtype = itemtype.length > 0;
  const [manual, setManual] = useState(!hasItemtype);
  const [id, labelFromValue] = splitLabeledInt(value);

  if (manual) {
    return (
      <div className="space-y-1.5">
        <Input
          label={`${label}${hasItemtype ? ` · ${itemtype}` : ""}`}
          type="number"
          step={1}
          value={id != null ? String(id) : ""}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10);
            onChange(Number.isFinite(n) && n > 0 ? `${n}|#${n}` : "");
          }}
          placeholder="ID numérico no GLPI"
          error={error}
        />
        {hasItemtype && (
          <button
            type="button"
            onClick={() => setManual(false)}
            className="text-[11px] text-[var(--accent)] hover:underline"
          >
            ← Voltar à busca
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      <AsyncPicker
        label={`${label} · ${itemtype}`}
        value={id}
        selectedLabel={labelFromValue}
        fetcher={async (q) => {
          if (!profileID) return [];
          const res = await glpiAPI.searchDropdown(itemtype, profileID, q);
          return res.items.map(
            (r) =>
              ({
                id: r.id,
                label: r.name,
                // For hierarchical itemtypes (ITILCategory, Location), the
                // catalogue may provide a full path like "A > B > C" — show it
                // as a secondary line so users can disambiguate same-named
                // leaves under different parents.
                secondary: r.completename && r.completename !== r.name ? r.completename : undefined,
              } as AsyncPickerItem)
          );
        }}
        onChange={(item) => onChange(item ? `${item.id}|${item.label}` : "")}
        error={error}
        placeholder={`Buscar ${itemtype}…`}
        disabled={!profileID}
      />
      <button
        type="button"
        onClick={() => setManual(true)}
        className="text-[11px] text-[var(--text-muted)] hover:text-[var(--accent)] hover:underline"
      >
        Digitar ID manualmente
      </button>
    </div>
  );
}
