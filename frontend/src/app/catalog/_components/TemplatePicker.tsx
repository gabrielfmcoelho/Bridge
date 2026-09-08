"use client";

import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import type { FormField, OfferingTemplate } from "@/lib/types";

// Presets over the offering's own form_schema. "How big should it be?" is the
// question a requester is least equipped to answer, so the offering answers it
// with worked examples and the user adjusts. Custom is a first-class option,
// not a fallback — picking it clears the preset and leaves the form as authored.

/** Renders a template's values using the schema's own labels, so the row shows
 *  what it will actually set rather than a marketing line. */
function specLine(fields: FormField[], template: OfferingTemplate, locale: string): string {
  return Object.entries(template.values)
    .map(([key, value]) => {
      const f = fields.find((x) => x.key === key);
      if (!f) return null;
      const label = locale === "pt-BR" ? f.label_pt : f.label_en;
      return `${label}: ${String(value)}`;
    })
    .filter(Boolean)
    .join("  ·  ");
}

export default function TemplatePicker({
  templates,
  fields,
  selected,
  onSelect,
}: {
  templates: OfferingTemplate[];
  fields: FormField[];
  /** null means the custom path. */
  selected: string | null;
  onSelect: (key: string | null) => void;
}) {
  const { t, locale } = useLocale();

  const row = (active: boolean) =>
    `w-full rounded-[var(--radius-md)] border p-3 text-left transition-[border-color,background-color,transform] duration-200 ease-[cubic-bezier(0.16,1,0.3,1)] active:scale-[0.995] ${
      active
        ? "border-[var(--accent)] bg-[var(--accent-muted)]"
        : "border-[var(--border-subtle)] bg-[var(--bg-surface)] hover:border-[var(--border-strong)]"
    }`;

  return (
    <div className="space-y-2">
      {templates.map((tpl) => {
        const active = selected === tpl.key;
        const spec = specLine(fields, tpl, locale);
        return (
          <button key={tpl.key} type="button" aria-pressed={active} className={row(active)} onClick={() => onSelect(tpl.key)}>
            <span className="flex items-start gap-2.5">
              <span className={`mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border ${active ? "border-[var(--accent)] bg-[var(--accent)] text-white" : "border-[var(--border-default)]"}`}>
                {active && <Icon path={ICON_PATHS.checkCircle} className="h-2.5 w-2.5" strokeWidth={3} />}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-[var(--text-primary)]">
                  {locale === "pt-BR" ? tpl.name_pt : tpl.name_en}
                </span>
                <span className="mt-0.5 block text-xs leading-relaxed text-[var(--text-muted)]">
                  {locale === "pt-BR" ? tpl.summary_pt : tpl.summary_en}
                </span>
                {spec && (
                  <span className="mt-1.5 block text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
                    {spec}
                  </span>
                )}
              </span>
            </span>
          </button>
        );
      })}

      <button type="button" aria-pressed={selected === null} className={row(selected === null)} onClick={() => onSelect(null)}>
        <span className="flex items-start gap-2.5">
          <span className={`mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border ${selected === null ? "border-[var(--accent)] bg-[var(--accent)] text-white" : "border-[var(--border-default)]"}`}>
            {selected === null && <Icon path={ICON_PATHS.checkCircle} className="h-2.5 w-2.5" strokeWidth={3} />}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-sm font-medium text-[var(--text-primary)]">{t("catalog.templates.custom")}</span>
            <span className="mt-0.5 block text-xs leading-relaxed text-[var(--text-muted)]">{t("catalog.templates.customHint")}</span>
          </span>
        </span>
      </button>
    </div>
  );
}
