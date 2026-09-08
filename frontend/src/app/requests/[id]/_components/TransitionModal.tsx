"use client";

import { useEffect, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Textarea from "@/components/ui/Textarea";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import { requestsAPI } from "@/lib/api";
import { TRANSITION_ACTION_KEY, transitionNeedsNote, transitionVariant, statusLabelKey } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { RequestStatus } from "@/lib/types";

interface TransitionModalProps {
  requestId: number;
  /** Target status, or null when closed. */
  to: RequestStatus | null;
  onClose: () => void;
  onDone: () => void;
}

// Confirms one status move and captures the note that goes with it. The note
// becomes the `status` event's body, which is what the timeline renders — so
// "why was this rejected?" is answerable from the request itself.
//
// ponytail: the `delivered` move can also carry fulfilled_asset_type/_id (the
// API and seed both support it), but collecting that needs an asset-type
// picker this task does not otherwise need. Delivering leaves the link empty;
// add the picker when someone asks to set it from the UI.
export default function TransitionModal({ requestId, to, onClose, onDone }: TransitionModalProps) {
  const { t } = useLocale();
  const [note, setNote] = useState("");
  const [noteError, setNoteError] = useState("");
  const [submitError, setSubmitError] = useState("");

  const open = to != null;
  const noteRequired = to != null && transitionNeedsNote(to);

  /* eslint-disable react-hooks/set-state-in-effect -- clearing local form
     state when a (possibly different) transition is opened, same pattern as
     RequestFormModal.tsx. */
  useEffect(() => {
    if (open) {
      setNote("");
      setNoteError("");
      setSubmitError("");
    }
  }, [open, to]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const mutation = useMutation({
    mutationFn: () => requestsAPI.transition(requestId, { to: to!, note: note.trim() || undefined }),
    onSuccess: () => {
      onDone();
      onClose();
    },
    onError: (err) => setSubmitError(err instanceof Error ? err.message : t("requests.transitionError")),
  });

  const handleSubmit = () => {
    setSubmitError("");
    if (noteRequired && !note.trim()) {
      setNoteError(t("requests.form.required"));
      return;
    }
    setNoteError("");
    mutation.mutate();
  };

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title={to ? t(TRANSITION_ACTION_KEY[to]) : ""}
      subHeader={
        to && (
          <p className="text-sm text-[var(--text-secondary)]">
            {t("requests.transitionTo", { status: t(statusLabelKey(to)) })}
          </p>
        )
      }
      footer={
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            variant={to ? transitionVariant(to) : "primary"}
            onClick={handleSubmit}
            loading={mutation.isPending}
            disabled={mutation.isPending}
          >
            {t("common.confirm")}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {submitError && <StatusAlert variant="error">{submitError}</StatusAlert>}
        <div className="space-y-1.5">
          <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
            {t("requests.transitionNote")}
            {noteRequired && <span className="text-[var(--danger)] ml-0.5">*</span>}
          </label>
          <Textarea
            value={note}
            onChange={(e) => {
              setNote(e.target.value);
              if (noteError) setNoteError("");
            }}
            rows={4}
          />
          {noteError && <FormError message={noteError} />}
        </div>
      </div>
    </ResponsiveModal>
  );
}
