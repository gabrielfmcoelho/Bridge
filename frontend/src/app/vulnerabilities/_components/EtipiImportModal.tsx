"use client";

import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import Modal from "@/components/ui/Modal";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import FormError from "@/components/ui/FormError";
import FormFooter from "@/components/ui/FormFooter";
import Checkbox from "@/components/ui/Checkbox";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { vulnAPI } from "@/lib/api";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { VulnImportResult } from "@/lib/types";

const COUNTS = ["parsed", "new", "updated", "reopened", "resolved", "unmatched"] as const;

/** Upload an ETIPI report: a dry run shows what would change (above all, how
 *  many open findings it would resolve) before anything is written. */
export default function EtipiImportModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useLocale();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<VulnImportResult | null>(null);
  const [error, setError] = useState("");
  const [complete, setComplete] = useState(false);

  const reset = () => {
    setFile(null);
    setPreview(null);
    setError("");
    if (inputRef.current) inputRef.current.value = "";
  };
  const close = () => {
    reset();
    setComplete(false);
    onClose();
  };

  const dryRun = useMutation({
    mutationFn: ({ f, all }: { f: File; all: boolean }) => vulnAPI.importETIPI(f, { dryRun: true, complete: all }),
    onSuccess: setPreview,
    onError: (e: Error) => setError(e.message),
  });
  const apply = useMutation({
    mutationFn: () => vulnAPI.importETIPI(file!, { dryRun: false, complete }),
    onSuccess: (r) => {
      queryClient.invalidateQueries({ queryKey: ["vulnerabilities"] });
      flag({ appearance: "success", title: t("vuln.imported"), description: t("vuln.importSummary", { new: String(r.new), resolved: String(r.resolved), unmatched: String(r.unmatched) }) });
      close();
    },
    onError: (e: Error) => setError(e.message),
  });

  const pick = (f: File | undefined) => {
    if (!f) return;
    reset();
    setFile(f);
    dryRun.mutate({ f, all: complete });
  };
  // The preview must match what Import will do: re-run it when the scope changes.
  const toggleComplete = (all: boolean) => {
    setComplete(all);
    if (file) {
      setPreview(null);
      dryRun.mutate({ f: file, all });
    }
  };

  return (
    <Modal
      open={open}
      onClose={close}
      title={t("vuln.importEtipi")}
      footer={
        <FormFooter
          onCancel={close}
          submitLabel={t("vuln.importConfirm")}
          onSubmit={() => apply.mutate()}
          loading={apply.isPending}
          disabled={!preview}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-[var(--text-secondary)]">{t("vuln.importHint")}</p>
        <input ref={inputRef} type="file" accept=".txt,text/plain" className="hidden" onChange={(e) => pick(e.target.files?.[0])} />
        <button
          type="button"
          onClick={() => inputRef.current?.click()}
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => { e.preventDefault(); pick(e.dataTransfer.files?.[0]); }}
          className="w-full py-8 border-2 border-dashed border-[var(--border-default)] rounded-[var(--radius-lg)] hover:border-[var(--accent)] hover:bg-[var(--accent-muted)]/5 transition group"
        >
          <div className="flex flex-col items-center gap-2">
            <Icon path={ICON_PATHS.cloudUpload} className="w-8 h-8 text-[var(--text-faint)] group-hover:text-[var(--accent)] transition-colors" strokeWidth={1.5} />
            <span className="text-sm text-[var(--text-muted)] group-hover:text-[var(--text-secondary)] font-mono">
              {file ? file.name : t("vuln.selectFile")}
            </span>
          </div>
        </button>

        <Checkbox label={t("vuln.importComplete")} checked={complete} onChange={toggleComplete} />
        <p className="-mt-2 text-xs text-[var(--text-muted)]">{t("vuln.importCompleteHint")}</p>

        {dryRun.isPending && <p className="text-sm text-[var(--text-muted)]">{t("vuln.checking")}</p>}
        {error && <FormError message={error} />}
        {preview && (
          <div className="space-y-2">
            <p className="text-sm font-medium text-[var(--text-primary)]">{t("vuln.previewTitle")}</p>
            <dl className="grid grid-cols-3 gap-2">
              {COUNTS.map((k) => (
                <div key={k} className="rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-elevated)] p-2">
                  <dt className="text-xs text-[var(--text-muted)]">{t(`vuln.count.${k}`)}</dt>
                  <dd className={`text-lg font-semibold ${k === "resolved" && preview.resolved > 0 ? "text-[var(--warning)]" : "text-[var(--text-primary)]"}`}>{preview[k]}</dd>
                </div>
              ))}
            </dl>
            {preview.unmatched > 0 && <p className="text-xs text-[var(--text-muted)]">{t("vuln.unmatchedPreviewHint")}</p>}
            <Button size="sm" variant="ghost" onClick={reset}>{t("vuln.pickAnother")}</Button>
          </div>
        )}
      </div>
    </Modal>
  );
}
