"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import { useCopy } from "@/hooks/useCopy";
import { useLocale } from "@/contexts/LocaleContext";
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
  const { t } = useLocale();
  const [includeKey, setIncludeKey] = useState(!!host.has_key);
  const { copied, copy } = useCopy();

  const { data } = useQuery({
    queryKey: ["ssh-config", slug, includeKey],
    queryFn: () => sshAPI.hostConfig(slug, includeKey),
    enabled: open,
  });

  const config = data?.config || "";

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
            className="text-xs text-[var(--text-primary)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-4 overflow-x-auto whitespace-pre"
            style={{ fontFamily: "var(--font-mono)" }}
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
              <Icon path={ICON_PATHS.check} className="w-4 h-4 text-emerald-400" />
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
        <p className="text-[10px] text-[var(--text-faint)] leading-relaxed">
          {t("host.sshConfigHint")}
        </p>
      </div>
    </Drawer>
  );
}
