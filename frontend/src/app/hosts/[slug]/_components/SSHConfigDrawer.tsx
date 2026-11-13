"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import { useCopy } from "@/hooks/useCopy";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import SectionHeading from "@/components/ui/SectionHeading";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import type { Host } from "@/lib/types";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function SSHConfigDrawer({ open, onClose, slug, host }: {
  open: boolean;
  onClose: () => void;
  slug: string;
  host: Host;
}) {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const [includeKey, setIncludeKey] = useState(!!host.has_key);
  const { copied, copy } = useCopy();

  const { data } = useQuery({
    queryKey: ["ssh-config", slug, includeKey],
    queryFn: () => sshAPI.hostConfig(slug, includeKey),
    enabled: open,
  });

  const config = data?.config || "";
  const pin = data?.host_key;

  const reset = useMutation({
    mutationFn: () => sshAPI.resetHostKey(slug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ssh-config", slug] });
      queryClient.invalidateQueries({ queryKey: ["ssh-host-keys-shared"] });
      flag({ appearance: "success", title: t("host.hostKey.resetDone") });
    },
    onError: (err) => flag({ appearance: "error", title: t("host.hostKey.resetFailed"), description: (err as Error).message }),
  });
  const handleReset = async () => {
    if (await confirm({ title: t("host.hostKey.resetTitle"), message: t("host.hostKey.resetMessage"), danger: true, confirmLabel: t("host.hostKey.reset") })) reset.mutate();
  };

  const handleCopy = () => { if (config) copy(config); };

  return (
    <Drawer open={open} onClose={onClose} title={t("host.sshConfig")}>
      <div className="space-y-4">
        {/* Key toggle */}
        {host.has_key && (
          <div className="flex gap-1 p-1 rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-subtle)]">
            <button
              type="button"
              onClick={() => setIncludeKey(true)}
              className={`flex-1 px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] transition duration-150 ${
                includeKey
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              {t("host.sshConfigWithKey")}
            </button>
            <button
              type="button"
              onClick={() => setIncludeKey(false)}
              className={`flex-1 px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] transition duration-150 ${
                !includeKey
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              {t("host.sshConfigWithoutKey")}
            </button>
          </div>
        )}

        {/* Config block */}
        <div className="relative group">
          <pre
            className="text-xs text-[var(--text-primary)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-4 overflow-x-auto whitespace-pre font-mono"
          >
            {config || "..."}
          </pre>
          <button
            type="button"
            onClick={handleCopy}
            className="absolute top-2 right-2 p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:border-[var(--border-default)] transition"
            title={t("common.copy")}
          >
            {copied ? (
              <Icon path={ICON_PATHS.check} className="w-4 h-4 text-[var(--success)]" />
            ) : (
              <Icon path={ICON_PATHS.copy} />
            )}
          </button>
        </div>

        {/* Full-width copy button */}
        <Button size="sm" variant="secondary" className="w-full" onClick={handleCopy}>
          {copied ? t("common.copied") : t("common.copy")}
        </Button>

        {/* Hint */}
        <p className="text-2xs text-[var(--text-faint)] leading-relaxed">
          {t("host.sshConfigHint")}
        </p>

        {/* Pinned host key: Bridge refuses the host when it shows another one. */}
        <div className="space-y-2 pt-2 border-t border-[var(--border-subtle)]">
          <SectionHeading
            icon={ICON_PATHS.lock}
            actions={pin && user?.role === "admin" ? (
              <Button size="sm" variant="secondary" onClick={handleReset} loading={reset.isPending}>{t("host.hostKey.reset")}</Button>
            ) : undefined}
          >
            {t("host.hostKey.title")}
          </SectionHeading>
          {pin ? (
            <>
              <p className="font-mono text-xs text-[var(--text-primary)] break-all">{pin.key_type} {pin.fingerprint}</p>
              {pin.pinned_at && (
                <p className="text-2xs text-[var(--text-faint)]">{t("host.hostKey.pinnedAt", { date: new Date(pin.pinned_at).toLocaleString(locale) })}</p>
              )}
            </>
          ) : (
            <p className="text-2xs text-[var(--text-faint)] leading-relaxed">{t("host.hostKey.unpinned")}</p>
          )}
        </div>
      </div>
    </Drawer>
  );
}
