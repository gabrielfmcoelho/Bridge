"use client";

import { useState } from "react";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormFooter from "@/components/ui/FormFooter";
import Input from "@/components/ui/Input";

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (title: string) => Promise<void>;
  submitting: boolean;
  defaultTitle?: string;
}

export default function CreateDocumentModal({ open, onClose, onSubmit, submitting, defaultTitle = "" }: Props) {
  const [title, setTitle] = useState(defaultTitle);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    const t = title.trim();
    if (!t) {
      setError("Title is required");
      return;
    }
    setError(null);
    try {
      await onSubmit(t);
      setTitle("");
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create");
    }
  };

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title="New page"
      footer={
        <FormFooter onCancel={onClose} cancelLabel="Cancel" submitLabel="Create" onSubmit={handleSubmit} loading={submitting} disabled={!title.trim()} />
      }
    >
      <div className="space-y-4">
        <Input
          label="Title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="e.g. Runbook: Database failover"
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              handleSubmit();
            }
          }}
          autoFocus
        />
        <p className="text-xs text-[var(--text-muted)]">
          The page will be created in Outline and opened in a new tab so you can fill in the content there.
        </p>
        {error && <p className="text-xs text-red-400">{error}</p>}
      </div>
    </ResponsiveModal>
  );
}
