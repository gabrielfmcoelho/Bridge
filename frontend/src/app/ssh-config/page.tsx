"use client";

import { useState, useRef, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import { Skeleton } from "@/components/ui/Skeleton";

function highlightSSHConfig(content: string): string {
  return content
    .split("\n")
    .map((line) => {
      if (line.startsWith("#")) {
        return `<span class="text-[var(--text-faint)]">${line}</span>`;
      }
      if (line.startsWith("Host ")) {
        const parts = line.split(" ");
        return `<span class="text-cyan-400 font-semibold">${parts[0]}</span> <span class="text-amber-400">${parts.slice(1).join(" ")}</span>`;
      }
      const match = line.match(/^(\s+)(\S+)\s+(.*)/);
      if (match) {
        return `${match[1]}<span class="text-purple-400">${match[2]}</span> <span class="text-[var(--text-primary)]">${match[3]}</span>`;
      }
      return `<span class="text-[var(--text-secondary)]">${line}</span>`;
    })
    .join("\n");
}

export default function SSHConfigPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const [genResult, setGenResult] = useState<{ status: string; host_count: number; path: string } | null>(null);
  const [copied, setCopied] = useState(false);

  const [showWriteMenu, setShowWriteMenu] = useState(false);
  const writeMenuRef = useRef<HTMLDivElement>(null);

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
      setShowWriteMenu(false);
    },
  });

  const canEdit = user?.role === "admin" || user?.role === "editor";

  // Close write menu on outside click
  useEffect(() => {
    if (!showWriteMenu) return;
    const handler = (e: MouseEvent) => {
      if (writeMenuRef.current && !writeMenuRef.current.contains(e.target as Node)) {
        setShowWriteMenu(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [showWriteMenu]);

  const handleCopy = async () => {
    if (preview?.content) {
      await navigator.clipboard.writeText(preview.content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

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
          <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("sshConfig.title")}</h1>
          <p className="text-sm text-[var(--text-muted)] mt-1">{t("sshConfig.generateDescription")}</p>
        </div>
        <div className="flex gap-2">
          <Button onClick={handleDownload} disabled={!preview?.content}>
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            {t("sshConfig.download")}
          </Button>
          <Button variant="secondary" onClick={handleCopy} disabled={!preview?.content}>
            {copied ? (
              <>
                <svg className="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
                {t("sshConfig.copied")}
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
                {t("sshConfig.copy")}
              </>
            )}
          </Button>
          {canEdit && (
            <div className="relative" ref={writeMenuRef}>
              <Button
                variant="secondary"
                onClick={() => setShowWriteMenu((v) => !v)}
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 12.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 18.75a.75.75 0 110-1.5.75.75 0 010 1.5z" />
                </svg>
              </Button>
              {showWriteMenu && (
                <div className="absolute right-0 top-full mt-1 z-50 w-72 rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] shadow-xl animate-fade-in">
                  <div className="p-3 border-b border-[var(--border-subtle)]">
                    <p className="text-xs text-amber-400 flex items-center gap-1.5">
                      <svg className="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z" />
                      </svg>
                      {t("sshConfig.writeWarning")}
                    </p>
                    {serverInfo && (
                      <code className="block mt-1.5 text-[10px] text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>
                        {serverInfo.hostname}:{serverInfo.config_path}
                      </code>
                    )}
                  </div>
                  <div className="p-1.5">
                    <button
                      className="w-full text-left px-3 py-2 text-sm rounded-[var(--radius-sm)] hover:bg-[var(--bg-hover)] text-[var(--text-primary)] transition-colors disabled:opacity-50"
                      onClick={() => generateMutation.mutate()}
                      disabled={generateMutation.isPending}
                    >
                      {generateMutation.isPending ? t("sshConfig.writing") : t("sshConfig.writeToServer")}
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Server context banner */}
      {serverInfo && (
        <div className={`mb-4 rounded-[var(--radius-md)] border p-3 text-sm flex items-center justify-between animate-fade-in ${
          serverInfo.is_local
            ? "bg-cyan-500/8 border-cyan-500/20 text-cyan-400"
            : "bg-amber-500/8 border-amber-500/20 text-amber-400"
        }`}>
          <div className="flex items-center gap-2">
            <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2z" />
            </svg>
            <span>{serverInfo.message}</span>
          </div>
          <div className="flex items-center gap-2">
            <Badge className={serverInfo.is_local
              ? "bg-cyan-500/15 text-cyan-300 border-cyan-500/30"
              : "bg-amber-500/15 text-amber-300 border-amber-500/30"
            }>
              {serverInfo.is_local ? "localhost" : "remote"}
            </Badge>
            <span className="text-xs opacity-70" style={{ fontFamily: "var(--font-mono)" }}>{serverInfo.config_path}</span>
          </div>
        </div>
      )}

      {!serverInfo?.is_local && serverInfo && (
        <div className="mb-4 rounded-[var(--radius-md)] border border-amber-500/20 bg-amber-500/5 p-3 text-xs text-amber-400/80 animate-fade-in">
          <strong>Remote server detected.</strong> The &quot;Generate Config&quot; button writes to <code style={{ fontFamily: "var(--font-mono)" }}>{serverInfo.config_path}</code> on <strong>{serverInfo.hostname}</strong>.
          To use this config on your local machine, use the <strong>Download</strong> or <strong>Copy</strong> buttons instead.
        </div>
      )}

      {genResult && (
        <div className="mb-4 bg-emerald-500/10 border border-emerald-500/25 text-emerald-400 rounded-[var(--radius-md)] p-3 text-sm animate-slide-down flex items-center gap-2">
          <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Config generated with {genResult.host_count} hosts at <code style={{ fontFamily: "var(--font-mono)" }}>{genResult.path}</code>
        </div>
      )}

      {generateMutation.isError && (
        <div className="mb-4 bg-red-500/10 border border-red-500/25 text-red-400 rounded-[var(--radius-md)] p-3 text-sm animate-slide-down">
          {generateMutation.error instanceof Error ? generateMutation.error.message : "Generation failed"}
        </div>
      )}

      <Card hover={false}>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-semibold text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-display)" }}>
            {t("sshConfig.preview")}
          </h2>
          {preview?.content && (
            <span className="text-xs text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>
              {preview.content.split("\n").filter((l) => l.startsWith("Host ")).length} hosts
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
                <span className="w-2.5 h-2.5 rounded-full bg-red-500/60" />
                <span className="w-2.5 h-2.5 rounded-full bg-yellow-500/60" />
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-500/60" />
                <span className="ml-2 text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>~/.ssh/config</span>
              </div>
              <div className="flex max-h-[70vh] overflow-auto">
                {/* Line numbers */}
                {preview?.content && (
                  <div className="py-4 pl-4 pr-3 text-right select-none border-r border-[var(--border-subtle)] bg-[var(--bg-base)]">
                    {preview.content.split("\n").map((_, i) => (
                      <div key={i} className="text-xs leading-6 text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
                        {i + 1}
                      </div>
                    ))}
                  </div>
                )}
                <pre
                  className="flex-1 p-4 text-sm leading-6 overflow-x-auto whitespace-pre"
                  style={{ fontFamily: "var(--font-mono)" }}
                  dangerouslySetInnerHTML={{
                    __html: preview?.content
                      ? highlightSSHConfig(preview.content)
                      : '<span class="text-[var(--text-faint)]"># No active hosts configured</span>',
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
