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
  const { locale, t } = useLocale();
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

  const title = t("project.ai.title");
  const subtitle = t("project.ai.subtitle");
  const generateLabel = cached?.content ? t("project.ai.regenerate") : t("project.ai.generate");
  const hint = t("project.ai.hint");
  const emptyLabel = t("project.ai.empty");
  const loadingLabel = t("project.ai.loading");

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
    return t("project.ai.generatedAt", { relative, absolute });
  }, [cached, locale, t]);

  const metaLine = useMemo(() => {
    if (!cached) return null;
    const parts: string[] = [];
    if (timestampDisplay) parts.push(timestampDisplay);
    parts.push(t("project.ai.commitsAcrossRepos", { commits: String(cached.commits_used), repos: String(cached.repos_used) }));
    return parts.join(" · ");
  }, [cached, timestampDisplay, t]);

  return (
    <Card accent="accent" hover={false} className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] font-display">
            {title}
          </h2>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">{subtitle}</p>
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
        <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-xs px-3 py-2 mb-3">
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
            <p className="mt-3 pt-3 border-t border-[var(--border-subtle)] text-2xs text-[var(--text-faint)]">
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

