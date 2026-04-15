"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import type { Host } from "@/lib/types";

export default function SSHConfigDrawer({ open, onClose, slug, host }: {
  open: boolean;
  onClose: () => void;
  slug: string;
  host: Host;
}) {
  const { t } = useLocale();
  const [includeKey, setIncludeKey] = useState(!!host.has_key);
  const [copied, setCopied] = useState(false);

  const { data } = useQuery({
    queryKey: ["ssh-config", slug, includeKey],
    queryFn: () => sshAPI.hostConfig(slug, includeKey),
    enabled: open,
  });

  const config = data?.config || "";

  const handleCopy = async () => {
    if (!config) return;
    await navigator.clipboard.writeText(config);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Drawer open={open} onClose={onClose} title={t("host.sshConfig")}>
      <div className="space-y-4">
        {/* Key toggle */}
        {host.has_key && (
          <div className="flex gap-1 p-1 rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-subtle)]">
            <button
              type="button"
              onClick={() => setIncludeKey(true)}
              className={`flex-1 px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] transition-all duration-150 ${
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
              className={`flex-1 px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] transition-all duration-150 ${
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
            className="absolute top-2 right-2 p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:border-[var(--border-default)] transition-all opacity-0 group-hover:opacity-100"
            title={t("common.copy")}
          >
            {copied ? (
              <svg className="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
            )}
          </button>
        </div>

        {/* Copy button (always visible on mobile) */}
        <Button size="sm" variant="secondary" className="w-full md:hidden" onClick={handleCopy}>
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
