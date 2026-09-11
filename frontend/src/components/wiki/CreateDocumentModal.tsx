"use client";

import { useState } from "react";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormFooter from "@/components/ui/FormFooter";
import Input from "@/components/ui/Input";
import { useLocale } from "@/contexts/LocaleContext";

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (title: string) => Promise<void>;
  submitting: boolean;
  defaultTitle?: string;
}

export default function CreateDocumentModal({ open, onClose, onSubmit, submitting, defaultTitle = "" }: Props) {
  const { t } = useLocale();
  const [title, setTitle] = useState(defaultTitle);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    const trimmed = title.trim();
    if (!trimmed) {
      setError(t("wiki.titleRequired"));
      return;
    }
    setError(null);
    try {
      await onSubmit(trimmed);
      setTitle("");
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("wiki.failedToCreate"));
    }
  };

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title={t("wiki.newPageTitle")}
      footer={
        <FormFooter onCancel={onClose} cancelLabel={t("common.cancel")} submitLabel={t("common.create")} onSubmit={handleSubmit} loading={submitting} disabled={!title.trim()} />
      }
    >
      <div className="space-y-4">
        <Input
          label={t("common.title")}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={t("wiki.titlePlaceholderExample")}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              handleSubmit();
            }
          }}
          autoFocus
        />
        <p className="text-xs text-[var(--text-muted)]">
          {t("wiki.createPageHint")}
        </p>
        {error && <p className="text-xs text-[var(--danger)]">{error}</p>}
      </div>
    </ResponsiveModal>
  );
}
