"use client";

import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import Textarea from "@/components/ui/Textarea";
import Button from "@/components/ui/Button";
import StatusAlert from "@/components/ui/StatusAlert";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { aiAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";

export interface AiDraft {
  title: string;
  priority: string;
  form_data: Record<string, unknown>;
  filled_keys: string[];
}

// Optional assist on top of the hand-rolled form: describe the need in prose,
// get a draft answer set for whatever fields this offering declares.
//
// Renders NOTHING unless the instance actually has an LLM configured
// (/api/ai/status), so a Bridge install without AI sees no dead button. The
// form underneath is fully usable either way — this never becomes the only
// path to submitting a request.
export default function AiPrefillPanel({ offeringId, onDraft }: { offeringId: number; onDraft: (d: AiDraft) => void }) {
  const { t } = useLocale();
  const [description, setDescription] = useState("");
  const [notice, setNotice] = useState<{ kind: "success" | "warning" | "error"; text: string } | null>(null);

  const { data: status } = useQuery({
    queryKey: ["ai-status"],
    queryFn: () => aiAPI.status(),
    // Availability rarely changes mid-session and this gates a whole panel;
    // refetching it on every modal open would be pure noise.
    staleTime: 5 * 60 * 1000,
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: () => aiAPI.assistRequestForm(offeringId, description.trim()),
    onSuccess: (draft) => {
      const n = draft.filled_keys.length + (draft.title ? 1 : 0);
      if (n === 0) {
        setNotice({ kind: "warning", text: t("catalog.ai.none") });
        return;
      }
      onDraft(draft);
      setNotice({ kind: "success", text: t("catalog.ai.filled", { n: String(n) }) });
    },
    onError: (err) => setNotice({ kind: "error", text: err instanceof Error ? err.message : t("catalog.ai.error") }),
  });

  if (!status?.enabled || !status?.configured) return null;

  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] p-3.5 space-y-2.5">
      <div className="flex items-center gap-2">
        <Icon path={ICON_PATHS.bolt} className="h-3.5 w-3.5 text-[var(--accent)]" strokeWidth={2} />
        <p className="text-xs font-medium text-[var(--text-secondary)]">{t("catalog.ai.title")}</p>
        {status.model && (
          <span className="ml-auto truncate text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
            {status.model}
          </span>
        )}
      </div>

      <Textarea
        value={description}
        onChange={(e) => {
          setDescription(e.target.value);
          if (notice) setNotice(null);
        }}
        rows={2}
        placeholder={t("catalog.ai.placeholder")}
      />

      {notice && <StatusAlert variant={notice.kind === "warning" ? "warning" : notice.kind}>{notice.text}</StatusAlert>}

      <div className="flex items-center justify-between gap-3">
        <p className="text-[10px] leading-snug text-[var(--text-faint)]">{t("catalog.ai.caution")}</p>
        <Button
          type="button"
          size="sm"
          variant="secondary"
          onClick={() => mutation.mutate()}
          loading={mutation.isPending}
          disabled={!description.trim() || mutation.isPending}
        >
          {t("catalog.ai.action")}
        </Button>
      </div>
    </div>
  );
}
