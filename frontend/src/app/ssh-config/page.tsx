"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import CopyButton from "@/components/ui/CopyButton";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import DropdownMenu, { DropdownMenuItem } from "@/components/ui/DropdownMenu";
import Badge from "@/components/ui/Badge";
import { Skeleton } from "@/components/ui/Skeleton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import StatusDot from "@/components/ui/StatusDot";

function highlightSSHConfig(content: string): string {
  return content
    .split("\n")
    .map((line) => {
      if (line.startsWith("#")) {
        return `<span class="text-[var(--text-faint)]">${line}</span>`;
      }
      if (line.startsWith("Host ")) {
        const parts = line.split(" ");
        return `<span class="text-[var(--cyan)] font-semibold">${parts[0]}</span> <span class="text-[var(--warning)]">${parts.slice(1).join(" ")}</span>`;
      }
      const match = line.match(/^(\s+)(\S+)\s+(.*)/);
      if (match) {
        return `${match[1]}<span class="text-[var(--accent)]">${match[2]}</span> <span class="text-[var(--text-primary)]">${match[3]}</span>`;
      }
      return `<span class="text-[var(--text-secondary)]">${line}</span>`;
    })
    .join("\n");
}

export default function SSHConfigPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const [genResult, setGenResult] = useState<{ status: string; host_count: number; path: string } | null>(null);


  const { data: preview, isLoading } = useQuery({
    queryKey: ["ssh-config-preview"],
    queryFn: sshAPI.previewConfig,
  });

  const { data: serverInfo } = useQuery({
    queryKey: ["ssh-server-info"],
    queryFn: sshAPI.serverInfo,
  });

  const generateMutation = useMutation({
    mutationFn: sshAPI.generateConfig,
    onSuccess: (data) => {
      setGenResult(data);
    },
  });

  const canEdit = user?.role === "admin" || user?.role === "editor";


  const handleDownload = () => {
    if (preview?.content) {
      const blob = new Blob([preview.content], { type: "text/plain" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "config";
      a.click();
      URL.revokeObjectURL(url);
    }
  };

  return (
    <PageShell>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold font-display">{t("sshConfig.title")}</h1>
          <p className="text-sm text-[var(--text-muted)] mt-1">{t("sshConfig.generateDescription")}</p>
        </div>
        <div className="flex gap-2">
          <Button onClick={handleDownload} disabled={!preview?.content}>
            <Icon path={ICON_PATHS.download} />
            {t("sshConfig.download")}
          </Button>
          <CopyButton value={preview?.content ?? ""} icon label={t("sshConfig.copy")} copiedLabel={t("sshConfig.copied")} disabled={!preview?.content} />
          {canEdit && (
            <DropdownMenu
              trigger={
                <Button variant="secondary">
                  <Icon path={ICON_PATHS.moreVertical} />
                </Button>
              }
              className="w-72"
            >
              <div className="p-3 border-b border-[var(--border-subtle)]">
                    <p className="text-xs text-[var(--warning)] flex items-center gap-1.5">
                      <Icon path={ICON_PATHS.alertOutline} className="w-3.5 h-3.5 shrink-0" />
                      {t("sshConfig.writeWarning")}
                    </p>
                    {serverInfo && (
                      <code className="block mt-1.5 text-2xs text-[var(--text-muted)] font-mono">
                        {serverInfo.hostname}:{serverInfo.config_path}
                      </code>
                    )}
                  </div>
              <div className="p-1.5">
                <DropdownMenuItem
                  className="rounded-[var(--radius-sm)] py-2 text-[var(--text-primary)]"
                  onClick={() => generateMutation.mutate()}
                  disabled={generateMutation.isPending}
                >
                  {generateMutation.isPending ? t("sshConfig.writing") : t("sshConfig.writeToServer")}
                </DropdownMenuItem>
              </div>
            </DropdownMenu>
          )}
        </div>
      </div>

      {/* Server context banner */}
      {serverInfo && (
        <div className={`mb-4 rounded-[var(--radius-md)] border p-3 text-sm flex items-center justify-between animate-fade-in ${
          serverInfo.is_local
            ? "bg-[var(--cyan)]/8 border-[var(--cyan)]/20 text-[var(--cyan)]"
            : "bg-[var(--warning)]/8 border-[var(--warning)]/20 text-[var(--warning)]"
        }`}>
          <div className="flex items-center gap-2">
            <Icon path={ICON_PATHS.rack} className="w-4 h-4 shrink-0" strokeWidth={1.5} />
            <span>{serverInfo.message}</span>
          </div>
          <div className="flex items-center gap-2">
            <Badge className={serverInfo.is_local
              ? "bg-[var(--cyan)]/15 text-[var(--cyan)] border-[var(--cyan)]/30"
              : "bg-[var(--warning)]/15 text-[var(--warning)] border-[var(--warning)]/30"
            }>
              {serverInfo.is_local ? "localhost" : t("sshConfig.remoteBadge")}
            </Badge>
            <span className="text-xs opacity-70 font-mono">{serverInfo.config_path}</span>
          </div>
        </div>
      )}

      {!serverInfo?.is_local && serverInfo && (
        <div className="mb-4 rounded-[var(--radius-md)] border border-[var(--warning)]/20 bg-[var(--warning)]/5 p-3 text-xs text-[var(--warning)]/80 animate-fade-in">
          {t("sshConfig.remoteDetected", { button: t("sshConfig.generate"), path: serverInfo.config_path, hostname: serverInfo.hostname })}{" "}
          {t("sshConfig.remoteDetectedHint", { download: t("sshConfig.download"), copy: t("sshConfig.copy") })}
        </div>
      )}

      {genResult && (
        <div className="mb-4 bg-[var(--success)]/10 border border-[var(--success)]/25 text-[var(--success)] rounded-[var(--radius-md)] p-3 text-sm animate-slide-down flex items-center gap-2">
          <Icon path={ICON_PATHS.checkCircle} className="w-4 h-4 shrink-0" />
          {t("sshConfig.configGenerated", { count: String(genResult.host_count), path: genResult.path })}
        </div>
      )}

      {generateMutation.isError && (
        <div className="mb-4 bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)] rounded-[var(--radius-md)] p-3 text-sm animate-slide-down">
          {generateMutation.error instanceof Error ? generateMutation.error.message : t("sshConfig.generationFailed")}
        </div>
      )}

      <Card hover={false}>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] font-display">
            {t("sshConfig.preview")}
          </h2>
          {preview?.content && (
            <span className="text-xs text-[var(--text-muted)] font-mono">
              {t("sshConfig.hostsCount", { count: String(preview.content.split("\n").filter((l) => l.startsWith("Host ")).length) })}
            </span>
          )}
        </div>
        {isLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-4 w-full" />)}
          </div>
        ) : (
          <div className="relative">
            <div className="bg-[var(--bg-base)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] overflow-hidden">
              {/* Terminal header */}
              <div className="flex items-center gap-1.5 px-4 py-2 border-b border-[var(--border-subtle)] bg-[var(--bg-elevated)]">
                <StatusDot size="md" className="bg-[var(--danger)]/60" />
                <StatusDot size="md" className="bg-[var(--warning)]/60" />
                <StatusDot size="md" className="bg-[var(--success)]/60" />
                <span className="ml-2 text-2xs text-[var(--text-faint)] font-mono">~/.ssh/config</span>
              </div>
              <div className="flex max-h-[70vh] overflow-auto">
                {/* Line numbers */}
                {preview?.content && (
                  <div className="py-4 pl-4 pr-3 text-right select-none border-r border-[var(--border-subtle)] bg-[var(--bg-base)]">
                    {preview.content.split("\n").map((_, i) => (
                      <div key={i} className="text-xs leading-6 text-[var(--text-faint)] font-mono">
                        {i + 1}
                      </div>
                    ))}
                  </div>
                )}
                <pre
                  className="flex-1 p-4 text-sm leading-6 overflow-x-auto whitespace-pre font-mono"
                  dangerouslySetInnerHTML={{
                    __html: preview?.content
                      ? highlightSSHConfig(preview.content)
                      : `<span class="text-[var(--text-faint)]"># ${t("sshConfig.noActiveHosts")}</span>`,
                  }}
                />
              </div>
            </div>
          </div>
        )}
      </Card>
    </PageShell>
  );
}
