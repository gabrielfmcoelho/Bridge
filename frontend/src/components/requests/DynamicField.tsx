"use client";

import { useState } from "react";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import Select from "@/components/ui/Select";
import Checkbox from "@/components/ui/Checkbox";
import DateTimeInput from "@/components/ui/DateTimeInput";
import TagInput from "@/components/ui/TagInput";
import AsyncPicker from "@/components/ui/AsyncPicker";
import FormError from "@/components/ui/FormError";
import { useLocale } from "@/contexts/LocaleContext";
import { hostsAPI, dnsAPI, servicesAPI, projectsAPI, toolsAPI, apiCatalogAPI } from "@/lib/api";
import type { FormField } from "@/lib/types";

// asset_ref pickers: compose the existing per-asset list() endpoints rather
// than inventing a generic "search assets by type" API. AssetType values with
// no obvious pick source (contact, ssh_key, secret, offering) have no entry
// here and fall through to the plain-Input fallback below.
const ASSET_PICKERS: Partial<Record<string, () => Promise<{ id: number; label: string }[]>>> = {
  host: () => hostsAPI.list().then((rows) => rows.map((r) => ({ id: r.id, label: r.nickname }))),
  dns: () => dnsAPI.list().then((rows) => rows.map((r) => ({ id: r.id, label: r.domain }))),
  service: () => servicesAPI.list().then((rows) => rows.map((r) => ({ id: r.id, label: r.nickname }))),
  project: () => projectsAPI.list().then((rows) => rows.map((r) => ({ id: r.id, label: r.name }))),
  tool: () => toolsAPI.list().then((rows) => rows.map((r) => ({ id: r.id, label: r.name }))),
  api_catalog: () => apiCatalogAPI.list().then((rows) => rows.map((r) => ({ id: r.id, label: r.name }))),
};

interface DynamicFieldProps {
  field: FormField;
  value: unknown;
  onChange: (value: unknown) => void;
  error?: string;
}

// Renders exactly one FormField descriptor — the offering's form_schema is
// authored data, not code, so this is the one place that turns a field
// descriptor into a control. Every control below is an existing src/components/ui
// primitive; this file only picks which one and wires bilingual label/help.
export default function DynamicField({ field, value, onChange, error }: DynamicFieldProps) {
  const { locale, t } = useLocale();
  const label = locale === "pt-BR" ? field.label_pt : field.label_en;
  const help = locale === "pt-BR" ? field.help_pt : field.help_en;
  // The format rule, stated to the user instead of only enforced on submit.
  const patternHint = locale === "pt-BR" ? field.pattern_hint_pt : field.pattern_hint_en;
  const placeholder = locale === "pt-BR" ? field.placeholder_pt : field.placeholder_en;

  // Remembers the display label of whatever asset_ref selection was made this
  // session, since AsyncPicker's single-select mode stores only the numeric
  // id in form_data and needs a label to redraw the trigger.
  const [pickedLabel, setPickedLabel] = useState("");

  const wrap = (control: React.ReactNode, opts?: { skipLabel?: boolean }) => (
    <div className="space-y-1.5">
      {!opts?.skipLabel && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
          {field.required && <span className="text-[var(--danger)] ml-0.5">*</span>}
        </label>
      )}
      {control}
      {help && <p className="text-xs text-[var(--text-muted)]">{help}</p>}
      {patternHint && <p className="text-xs text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>{patternHint}</p>}
      {error && <FormError message={error} />}
    </div>
  );

  switch (field.type) {
    case "text":
      return wrap(
        <Input
          value={(value as string) ?? ""}
          onChange={(e) => onChange(e.target.value)}
          maxLength={field.max_length}
          pattern={field.pattern}
          placeholder={placeholder}
        />
      );

    case "textarea":
      return wrap(
        <Textarea
          value={(value as string) ?? ""}
          onChange={(e) => onChange(e.target.value)}
          maxLength={field.max_length}
          rows={4}
          placeholder={placeholder}
        />
      );

    case "number":
      return wrap(
        <Input
          type="number"
          min={field.min}
          max={field.max}
          placeholder={placeholder}
          value={value === "" || value == null ? "" : String(value)}
          onChange={(e) => onChange(e.target.value === "" ? "" : Number(e.target.value))}
        />
      );

    case "select":
      return wrap(
        <Select
          value={(value as string) ?? ""}
          onChange={(e) => onChange(e.target.value)}
          options={(field.options ?? []).map((o) => ({ value: o, label: o }))}
        />
      );

    case "checkbox":
      // Checkbox's own label sits beside the box, matching how every other
      // checkbox in this app reads — a header label above it would be
      // redundant here.
      return wrap(
        <Checkbox label={label} checked={Boolean(value)} onChange={(checked) => onChange(checked)} />,
        { skipLabel: true }
      );

    case "date":
      return wrap(<DateTimeInput variant="date" value={(value as string) ?? ""} onChange={onChange} />);

    case "tags":
      // suggestions=[] skips TagInput's own tag-suggestion fetch. Task A5
      // has since added GET /api/tags to the mock, so the endpoint exists —
      // but the app's asset-tagging vocabulary (host/service/dns/project
      // tags) still isn't a meaningful autocomplete source for an arbitrary
      // offering-authored tags field, so this stays empty on purpose.
      return wrap(<TagInput tags={Array.isArray(value) ? (value as string[]) : []} onChange={onChange} suggestions={[]} />);

    case "asset_ref": {
      const listFn = field.asset_type ? ASSET_PICKERS[field.asset_type] : undefined;
      if (!listFn) {
        // No known list source for this asset type — fall back to a plain
        // text input rather than inventing a new picker API.
        return wrap(<Input value={(value as string) ?? ""} onChange={(e) => onChange(e.target.value)} />);
      }
      const idValue = typeof value === "number" ? value : null;
      return wrap(
        <AsyncPicker
          value={idValue}
          selectedLabel={pickedLabel}
          onChange={(item) => {
            setPickedLabel(item?.label ?? "");
            onChange(item?.id ?? null);
          }}
          fetcher={(query) =>
            listFn().then((rows) =>
              query ? rows.filter((r) => r.label.toLowerCase().includes(query.toLowerCase())) : rows
            )
          }
          emptyLabel={t("catalog.empty")}
        />
      );
    }

    default:
      return null;
  }
}
