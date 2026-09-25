"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import Input from "@/components/ui/Input";
import Button from "@/components/ui/Button";
import { MASKED, type IntegrationForm } from "./useIntegrationForm";

// A masked input for one secret key of an integration form, plus Clear when the
// server holds a value. Clear goes through the dedicated DELETE endpoint so a
// stored cipher is only wiped on explicit admin intent — never as a side-effect
// of saving (the backend also ignores "" and the mask on PUT).
export default function SecretInputWithClear({ form: f, name, label }: { form: IntegrationForm; name: string; label: string }) {
  const confirm = useConfirm();
  const { t } = useLocale();
  return (
    <div className="flex items-end gap-3">
      <div className="flex-1 min-w-0">
        <Input
          label={label}
          type="password"
          autoComplete="new-password"
          value={f.form[name] ?? ""}
          onChange={(e) => f.set(name, e.target.value)}
          placeholder={MASKED}
        />
      </div>
      {f.isStored(name) && (
        <Button
          type="button"
          variant="danger"
          size="sm"
          className="mb-1"
          onClick={async () => { if (await confirm({ title: t("confirm.clearSecretTitle"), message: t("settings.integrations.confirmClearSecret", { label }), danger: true, confirmLabel: t("settings.integrations.clearStored") })) f.clearSecret(name); }}
        >
          {t("settings.integrations.clearStored")}
        </Button>
      )}
    </div>
  );
}
