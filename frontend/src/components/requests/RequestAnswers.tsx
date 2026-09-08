"use client";

import Link from "next/link";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import { assetHref, assetTypeLabelKey } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { FormField, FormSchema } from "@/lib/types";

// Read-only mirror of DynamicField: the same closed field vocabulary, rendered
// as answers rather than controls. Two consumers, which is why it sits here
// rather than inside a route: the request detail page renders it against
// form_schema_snapshot (the schema a request was submitted against, so an
// offering edited later never retro-changes how an old request reads), and the
// request form's review step renders it against the live schema so a user sees
// exactly what they are about to send.

function AnswerValue({ field, value }: { field: FormField; value: unknown }) {
  const { t, formatDate } = useLocale();

  if (field.type === "checkbox") {
    return <span>{value ? t("common.yes") : t("common.no")}</span>;
  }

  if (field.type === "tags") {
    const tags = Array.isArray(value) ? (value as string[]) : [];
    if (tags.length === 0) return <span className="text-[var(--text-faint)]">—</span>;
    return (
      <span className="flex flex-wrap gap-1.5">
        {tags.map((tag) => (
          <Badge key={tag} color="gray">{tag}</Badge>
        ))}
      </span>
    );
  }

  if (value === null || value === undefined || value === "") {
    return <span className="text-[var(--text-faint)]">—</span>;
  }

  if (field.type === "date") {
    return <span>{formatDate(String(value))}</span>;
  }

  if (field.type === "asset_ref" && typeof value === "number" && field.asset_type) {
    const href = assetHref(field.asset_type, value);
    const labelKey = assetTypeLabelKey(field.asset_type);
    const label = `${labelKey ? t(labelKey) : field.asset_type} #${value}`;
    return href ? (
      <Link href={href} className="text-[var(--accent)] hover:underline">{label}</Link>
    ) : (
      <span>{label}</span>
    );
  }

  return <span className="whitespace-pre-wrap break-words">{String(value)}</span>;
}

export default function RequestAnswers({
  schema,
  data,
}: {
  schema: FormSchema | undefined;
  data: Record<string, unknown>;
}) {
  const { t, locale } = useLocale();
  const fields = schema?.fields ?? [];

  if (fields.length === 0) {
    return <EmptyState icon="folder" title={t("requests.noAnswers")} compact />;
  }

  return (
    <dl className="space-y-3">
      {fields.map((field) => (
        <div key={field.key} className="grid grid-cols-1 sm:grid-cols-3 gap-1 sm:gap-3">
          <dt className="text-xs font-medium text-[var(--text-muted)] tracking-wide sm:pt-0.5">
            {locale === "pt-BR" ? field.label_pt : field.label_en}
          </dt>
          <dd className="sm:col-span-2 text-sm text-[var(--text-primary)]">
            <AnswerValue field={field} value={data[field.key]} />
          </dd>
        </div>
      ))}
    </dl>
  );
}
