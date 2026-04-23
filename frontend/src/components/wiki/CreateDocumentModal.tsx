"use client";

import { useState } from "react";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
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
    <ResponsiveModal open={open} onClose={onClose} title="New page">
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
        <p className="text-[11px] text-[var(--text-muted)]">
          The page will be created in Outline and opened in a new tab so you can fill in the content there.
        </p>
        {error && <p className="text-xs text-red-400">{error}</p>}
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="button" onClick={handleSubmit} loading={submitting} disabled={!title.trim()}>
            Create
          </Button>
        </div>
      </div>
    </ResponsiveModal>
  );
}
