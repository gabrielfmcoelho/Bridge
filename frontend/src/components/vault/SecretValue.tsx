"use client";

import { useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { secretFields } from "@/lib/secretFields";
import type { UseSecretReveal } from "@/hooks/useSecretReveal";

/**
 * A revealed secret, readable: one labelled row per field (app, URL, user,
 * password…), each with its own copy button — keys in a scrolling mono box.
 * Copies go through the reveal hook, so the clipboard is cleared again.
 */
export default function SecretValue({ type, reveal }: { type: string; reveal: UseSecretReveal }) {
  const { t } = useLocale();
  const [copied, setCopied] = useState<string | null>(null);
  if (!reveal.revealed || reveal.value == null) return null;
  const fields = secretFields(type, reveal.value);

  return (
    <dl className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] divide-y divide-[var(--border-subtle)]">
      {fields.map((f) => {
        const label = t(`vault.field.${f.key}`) === `vault.field.${f.key}` ? f.key : t(`vault.field.${f.key}`);
        const copyBtn = (
          <IconButton label={copied === f.key && reveal.copyState === "copied" ? t("vault.copiedLabel") : t("vault.copyField", { field: label })}
            onClick={() => { reveal.copy(f.value); setCopied(f.key); }}>
            <Icon path={copied === f.key && reveal.copyState === "copied" ? ICON_PATHS.check : ICON_PATHS.copy}
              className={copied === f.key && reveal.copyState === "copied" ? "text-[var(--success)]" : undefined} />
          </IconButton>
        );
        return (
          <div key={f.key} className="px-3 py-2">
            <dt className="text-xs text-[var(--text-muted)]">{label}</dt>
            {f.kind === "key" ? (
              <dd className="mt-1 flex items-start gap-2">
                <pre className="flex-1 min-w-0 max-h-40 overflow-auto text-xs font-mono text-[var(--text-secondary)] bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] p-2 whitespace-pre-wrap break-all">{f.value}</pre>
                {copyBtn}
              </dd>
            ) : (
              <dd className="flex items-center gap-2">
                {f.kind === "url" ? (
                  <a href={f.value} target="_blank" rel="noopener noreferrer" className="flex-1 min-w-0 truncate text-sm text-[var(--accent)] hover:underline">{f.value}</a>
                ) : (
                  <span className={`flex-1 min-w-0 text-sm text-[var(--text-primary)] break-all ${f.kind === "secret" ? "font-mono" : ""}`}>{f.value}</span>
                )}
                {copyBtn}
              </dd>
            )}
          </div>
        );
      })}
    </dl>
  );
}
