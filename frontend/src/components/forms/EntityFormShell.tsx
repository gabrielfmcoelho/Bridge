"use client";

import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import FormError from "@/components/ui/FormError";
import FormFooter from "@/components/ui/FormFooter";
import FormSectionNav from "./FormSectionNav";

export interface FormSectionRef { id: string; label: string }

/**
 * The one shell for an entity's create/edit form (host, DNS, serviço): a
 * single scrolling form in sections — no wizard — with a section index in the
 * drawer's sub-header, the error on top, and a Cancel + primary footer. Enter
 * submits; the first field is focused on create; native validation is off so
 * the form's own inline errors show instead of browser bubbles.
 *
 * `onSubmit` returns false when the form's own validation blocks the save;
 * the shell then scrolls to and focuses the first field marked invalid.
 */
export default function EntityFormShell({
  id, sections, isEdit, isPending, submitLabel, error, onSubmit, onCancel, onFooterChange, onSubHeaderChange, children,
}: {
  id: string;
  sections: FormSectionRef[];
  isEdit: boolean;
  isPending: boolean;
  submitLabel: string;
  error?: string;
  onSubmit: () => boolean | void;
  onCancel?: () => void;
  onFooterChange?: (footer: ReactNode) => void;
  onSubHeaderChange?: (subHeader: ReactNode) => void;
  children: ReactNode;
}) {
  const { t } = useLocale();
  const formRef = useRef<HTMLFormElement>(null);
  const [attempt, setAttempt] = useState(0);
  const onCancelRef = useRef(onCancel);
  onCancelRef.current = onCancel;

  // Section index above the scrolling body; footer outside it.
  const sectionKey = sections.map((s) => `${s.id}:${s.label}`).join("|");
  useEffect(() => {
    onSubHeaderChange?.(<FormSectionNav sections={sections} />);
    return () => onSubHeaderChange?.(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sectionKey stands for sections
  }, [sectionKey]);
  useEffect(() => {
    onFooterChange?.(
      <FormFooter
        onCancel={() => onCancelRef.current?.()}
        submitLabel={submitLabel}
        submitType="submit"
        form={id}
        loading={isPending}
      />,
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps -- rebuilt when the label or pending state changes
  }, [submitLabel, isPending, id]);

  useEffect(() => {
    if (isEdit) return;
    formRef.current?.querySelector<HTMLElement>("input:not([type=hidden]), select, textarea")?.focus();
  }, [isEdit]);

  // After a blocked submit, bring the first invalid field into view.
  useEffect(() => {
    if (!attempt) return;
    const bad = formRef.current?.querySelector<HTMLElement>("[aria-invalid=true]");
    bad?.scrollIntoView({ block: "center", behavior: "smooth" });
    bad?.focus({ preventScroll: true });
  }, [attempt]);

  const handle = (e: FormEvent) => {
    e.preventDefault();
    if (onSubmit() === false) setAttempt((n) => n + 1);
  };

  return (
    <form id={id} ref={formRef} onSubmit={handle} noValidate className="space-y-8" aria-label={t("form.sectionsLabel")}>
      <FormError message={error ?? ""} />
      {children}
    </form>
  );
}
