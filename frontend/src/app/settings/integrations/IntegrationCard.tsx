"use client";

import type { ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import Toggle from "@/components/ui/Toggle";
import Button from "@/components/ui/Button";
import StatusAlert from "@/components/ui/StatusAlert";
import type { IntegrationForm } from "./useIntegrationForm";

/**
 * The shell every integration section renders in (a SectionCard): heading + hint, an optional
 * enable toggle (bound to `enabledKey`; the body only shows while it is on),
 * and a save bar that is always there — so switching an integration off can
 * itself be saved.
 */
export default function IntegrationCard({
  form: f,
  title,
  hint,
  enabledKey,
  toggleLabel,
  children,
}: {
  form: IntegrationForm;
  title: string;
  hint?: string;
  enabledKey?: string;
  toggleLabel?: string;
  children: ReactNode;
}) {
  const { t } = useLocale();
  const enabled = !enabledKey || f.form[enabledKey] === "true";
  const status = f.dirty
    ? t("settings.integrations.unsavedChanges")
    : f.saved
      ? t("settings.integrations.saved")
      : t("settings.integrations.noChanges");

  // Flush body with its own padding: a switched-off integration shows only
  // header + save bar, with no empty padded strip between them.
  const showBody = enabled || !!f.saveError;
  return (
    <SectionCard
      as="h3"
      title={title}
      description={hint}
      body="flush"
      controls={enabledKey && (
        <Toggle
          checked={enabled}
          onChange={(v) => f.set(enabledKey, v ? "true" : "false")}
          ariaLabel={toggleLabel}
        />
      )}
      footer={
        <div className="flex items-center justify-between gap-3">
          <span className="text-xs text-[var(--text-muted)]" aria-live="polite">{status}</span>
          <div className="flex gap-2">
            <Button type="button" variant="secondary" onClick={f.reset} disabled={!f.dirty || f.saving}>
              {t("common.cancel")}
            </Button>
            <Button type="button" onClick={f.save} loading={f.saving} disabled={!f.dirty}>
              {t("settings.integrations.saveChanges")}
            </Button>
          </div>
        </div>
      }
    >
      {showBody && (
        <div className="p-5 space-y-4">
          {enabled && children}
          {f.saveError && <StatusAlert variant="error">{f.saveError.message}</StatusAlert>}
        </div>
      )}
    </SectionCard>
  );
}
