"use client";

import type { ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "./Button";

interface FormFooterProps {
  onCancel: () => void;
  cancelLabel?: string;
  submitLabel: string;
  onSubmit?: () => void;
  /** `submit` + `form` when the footer sits in a modal slot outside its `<form>`. */
  submitType?: "button" | "submit";
  form?: string;
  loading?: boolean;
  disabled?: boolean;
  variant?: "primary" | "danger";
  /** Left-aligned slot (a Back button, a delete link). */
  start?: ReactNode;
}

// The one action bar for Cancel/Confirm dialogs: cancel is always secondary
// and sits left of the primary, both right-aligned. Multi-step wizards keep
// their own full-width Back/Next pair from useMultiStepForm.
export default function FormFooter({
  onCancel,
  cancelLabel,
  submitLabel,
  onSubmit,
  submitType = "button",
  form,
  loading = false,
  disabled = false,
  variant = "primary",
  start,
}: FormFooterProps) {
  const { t } = useLocale();
  return (
    <div className="flex items-center gap-2">
      {start}
      <div className="ml-auto flex items-center gap-2">
        <Button type="button" variant="secondary" onClick={onCancel} disabled={loading}>
          {cancelLabel ?? t("common.cancel")}
        </Button>
        <Button type={submitType} form={form} variant={variant} onClick={onSubmit} loading={loading} disabled={disabled}>
          {submitLabel}
        </Button>
      </div>
    </div>
  );
}
