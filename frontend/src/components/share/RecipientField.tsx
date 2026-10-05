"use client";

import { useQuery } from "@tanstack/react-query";
import { contactsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import NativeSelect from "@/components/ui/NativeSelect";
import Input from "@/components/ui/Input";

export interface Recipient {
  contactId: number | null;
  label: string;
}

export const EMPTY_RECIPIENT: Recipient = { contactId: null, label: "" };

/** The share request fields for a Recipient (both omitted when empty). */
export const recipientPayload = (r: Recipient) => ({
  recipient_contact_id: r.contactId ?? undefined,
  recipient_label: r.label.trim() || undefined,
});

/**
 * Who a share link is for: a contact, plus free text for when the recipient
 * isn't one (or to say which team). Same pattern as the API key's owner.
 */
export default function RecipientField({ value, onChange, enabled = true }: {
  value: Recipient;
  onChange: (r: Recipient) => void;
  /** Load contacts only while the form is open. */
  enabled?: boolean;
}) {
  const { t } = useLocale();
  const { data: contacts = [] } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list, enabled });
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <NativeSelect
        label={t("shares.recipientContact")}
        value={value.contactId ?? ""}
        onChange={(e) => onChange({ ...value, contactId: e.target.value ? Number(e.target.value) : null })}
      >
        <option value="">{t("shares.noContact")}</option>
        {contacts.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
      </NativeSelect>
      <Input
        label={t("shares.recipientLabel")}
        value={value.label}
        onChange={(e) => onChange({ ...value, label: e.target.value })}
        placeholder={t("shares.recipientLabelPlaceholder")}
      />
    </div>
  );
}
