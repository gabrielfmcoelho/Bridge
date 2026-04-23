"use client";

import { useState, useMemo } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { projectGitlabAPI, integrationsAPI, type ProjectGitLabCommit } from "@/lib/api";
import Card from "@/components/ui/Card";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";

interface Props {
  projectId: number;
}

export default function CommitsTab({ projectId }: Props) {
  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
  });

  const gitlabSettings = integrations?.gitlab;
  const adminConfigured = !!integrations;
  const codeEnabled = gitlabSettings?.gitlab_integration_enabled === "true";

  const { data: response, isLoading } = useQuery({
    queryKey: ["project-commits", projectId],
    queryFn: () => projectGitlabAPI.listCommits(projectId),
  });

  const serverEnabled = response?.enabled ?? codeEnabled;
  const serverConfigured = response?.configured ?? false;
  const allCommits: ProjectGitLabCommit[] = response?.commits ?? [];
  const warnings: string[] = response?.warnings ?? [];
  const authError = response?.error === "auth_failed";

  if (!isLoading && adminConfigured && !codeEnabled) {
    return (
      <EmptyState
        icon="box"
        title="GitLab Code Management is disabled"
        description="Ask an admin to enable it in Settings → Integrations → GitLab."
        compact
      />
    );
  }

  if (!isLoading && !serverEnabled) {
    return (
      <EmptyState
        icon="box"
        title="GitLab Code Management is disabled"
        description="Ask an admin to enable it in Settings → Integrations → GitLab."
        compact
      />
    );
  }

  if (!isLoading && !serverConfigured) {
    return (
      <EmptyState
        icon="key"
        title="Service token not configured"
        description="Ask an admin to add a GitLab service access token in Settings → Integrations → GitLab."
        compact
      />
    );
  }

  return (
    <div className="space-y-5 animate-fade-in">
      {authError && (
        <Card accent="red" hover={false}>
          <p className="text-sm text-red-400">
            GitLab rejected the service access token. Ask an admin to refresh it in Settings.
          </p>
        </Card>
      )}

      {isLoading ? (
        <div className="flex flex-col md:flex-row md:flex-nowrap gap-3 md:-mx-2 md:px-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton
              key={i}
              className="h-80 w-full md:grow md:shrink-0 md:basis-1/2 lg:basis-1/3 rounded-[var(--radius-md)]"
            />
          ))}
        </div>
      ) : allCommits.length === 0 ? (
        <EmptyState
          icon="search"
          title="No commits yet"
          description={
            response && response.commits.length === 0 && !response.warnings?.length
              ? "Linked sources are up to date — or have no commits."
              : "Edit this project to link a GitLab repository or subgroup."
          }
          compact
        />
      ) : (
        // Groups stretch to fill the row; if they don't all fit they become horizontally scrollable.
        // Responsive target per row: mobile=1, md=2, lg=4, xl=5. Cards grow to fill extra space
        // when fewer than target are present, and overflow-scroll kicks in past the target.
        <div className="flex flex-col gap-3 md:flex-row md:flex-nowrap md:overflow-x-auto md:snap-x md:snap-mandatory md:-mx-2 md:px-2 md:pb-2">
          {groupByRepo(allCommits).map((g) => (
            <RepoGroup key={g.projectId} group={g} />
          ))}
        </div>
      )}

      {warnings.length > 0 && (
        <details className="text-xs text-[var(--text-muted)]">
          <summary className="cursor-pointer hover:text-[var(--text-secondary)]">
            {warnings.length} warning{warnings.length === 1 ? "" : "s"}
          </summary>
          <ul className="mt-2 space-y-1 pl-4 list-disc">
            {warnings.map((w, i) => (
              <li key={i} className="font-mono break-all">{w}</li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}

interface RepoCommitGroup {
  projectId: number;
  projectName: string;
  projectPath: string;
  commits: ProjectGitLabCommit[];
  latest: number;
}

function groupByRepo(commits: ProjectGitLabCommit[]): RepoCommitGroup[] {
  const byId = new Map<number, RepoCommitGroup>();
  for (const c of commits) {
    const ts = Date.parse(c.committed_date) || 0;
    const existing = byId.get(c.source_project_id);
    if (existing) {
      existing.commits.push(c);
      if (ts > existing.latest) existing.latest = ts;
    } else {
      byId.set(c.source_project_id, {
        projectId: c.source_project_id,
        projectName: c.source_project_name,
        projectPath: c.source_project_path,
        commits: [c],
        latest: ts,
      });
    }
  }
  return Array.from(byId.values()).sort((a, b) => b.latest - a.latest);
}

function RepoGroup({ group }: { group: RepoCommitGroup }) {
  const [open, setOpen] = useState(true);
  // Prefer the GitLab "Name" nickname; fall back to last path segment if the server
  // couldn't enrich it (older links, deleted repos).
  const repoTitle = useMemo(() => {
    if (group.projectName) return group.projectName;
    const parts = group.projectPath.split("/").filter(Boolean);
    return parts[parts.length - 1] || group.projectPath;
  }, [group.projectName, group.projectPath]);

  return (
    <section className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] overflow-hidden w-full self-start md:grow md:shrink-0 md:basis-1/2 lg:basis-1/3 md:snap-start">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center gap-2 px-3 py-2 hover:bg-[var(--bg-overlay)] transition-colors text-left"
        title={group.projectPath}
      >
        <svg
          className={`w-3.5 h-3.5 text-[var(--text-muted)] shrink-0 transition-transform ${open ? "rotate-90" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
        </svg>
        <svg className="w-3.5 h-3.5 text-[var(--accent)] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
        <div className="min-w-0 flex-1 flex flex-col leading-tight">
          <span className="text-sm font-semibold text-[var(--text-primary)] truncate">{repoTitle}</span>
          {group.projectPath && (
            // direction:rtl + LRM prefix makes the ellipsis appear on the LEFT when the
            // path overflows, so the most specific (rightmost) segment stays visible.
            // The LRM (‎) keeps the path content itself LTR so slashes don't reorder.
            <span
              className="text-[10px] text-[var(--text-faint)] font-mono block overflow-hidden whitespace-nowrap text-left"
              style={{ direction: "rtl", textOverflow: "ellipsis" }}
            >
              {"‎" + group.projectPath}
            </span>
          )}
        </div>
      </button>

      {open && (
        <ul className="space-y-1.5 p-2 pt-0">
          {group.commits.map((c) => (
            <li key={`${c.source_project_id}-${c.id}`}>
              <CommitRow commit={c} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function CommitRow({ commit }: { commit: ProjectGitLabCommit }) {
  const { locale } = useLocale();
  const when = useMemo(() => getTimeAgo(commit.committed_date, locale), [commit.committed_date, locale]);
  const initials = useMemo(() => getInitials(commit.author_name), [commit.author_name]);
  const primaryBranch = commit.branches?.[0];
  const extraBranches = (commit.branches?.length ?? 0) - 1;

  return (
    <Link
      href={commit.web_url}
      target="_blank"
      rel="noopener noreferrer"
      title={commit.id}
    >
      <Card hover clickIndicator="link" className="!p-3 h-[124px] flex flex-col">
        {/* Top row: time + branch on the left, hash anchored top-right */}
        <div className="flex items-start justify-between gap-3">
          <div
            className="flex items-center gap-2 text-[11px] text-[var(--text-faint)] min-w-0 flex-1"
            style={{ fontFamily: "var(--font-mono)" }}
            title={commit.branches?.join(", ")}
          >
            <svg className="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="shrink-0">{when}</span>
            {primaryBranch && (
              <>
                <svg className="w-3 h-3 shrink-0 text-[var(--text-muted)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 3v12m0 0a3 3 0 106 0m-6 0a3 3 0 116 0m6-12a3 3 0 11-6 0 3 3 0 016 0zm0 6a9 9 0 01-9 9" />
                </svg>
                <span className="text-[var(--text-muted)] truncate min-w-0">{primaryBranch}</span>
                {extraBranches > 0 && (
                  <span className="shrink-0 text-[var(--text-faint)]">+{extraBranches}</span>
                )}
              </>
            )}
          </div>
          <code
            className="shrink-0 px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[11px] text-[var(--text-faint)] border border-[var(--border-subtle)]"
            style={{ fontFamily: "var(--font-mono)" }}
            title={commit.id}
          >
            {commit.short_id}
          </code>
        </div>

        {/* Title — reserves height for 2 lines so 1-line titles don't shift layout */}
        <p className="text-sm font-medium text-[var(--text-primary)] leading-snug line-clamp-2 min-h-[2.75em] mt-1.5">
          {commit.title}
        </p>

        {/* Author — anchored to the bottom via mt-auto */}
        <div className="flex items-center gap-2 mt-auto pt-2 text-[11px] text-[var(--text-muted)]">
          <span
            className="w-5 h-5 rounded-full bg-[var(--accent-muted)] text-[var(--accent)] text-[9px] font-bold flex items-center justify-center shrink-0"
            title={commit.author_name}
          >
            {initials}
          </span>
          <span className="truncate text-[var(--text-secondary)]">{commit.author_name}</span>
        </div>
      </Card>
    </Link>
  );
}

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}
