"use client";

import { useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { contactsAPI, enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import Select from "@/components/ui/Select";
import Checkbox from "@/components/ui/Checkbox";
import EntityFormShell from "@/components/forms/EntityFormShell";
import FormSection from "@/components/forms/FormSection";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import { formatPhone, phoneDigits, phoneProblem } from "@/lib/phone";
import type { AssetGrantsInput, Contact } from "@/lib/types";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** The API refuses a duplicate (name, phone) with a code; say it in words. */
export function contactErrorMessage(err: unknown, t: (k: string) => string): string {
  const msg = err instanceof Error ? err.message : "";
  if (msg === "contact_exists") return t("contact.exists");
  if (msg === "contact_in_trash") return t("contact.existsInTrash");
  return msg || t("form.saveFailed");
}

/**
 * Create/edit a contact in the sectioned drawer form the inventory uses:
 * who they are, how to reach them, who sees the record.
 */
export default function ContactForm({ initial, onSaved, onCancel, onSubHeaderChange, onFooterChange }: {
  initial: Contact | null;
  onSaved: (c: Contact) => void;
  onCancel: () => void;
  onSubHeaderChange?: (n: ReactNode) => void;
  onFooterChange?: (n: ReactNode) => void;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isEdit = !!initial;
  // Edit: empty grants = leave as they are (EntidadeScopeFields loads them).
  const [grants, setGrants] = useState<AssetGrantsInput>(initial ? {} : defaultGrants(user));
  const [f, setF] = useState({
    name: initial?.name ?? "", phone: initial?.phone ?? "", email: initial?.email ?? "",
    role: initial?.role ?? "", entity: initial?.entity ?? "", notes: initial?.notes ?? "",
    is_external: initial?.is_external ?? false,
  });
  const set = <K extends keyof typeof f>(k: K, v: (typeof f)[K]) => setF((x) => ({ ...x, [k]: v }));
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const { data: areas = [] } = useQuery({ queryKey: ["enums", "entidade_responsavel"], queryFn: () => enumsAPI.list("entidade_responsavel") });

  const save = useMutation({
    mutationFn: () => {
      const payload = { ...f, name: f.name.trim(), email: f.email.trim(), ...grants };
      return initial ? contactsAPI.update(initial.id, payload) : contactsAPI.create(payload);
    },
    onSuccess: (c) => onSaved(c),
    onError: (e) => setError(contactErrorMessage(e, t)),
  });

  const errors: Record<string, string> = {};
  if (!f.name.trim()) errors.name = t("form.required");
  if (phoneProblem(f.phone)) errors.phone = t(phoneProblem(f.phone));
  if (f.email.trim() && !EMAIL_RE.test(f.email.trim())) errors.email = t("contact.emailInvalid");
  const err = (k: string) => (attempted ? errors[k] : undefined);

  return (
    <EntityFormShell
      id="contact-form"
      sections={[
        { id: "ct-who", label: t("contact.sectionWho") },
        { id: "ct-reach", label: t("contact.sectionReach") },
        { id: "ct-visibility", label: t("contact.sectionVisibility") },
      ]}
      isEdit={isEdit}
      isPending={save.isPending}
      submitLabel={isEdit ? t("form.saveChanges") : t("contact.create")}
      error={error}
      onSubmit={() => {
        setAttempted(true);
        if (Object.keys(errors).length) return false;
        setError("");
        save.mutate();
      }}
      onCancel={onCancel}
      onSubHeaderChange={onSubHeaderChange}
      onFooterChange={onFooterChange}
    >
      <FormSection id="ct-who" title={t("contact.sectionWho")}>
        <div className="sm:col-span-2">
          <Input label={t("responsavel.name")} required value={f.name} onChange={(e) => set("name", e.target.value)}
            error={err("name")} aria-invalid={!!err("name")} />
        </div>
        <Input label={t("responsavel.role")} value={f.role} onChange={(e) => set("role", e.target.value)} placeholder={t("contact.rolePlaceholder")} />
        <Select label={t("contact.area")} value={f.entity} hint={t("contact.areaHint")}
          onChange={(e) => set("entity", e.target.value)}
          options={areas.map((a) => ({ value: a.value, label: a.value }))} />
        <div className="sm:col-span-2">
          <Checkbox label={t("contact.externalLabel")} checked={f.is_external} onChange={(v) => set("is_external", v)} />
        </div>
      </FormSection>

      <FormSection id="ct-reach" title={t("contact.sectionReach")}>
        <Input label={t("responsavel.phone")} type="tel" inputMode="tel" value={formatPhone(f.phone)}
          onChange={(e) => set("phone", phoneDigits(e.target.value))} placeholder="(86) 9 9999-9999"
          hint={t("contact.phoneHint")} error={err("phone")} aria-invalid={!!err("phone")} />
        <Input label={t("contact.email")} type="email" value={f.email} onChange={(e) => set("email", e.target.value)}
          placeholder="nome@orgao.pi.gov.br" error={err("email")} aria-invalid={!!err("email")} />
        <div className="sm:col-span-2">
          <Textarea label={t("responsavel.notes")} rows={3} value={f.notes} onChange={(e) => set("notes", e.target.value)} />
        </div>
      </FormSection>

      <FormSection id="ct-visibility" title={t("contact.sectionVisibility")} description={t("contact.visibilityHint")} stack>
        <EntidadeScopeFields value={grants} onChange={setGrants} compact loadFrom={initial ? { type: "contact", id: initial.id } : null} />
      </FormSection>
    </EntityFormShell>
  );
}
