"use client";

import Input from "@/components/ui/Input";
import IconButton from "@/components/ui/IconButton";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import type { ApiCatalogURL } from "@/lib/types";

const URL_RE = /^https?:\/\/\S+$/i;

/** Rows whose URL is present but not http(s). */
export function invalidUrlRows(rows: ApiCatalogURL[]): Set<number> {
  const bad = new Set<number>();
  rows.forEach((r, i) => {
    if (r.url.trim() && !URL_RE.test(r.url.trim())) bad.add(i);
  });
  return bad;
}

/** The rows worth sending: trimmed, without fully blank ones. */
export function cleanUrls(rows: ApiCatalogURL[]): ApiCatalogURL[] {
  return rows
    .map((r) => ({ label: r.label.trim(), url: r.url.trim() }))
    .filter((r) => r.label || r.url);
}

interface Props {
  value: ApiCatalogURL[];
  onChange: (rows: ApiCatalogURL[]) => void;
  /** Show per-row errors (after a submit attempt). */
  showErrors?: boolean;
}

/** The API's other addresses (gateway, origin…), each with a short label. */
export default function ApiUrlsEditor({ value, onChange, showErrors }: Props) {
  const { t } = useLocale();
  const bad = showErrors ? invalidUrlRows(value) : new Set<number>();
  const set = (i: number, patch: Partial<ApiCatalogURL>) => onChange(value.map((r, j) => (j === i ? { ...r, ...patch } : r)));

  return (
    <div className="space-y-2">
      {value.map((r, i) => (
        <div key={i} className="flex flex-wrap sm:flex-nowrap items-start gap-2">
          <div className="w-full sm:w-40 shrink-0">
            <Input aria-label={t("atlas.apis.urlLabel")} placeholder={t("atlas.apis.urlLabelPlaceholder")} value={r.label}
              onChange={(e) => set(i, { label: e.target.value })} />
          </div>
          <div className="flex-1 min-w-0">
            <Input aria-label="URL" type="url" className="font-mono" placeholder="https://api.example.com" value={r.url}
              onChange={(e) => set(i, { url: e.target.value })}
              error={bad.has(i) ? t("form.urlInvalid") : undefined} aria-invalid={bad.has(i)} />
          </div>
          <IconButton label={t("common.removeItem", { label: r.label || r.url || "URL" })} onClick={() => onChange(value.filter((_, j) => j !== i))}>
            <Icon path={ICON_PATHS.close} />
          </IconButton>
        </div>
      ))}
      <Button type="button" size="sm" variant="secondary" onClick={() => onChange([...value, { label: "", url: "" }])}>
        {t("atlas.apis.addUrl")}
      </Button>
    </div>
  );
}
