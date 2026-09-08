"use client";

import { useMemo, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { entidadesAPI, offeringsAPI } from "@/lib/api";
import { indentedLabel, withDepth } from "@/lib/entidades";
import { emptyFormValues, visibleFields } from "@/lib/requests";
import { parseFormSchema, parseTemplates, type SchemaIssue } from "@/lib/offeringSchema";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import type { AssetGrantsInput, FormField, Offering, RequestType } from "@/lib/types";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Textarea from "@/components/ui/Textarea";
import Toggle from "@/components/ui/Toggle";
import TagInput from "@/components/ui/TagInput";
import FormError from "@/components/ui/FormError";
import DynamicField from "@/components/requests/DynamicField";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import TemplatePicker from "@/app/catalog/_components/TemplatePicker";

const REQUEST_TYPES: RequestType[] = ["vm", "service", "dns", "api_token", "feature", "account", "support"];
export const GLPI_MODE_KEY: Record<Offering["glpi_mode"], string> = {
  inherit: "settings.offerings.glpiModeInherit",
  never: "settings.offerings.glpiModeNever",
  always: "settings.offerings.glpiModeAlways",
};

type Form = {
  name: string; slug: string; category: string; description: string;
  request_type: RequestType; approver_entidade_id: number | null;
  glpi_mode: Offering["glpi_mode"]; is_active: boolean; sort_order: string; use_cases: string[];
};

const fromOffering = (o: Offering | null): Form => ({
  name: o?.name ?? "", slug: o?.slug ?? "", category: o?.category ?? "", description: o?.description ?? "",
  request_type: o?.request_type ?? "service", approver_entidade_id: o?.approver_entidade_id ?? null,
  glpi_mode: o?.glpi_mode ?? "inherit", is_active: o?.is_active ?? true,
  sort_order: String(o?.sort_order ?? 0), use_cases: o?.use_cases ?? [],
});

const TEXTAREA_CLASS =
  "w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-[12px] focus:outline-none focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)]";

// Create/edit one offering. form_schema and templates are edited as the JSON
// they are stored as — the parser (lib/offeringSchema) decides what is valid
// and the right-hand preview shows the form a requester would actually see, so
// a typo is caught by eye or by code before it reaches anyone.
export default function OfferingFormModal({ offering, categories, onClose }: {
  offering: Offering | null;
  categories: string[];
  onClose: () => void;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const { data: entidades = [] } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list });

  const [form, setForm] = useState<Form>(() => fromOffering(offering));
  const [schemaText, setSchemaText] = useState(() => (offering ? JSON.stringify(offering.form_schema, null, 2) : ""));
  const [templatesText, setTemplatesText] = useState(() => (offering?.templates?.length ? JSON.stringify(offering.templates, null, 2) : ""));
  const [grants, setGrants] = useState<AssetGrantsInput>(() => offering?.entidades ?? defaultGrants(user));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");

  const schema = useMemo(() => parseFormSchema(schemaText), [schemaText]);
  const fields: FormField[] = useMemo(() => schema.value?.fields ?? [], [schema]);
  // While the schema itself is broken there is no field set to check template
  // values against — reporting every value as "unknown" would just be noise on
  // top of the schema error that already blocks saving.
  const templates = useMemo(
    () => (schema.value ? parseTemplates(templatesText, fields) : { value: [], errors: [] }),
    [schema.value, templatesText, fields],
  );

  // Preview state lives here, keyed off the parsed fields: a schema edit that
  // renames a key must not leave a stale answer behind under the old one.
  const [preview, setPreview] = useState<{ fieldsKey: string; values: Record<string, unknown>; template: string | null }>({ fieldsKey: "", values: {}, template: null });
  const fieldsKey = fields.map((f) => f.key).join("|");
  const previewValues = preview.fieldsKey === fieldsKey ? preview.values : emptyFormValues(fields);
  const setPreviewValue = (key: string, v: unknown) =>
    setPreview({ fieldsKey, values: { ...previewValues, [key]: v }, template: null });
  const applyTemplate = (key: string | null) => {
    const tpl = templates.value.find((x) => x.key === key);
    setPreview({ fieldsKey, values: { ...emptyFormValues(fields), ...(tpl?.values ?? {}) }, template: key });
  };

  const set = <K extends keyof Form>(k: K, v: Form[K]) => setForm((f) => ({ ...f, [k]: v }));
  const issueText = (i: SchemaIssue) =>
    t(`settings.offerings.schemaErr.${i.code}`, { n: String(i.n ?? ""), key: i.key ?? "", value: i.value ?? "" });

  const save = useMutation({
    mutationFn: () => {
      const payload = {
        ...form,
        slug: form.slug.trim() || undefined,
        sort_order: Number(form.sort_order),
        form_schema: schema.value ?? { fields: [] },
        templates: templates.value,
        ...grants,
      };
      return offering ? offeringsAPI.update(offering.id, payload) : offeringsAPI.create(payload);
    },
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["offerings"] }); onClose(); },
    onError: (err: unknown) => setSubmitError(err instanceof Error ? err.message : "Failed"),
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const next: Record<string, string> = {};
    if (!form.name.trim()) next.name = t("settings.offerings.errNameRequired");
    if (form.approver_entidade_id == null) next.approver = t("settings.offerings.errApproverRequired");
    if (!/^\d+$/.test(form.sort_order.trim())) next.sort_order = t("settings.offerings.errSortOrder");
    if (!schema.value) next.schema = t("settings.offerings.errSchema");
    if (templates.errors.length) next.templates = t("settings.offerings.errTemplates");
    setErrors(next);
    setSubmitError("");
    if (Object.keys(next).length === 0) save.mutate();
  };

  const nodes = withDepth(entidades);
  const shownFields = visibleFields(fields, previewValues);

  return (
    <ResponsiveModal open onClose={onClose} title={offering ? t("settings.offerings.edit") : t("settings.offerings.add")}>
      <form onSubmit={submit} className="space-y-6">
        <FormError message={submitError} />

        {/* ── Basics ─────────────────────────────────────────────────────── */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <Input label={`${t("settings.offerings.name")} *`} value={form.name} onChange={(e) => set("name", e.target.value)} error={errors.name} />
          <Input label={t("settings.offerings.slug")} value={form.slug} onChange={(e) => set("slug", e.target.value)} placeholder={t("settings.offerings.slugAuto")} />
          <div>
            <Input label={t("settings.offerings.category")} list="offering-categories" value={form.category} onChange={(e) => set("category", e.target.value)} placeholder={t("settings.offerings.categoryPlaceholder")} />
            <datalist id="offering-categories">{categories.map((c) => <option key={c} value={c} />)}</datalist>
          </div>
          <Select
            label={t("settings.offerings.requestType")}
            value={form.request_type}
            onChange={(e) => set("request_type", e.target.value as RequestType)}
            options={REQUEST_TYPES.map((rt) => ({ value: rt, label: t(`catalog.requestType.${rt}`) }))}
          />
          <div>
            <Select
              label={`${t("settings.offerings.approver")} *`}
              value={form.approver_entidade_id != null ? String(form.approver_entidade_id) : ""}
              onChange={(e) => set("approver_entidade_id", e.target.value ? Number(e.target.value) : null)}
              options={[{ value: "", label: "—" }, ...nodes.map((n) => ({ value: String(n.id), label: indentedLabel(n) }))]}
              error={errors.approver}
            />
            <p className="mt-1 text-xs text-[var(--text-faint)]">{t("settings.offerings.approverHint")}</p>
          </div>
          <Select
            label={t("settings.offerings.glpiMode")}
            value={form.glpi_mode}
            onChange={(e) => set("glpi_mode", e.target.value as Offering["glpi_mode"])}
            options={(Object.keys(GLPI_MODE_KEY) as Offering["glpi_mode"][]).map((m) => ({ value: m, label: t(GLPI_MODE_KEY[m]) }))}
          />
          <Input label={t("settings.offerings.sortOrder")} type="number" min={0} step={1} value={form.sort_order} onChange={(e) => set("sort_order", e.target.value)} error={errors.sort_order} />
          <label className="flex items-center gap-3 self-end pb-2 text-sm text-[var(--text-secondary)]">
            <Toggle checked={form.is_active} onChange={(v) => set("is_active", v)} ariaLabel={t("settings.offerings.active")} />
            {t("settings.offerings.active")}
          </label>
          <div className="md:col-span-2">
            <Textarea label={t("settings.offerings.description")} rows={2} value={form.description} onChange={(e) => set("description", e.target.value)} />
          </div>
          <div className="md:col-span-2">
            <TagInput label={t("settings.offerings.useCases")} tags={form.use_cases} onChange={(v) => set("use_cases", v)} />
            <p className="mt-1 text-xs text-[var(--text-faint)]">{t("settings.offerings.useCasesHint")}</p>
          </div>
        </div>

        {/* ── Scope ──────────────────────────────────────────────────────── */}
        <section>
          <h4 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-2">{t("settings.offerings.scope")}</h4>
          <EntidadeScopeFields value={grants} onChange={setGrants} compact />
        </section>

        {/* ── Form schema + live preview ─────────────────────────────────── */}
        <section className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1">{t("settings.offerings.schema")}</label>
            <textarea
              value={schemaText}
              onChange={(e) => setSchemaText(e.target.value)}
              rows={16}
              spellCheck={false}
              className={`${TEXTAREA_CLASS} ${errors.schema ? "border-[var(--danger)]" : ""}`}
              style={{ fontFamily: "var(--font-mono)" }}
              placeholder={t("settings.offerings.schemaHint")}
            />
            <IssueList issues={schema.errors} okText={schema.value ? t("settings.offerings.schemaValid", { n: String(fields.length) }) : null} render={issueText} />
            {errors.schema && <p className="text-xs text-[var(--danger)] mt-1">{errors.schema}</p>}

            <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mt-4 mb-1">{t("settings.offerings.templates")}</label>
            <textarea
              value={templatesText}
              onChange={(e) => setTemplatesText(e.target.value)}
              rows={8}
              spellCheck={false}
              className={`${TEXTAREA_CLASS} ${errors.templates ? "border-[var(--danger)]" : ""}`}
              style={{ fontFamily: "var(--font-mono)" }}
              placeholder={t("settings.offerings.templatesHint")}
            />
            <IssueList issues={templates.errors} okText={templates.value.length ? t("settings.offerings.templatesValid", { n: String(templates.value.length) }) : null} render={issueText} />
            {errors.templates && <p className="text-xs text-[var(--danger)] mt-1">{errors.templates}</p>}
          </div>

          <div className="rounded-[var(--radius-lg)] border border-dashed border-[var(--border-default)] p-4 space-y-4 min-h-40">
            <h4 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t("settings.offerings.schemaPreview")}</h4>
            {fields.length === 0 ? (
              <p className="text-sm text-[var(--text-faint)]">{t("settings.offerings.schemaPreviewEmpty")}</p>
            ) : (
              <>
                {templates.value.length > 0 && (
                  <TemplatePicker templates={templates.value} fields={fields} selected={preview.template} onSelect={applyTemplate} />
                )}
                {shownFields.map((field) => (
                  <DynamicField key={field.key} field={field} value={previewValues[field.key]} onChange={(v) => setPreviewValue(field.key, v)} />
                ))}
              </>
            )}
          </div>
        </section>

        <div className="flex justify-end gap-2 pt-2 border-t border-[var(--border-subtle)]">
          <Button type="button" variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button type="submit" loading={save.isPending}>{offering ? t("common.save") : t("common.create")}</Button>
        </div>
      </form>
    </ResponsiveModal>
  );
}

function IssueList({ issues, okText, render }: { issues: SchemaIssue[]; okText: string | null; render: (i: SchemaIssue) => string }) {
  if (issues.length) {
    return (
      <ul className="mt-1 space-y-0.5 text-xs text-[var(--danger)]">
        {issues.map((i, idx) => <li key={idx}>⚠ {render(i)}</li>)}
      </ul>
    );
  }
  if (okText) return <p className="mt-1 text-xs text-[var(--success)]">✓ {okText}</p>;
  return null;
}
