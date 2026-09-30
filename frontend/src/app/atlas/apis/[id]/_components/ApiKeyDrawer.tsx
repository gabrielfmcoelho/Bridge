"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiKeysAPI, contactsAPI } from "@/lib/api";
import type { ApiCatalog, ApiKey } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import Drawer from "@/components/ui/Drawer";
import FormFooter from "@/components/ui/FormFooter";
import FormError from "@/components/ui/FormError";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import Textarea from "@/components/ui/Textarea";

const EXPIRY_DAYS = [30, 90, 180, 365, 0]; // 0 = never
const LABEL_RE = /^[A-Za-z0-9._-]{1,64}$/;

// Issue (sead), register (manual) or edit a key. Editing only touches
// Bridge's metadata: a SEAD key's expiry and limits can't change remotely.
export default function ApiKeyDrawer({ api, editing, open, onClose, onIssued }: {
  api: ApiCatalog;
  editing: ApiKey | null;
  open: boolean;
  onClose: () => void;
  onIssued: (label: string, plaintext: string) => void;
}) {
  const { t } = useLocale();
  const qc = useQueryClient();
  const isEdit = !!editing;
  const manual = isEdit ? editing.source === "manual" : api.key_management === "manual";

  const [label, setLabel] = useState(editing?.label ?? "");
  const [owner, setOwner] = useState(editing?.owner ?? "");
  const [contactId, setContactId] = useState<number | null>(editing?.owner_contact_id ?? null);
  const [notes, setNotes] = useState(editing?.notes ?? "");
  const [expiresDays, setExpiresDays] = useState(90);
  const [expiresAt, setExpiresAt] = useState(editing?.expires_at?.slice(0, 10) ?? "");
  const [rateLimit, setRateLimit] = useState("");
  const [value, setValue] = useState("");
  const [header, setHeader] = useState("X-API-Key");
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const { data: contacts = [] } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list, enabled: open });

  const labelError = !isEdit && attempted && !LABEL_RE.test(label.trim()) ? t("atlas.apis.keys.labelInvalid") : undefined;
  const valueError = !isEdit && manual && attempted && !value.trim() ? t("atlas.apis.keys.valueRequired") : undefined;

  const save = useMutation({
    mutationFn: async () => {
      if (isEdit) {
        return apiKeysAPI.update(api.id, editing.id, {
          owner: owner.trim(),
          owner_contact_id: contactId,
          notes,
          expires_at: manual ? (expiresAt ? new Date(`${expiresAt}T23:59:59`).toISOString() : null) : editing.expires_at,
        }).then(() => null);
      }
      return apiKeysAPI.create(api.id, {
        label: label.trim(),
        owner: owner.trim(),
        owner_contact_id: contactId,
        notes,
        expires_days: expiresDays,
        rate_limit_per_minute: !manual && rateLimit.trim() ? Number(rateLimit) : undefined,
        value: manual ? value.trim() : undefined,
        header: manual ? header.trim() : undefined,
      });
    },
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["api-keys", api.id] });
      if (res) onIssued(res.key.label, res.plaintext);
      onClose();
    },
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  const submit = () => {
    setAttempted(true);
    setError("");
    if (!isEdit && (!LABEL_RE.test(label.trim()) || (manual && !value.trim()))) return;
    save.mutate();
  };

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={isEdit ? t("atlas.apis.keys.editTitle", { name: editing.label }) : t(manual ? "atlas.apis.keys.registerTitle" : "atlas.apis.keys.issueTitle")}
      footer={<FormFooter onCancel={onClose} submitLabel={isEdit ? t("common.save") : t(manual ? "atlas.apis.keys.register" : "atlas.apis.keys.issue")} onSubmit={submit} loading={save.isPending} />}
    >
      <div className="space-y-4">
        {!isEdit && (
          <Input
            label={t("atlas.apis.keys.label")}
            hint={t("atlas.apis.keys.labelHint")}
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="painel-rh"
            error={labelError}
            aria-invalid={!!labelError}
            required
          />
        )}
        {!isEdit && manual && (
          <>
            <Input
              label={t("atlas.apis.keys.value")}
              hint={t("atlas.apis.keys.valueHint")}
              type="password"
              autoComplete="off"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              error={valueError}
              aria-invalid={!!valueError}
              required
            />
            <Input label={t("atlas.apis.keys.header")} value={header} onChange={(e) => setHeader(e.target.value)} />
          </>
        )}
        <Input label={t("atlas.apis.keys.owner")} hint={t("atlas.apis.keys.ownerHint")} value={owner} onChange={(e) => setOwner(e.target.value)} />
        <NativeSelect
          label={t("atlas.apis.keys.ownerContact")}
          value={contactId ?? ""}
          onChange={(e) => setContactId(e.target.value ? Number(e.target.value) : null)}
        >
          <option value="">{t("atlas.apis.keys.noContact")}</option>
          {contacts.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </NativeSelect>
        {!isEdit && (
          <NativeSelect label={t("atlas.apis.keys.expiry")} value={expiresDays} onChange={(e) => setExpiresDays(Number(e.target.value))}>
            {EXPIRY_DAYS.map((d) => (
              <option key={d} value={d}>{d === 0 ? t("atlas.apis.keys.noExpiry") : t("atlas.apis.keys.days", { n: String(d) })}</option>
            ))}
          </NativeSelect>
        )}
        {isEdit && manual && (
          <Input label={t("atlas.apis.keys.expiresAt")} type="date" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
        )}
        {!isEdit && !manual && (
          <Input
            label={t("atlas.apis.keys.rateLimit")}
            hint={t("atlas.apis.keys.rateLimitHint")}
            type="number"
            min={0}
            value={rateLimit}
            onChange={(e) => setRateLimit(e.target.value)}
            placeholder="1000"
          />
        )}
        <Textarea label={t("atlas.apis.keys.notes")} value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} />
        {isEdit && !manual && <p className="text-xs text-[var(--text-muted)]">{t("atlas.apis.keys.seadEditHint")}</p>}
        <FormError message={error} />
      </div>
    </Drawer>
  );
}
