"use client";

import { useState, useRef } from "react";
import { useRouter } from "next/navigation";
import { backupAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useAuth } from "@/contexts/AuthContext";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function BackupTab() {
  const confirm = useConfirm();
  const { t } = useLocale();
  const { logout } = useAuth();
  const router = useRouter();
  const restoreInputRef = useRef<HTMLInputElement>(null);
  const [downloading, setDownloading] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);

  const handleBackup = async () => {
    setDownloading(true);
    setResult(null);
    try {
      await backupAPI.download();
      setResult({ ok: true, message: t("settings.backupSuccess") });
    } catch (err) {
      setResult({ ok: false, message: err instanceof Error ? err.message : "Backup failed" });
    } finally {
      setDownloading(false);
    }
  };

  const handleRestore = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const word = t("settings.restoreConfirmWord");
    if (!(await confirm({ title: t("confirm.restoreTitle"), message: <p className="whitespace-pre-line">{t("settings.restoreConfirm")}</p>, danger: true, requireText: word, confirmLabel: t("trash.restore") }))) {
      if (restoreInputRef.current) restoreInputRef.current.value = "";
      return;
    }
    setRestoring(true);
    setResult(null);
    try {
      const res = await backupAPI.restore(file);
      const parts: string[] = [res.message];
      if (res.row_count != null) {
        parts.push(`${res.row_count} rows restored.`);
      }
      if (res.cross_dialect && res.source_dialect && res.target_dialect) {
        parts.push(`Cross-dialect: ${res.source_dialect} → ${res.target_dialect}.`);
      }
      parts.push(t("settings.restoreSuccessLogout"));
      setResult({ ok: true, message: parts.join(" ") });
      if (restoreInputRef.current) restoreInputRef.current.value = "";
      // The restore wiped the users and sessions tables, so the current
      // session cookie now points to nothing. Give the user a moment to
      // read the success banner, then force a clean re-login.
      setTimeout(async () => {
        try {
          await logout();
        } catch {
          // Server-side session is already gone — ignore and continue.
        }
        router.push("/login");
      }, 2500);
      return;
    } catch (err) {
      setResult({ ok: false, message: err instanceof Error ? err.message : "Restore failed" });
    } finally {
      setRestoring(false);
      if (restoreInputRef.current) restoreInputRef.current.value = "";
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      {/* Backup */}
      <div className="stagger-in" style={{ "--i": 0 } as React.CSSProperties}>
        <SectionCard as="h3" title={t("settings.backupTitle")} icon={ICON_PATHS.exportDoc} description={t("settings.backupDescription")}>
          <Button size="sm" variant="secondary" onClick={handleBackup} loading={downloading}>
            <Icon path={ICON_PATHS.exportDoc} className="w-3.5 h-3.5 mr-1.5" />
            {t("settings.downloadBackup")}
          </Button>
        </SectionCard>
      </div>

      {/* Restore */}
      <div className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
        <SectionCard as="h3" title={t("settings.restoreTitle")} icon={ICON_PATHS.upload} description={t("settings.restoreDescription")}>
          <p className="text-xs text-[var(--danger)]/80 mb-3">
            {t("settings.restoreWarning")}
          </p>
          <input
            ref={restoreInputRef}
            type="file"
            accept=".sshcmbak,.db,.sqlite,.sqlite3,.gz,application/gzip"
            onChange={handleRestore}
            className="hidden"
          />
          <Button size="sm" variant="danger" onClick={() => restoreInputRef.current?.click()} loading={restoring}>
            <Icon path={ICON_PATHS.upload} className="w-3.5 h-3.5 mr-1.5" />
            {t("settings.uploadRestore")}
          </Button>
        </SectionCard>
      </div>

      {/* Result message */}
      {result && (
        <div className={`p-3 rounded-[var(--radius-md)] text-sm animate-slide-up ${
          result.ok
            ? "bg-[var(--success)]/10 border border-[var(--success)]/25 text-[var(--success)]"
            : "bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)]"
        }`}>
          {result.message}
        </div>
      )}
    </div>
  );
}
