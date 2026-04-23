"use client";

import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { aiAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";

interface Props {
  projectId: number;
}

export default function ProjectAiAnalysis({ projectId }: Props) {
  const { locale } = useLocale();
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  // Load the cached analysis on mount — never calls the LLM.
  const { data: cached, isLoading } = useQuery({
    queryKey: ["project-ai-analysis", projectId],
    queryFn: () => aiAPI.getProjectAnalysis(projectId),
    staleTime: 60_000,
  });

  const mutation = useMutation({
    mutationFn: () => aiAPI.analyzeProject(projectId, locale),
    onSuccess: (fresh) => {
      setError(null);
      // Replace the cached query data with the freshly saved record.
      queryClient.setQueryData(["project-ai-analysis", projectId], fresh);
    },
    onError: (err: Error) => setError(err.message),
  });

  const isPt = locale.toLowerCase().startsWith("pt");
  const title = isPt ? "Análise de IA" : "AI Analysis";
  const subtitle = isPt
    ? "Resumo do que está sendo trabalhado com base nos últimos commits."
    : "Summary of current work based on the most recent commits.";
  const generateLabel = cached?.content
    ? (isPt ? "Regenerar" : "Regenerate")
    : (isPt ? "Gerar análise" : "Generate analysis");
  const hint = isPt
    ? "Lê os commits mais recentes dos repositórios vinculados ao projeto e pede ao modelo um resumo curto."
    : "Reads the most recent commits from linked repos and asks the model for a short summary.";
  const emptyLabel = isPt ? "Nenhuma análise gerada ainda." : "No analysis generated yet.";
  const loadingLabel = isPt ? "Analisando commits…" : "Analyzing commits…";

  // generated_at fallback to relative-time + absolute.
  const timestampDisplay = useMemo(() => {
    if (!cached?.generated_at) return null;
    const d = new Date(cached.generated_at);
    if (isNaN(d.getTime())) return null;
    const relative = getTimeAgo(cached.generated_at, locale);
    const absolute = d.toLocaleString(locale === "pt-BR" ? "pt-BR" : undefined, {
      day: "2-digit", month: "2-digit", year: "numeric",
      hour: "2-digit", minute: "2-digit",
    });
    const prefix = isPt ? "Gerado" : "Generated";
    return `${prefix} ${relative} · ${absolute}`;
  }, [cached?.generated_at, locale, isPt]);

  const metaLine = useMemo(() => {
    if (!cached) return null;
    const parts: string[] = [];
    if (timestampDisplay) parts.push(timestampDisplay);
    parts.push(isPt
      ? `${cached.commits_used} commits de ${cached.repos_used} repos`
      : `${cached.commits_used} commits across ${cached.repos_used} repos`);
    return parts.join(" · ");
  }, [cached, timestampDisplay, isPt]);

  return (
    <Card accent="purple" hover={false} className="animate-slide-up stagger-2" style={{ animationFillMode: "both" } as React.CSSProperties}>
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-display)" }}>
            {title}
          </h2>
          <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{subtitle}</p>
        </div>
        <Button
          type="button"
          size="sm"
          variant="secondary"
          onClick={() => mutation.mutate()}
          loading={mutation.isPending}
          disabled={isLoading}
        >
          {generateLabel}
        </Button>
      </div>

      {error && (
        <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-xs px-3 py-2 mb-3">
          {error}
        </div>
      )}

      {mutation.isPending && (
        <p className="text-xs text-[var(--text-muted)] italic animate-pulse mb-3">
          {loadingLabel}
        </p>
      )}

      {isLoading ? (
        <p className="text-xs text-[var(--text-muted)]">…</p>
      ) : cached?.content ? (
        <>
          <div className="text-[var(--text-primary)]">
            <MarkdownContent content={cached.content} />
          </div>
          {metaLine && (
            <p className="mt-3 pt-3 border-t border-[var(--border-subtle)] text-[10px] text-[var(--text-faint)]">
              {metaLine}
            </p>
          )}
        </>
      ) : !mutation.isPending && !error ? (
        <p className="text-xs text-[var(--text-muted)]">
          {emptyLabel} <span className="text-[var(--text-faint)]">{hint}</span>
        </p>
      ) : null}
    </Card>
  );
}

