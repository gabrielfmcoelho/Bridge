"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projectGitlabAPI, type ProjectGitLabLink } from "@/lib/api";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import StatusDot from "@/components/ui/StatusDot";
import SectionCard from "@/components/ui/SectionCard";
import { useLocale } from "@/contexts/LocaleContext";

interface Props {
  projectId: number;
  canEdit: boolean;
  gitlabBaseURL?: string;
}

// Normalize a user-entered string (full URL or bare path) into a GitLab path.
// Also detects whether the URL was a canonical group URL (/groups/...) so the kind
// selector can auto-switch.
//
// IMPORTANT: host stripping is ONLY performed when the input starts with http(s)://.
// A plain path like "marcos.waquim/gestor" must be left untouched — its first segment
// contains a dot (the user's last name) but is absolutely not a hostname. The parser
// is idempotent: running it twice on its own output always yields the same result.
function parseGitLabInput(raw: string): { path: string; detectedKind: "group" | null } {
  let s = raw.trim();
  const hadProtocol = /^https?:\/\//i.test(s);
  const originalWithProto = s;

  if (hadProtocol) {
    s = s.replace(/^https?:\/\//i, "");
    // After the protocol, the first segment is always the host — strip it.
    const firstSlash = s.indexOf("/");
    if (firstSlash > 0) {
      s = s.slice(firstSlash + 1);
    } else {
      // URL with no path (e.g. "https://gitlab.com") — nothing to link to.
      s = "";
    }
  }

  const wasGroupURL =
    /^groups\//i.test(s) ||
    /^https?:\/\/[^/]+\/groups\//i.test(originalWithProto);
  s = s.replace(/^groups\//i, "");

  // GitLab prefixes sub-pages under /-/ (e.g. /-/merge_requests, /-/pipelines) — strip those.
  const dashIdx = s.indexOf("/-/");
  if (dashIdx >= 0) s = s.slice(0, dashIdx);

  // Strip trailing slash(es) and .git.
  s = s.replace(/\.git$/, "").replace(/\/+$/, "").replace(/^\/+/, "");

  return { path: s, detectedKind: wasGroupURL ? "group" : null };
}

export default function GitLabLinksEditor({ projectId, canEdit, gitlabBaseURL }: Props) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [kind, setKind] = useState<"project" | "group">("project");
  const [path, setPath] = useState("");
  const [refName, setRefName] = useState("");
  const [error, setError] = useState<string | null>(null);

  const { data: response, isLoading } = useQuery({
    queryKey: ["project-gitlab-links", projectId],
    queryFn: () => projectGitlabAPI.listLinks(projectId),
  });
  const links = response?.links ?? [];
  const integrationEnabled = response?.enabled ?? false;
  const integrationConfigured = response?.configured ?? false;
  const integrationActive = integrationEnabled && integrationConfigured;

  const invalidateLinked = () => {
    queryClient.invalidateQueries({ queryKey: ["project-gitlab-links", projectId] });
    queryClient.invalidateQueries({ queryKey: ["project-commits", projectId] });
  };

  const addMutation = useMutation({
    mutationFn: () => {
      const { path: cleanPath } = parseGitLabInput(path);
      // Dev aid: show exactly what the form submits so mis-parses are obvious in devtools.
      console.debug("[gitlab-links] addLink submit", { kind, rawInput: path, cleanPath, refName: refName.trim() || null });
      return projectGitlabAPI.addLink(projectId, { kind, path: cleanPath, ref_name: refName.trim() || undefined });
    },
    onSuccess: () => {
      setPath("");
      setRefName("");
      setAdding(false);
      setError(null);
      invalidateLinked();
    },
    onError: (err: Error) => setError(err.message || t("project.addLinkFailed")),
  });

  // When the user pastes or types a URL, strip it down to the path and
  // auto-switch kind if the URL points at a canonical group page.
  const handlePathChange = (raw: string) => {
    const { path: cleaned, detectedKind } = parseGitLabInput(raw);
    // Only rewrite the visible input when the raw input clearly looked like a URL —
    // otherwise let the user keep editing a plain path without our interference.
    if (/^https?:\/\//i.test(raw) || raw.includes("gitlab")) {
      setPath(cleaned);
    } else {
      setPath(raw);
    }
    if (detectedKind) setKind(detectedKind);
  };

  const deleteMutation = useMutation({
    mutationFn: (linkId: number) => projectGitlabAPI.deleteLink(projectId, linkId),
    onSuccess: invalidateLinked,
  });

  const baseHost = (() => {
    try {
      return gitlabBaseURL ? new URL(gitlabBaseURL).host : "gitlab.com";
    } catch {
      return "gitlab.com";
    }
  })();

  return (
    <SectionCard
      as="h3"
      title={t("project.linkedGitlabSourcesTitle")}
      description={t("project.linkedGitlabSourcesHint")}
      controls={
        <>
          <IntegrationStatusBadge enabled={integrationEnabled} configured={integrationConfigured} />
          {canEdit && !adding && (
            <Button type="button" size="sm" variant="secondary" onClick={() => { setAdding(true); setError(null); }}>
              + {t("project.addLink")}
            </Button>
          )}
        </>
      }
    >
      {!integrationActive && (
        <p className="mb-3 text-xs text-[var(--warning)]">
          {!integrationEnabled
            ? t("project.gitlabCodeMgmtDisabledWarn")
            : t("project.gitlabTokenNotSetWarn")}
        </p>
      )}

      {adding && (
        <div className="mb-3 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] p-3 space-y-3">
          <div className="flex gap-3">
            <label className="inline-flex items-center gap-2 text-xs cursor-pointer">
              <input type="radio" name="gl-link-kind" checked={kind === "project"} onChange={() => setKind("project")} />
              {t("project.repository")}
            </label>
            <label className="inline-flex items-center gap-2 text-xs cursor-pointer">
              <input type="radio" name="gl-link-kind" checked={kind === "group"} onChange={() => setKind("group")} />
              {t("project.subgroup")}
            </label>
          </div>
          <Input
            label={kind === "group" ? t("project.groupUrlOrPathLabel") : t("project.projectUrlOrPathLabel")}
            value={path}
            onChange={(e) => handlePathChange(e.target.value)}
            onKeyDown={(e) => {
              // Prevent the outer ProjectForm from submitting on Enter inside this
              // nested "add-link" panel. Enter submits this inline add instead.
              if (e.key === "Enter") {
                e.preventDefault();
                if (path.trim() && !addMutation.isPending) addMutation.mutate();
              }
            }}
            placeholder={kind === "group" ? t("project.groupUrlPlaceholder") : t("project.projectUrlPlaceholder")}
          />
          <Input
            label={t("project.branchOptionalLabel")}
            value={refName}
            onChange={(e) => setRefName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                if (path.trim() && !addMutation.isPending) addMutation.mutate();
              }
            }}
            placeholder={t("project.allBranchesPlaceholder")}
          />
          {error && <p className="text-xs text-[var(--danger)]">{error}</p>}
          <div className="flex gap-2">
            <Button type="button" size="sm" onClick={() => addMutation.mutate()} loading={addMutation.isPending} disabled={!path.trim()}>
              {t("common.add")}
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => { setAdding(false); setError(null); }}>
              {t("common.cancel")}
            </Button>
          </div>
        </div>
      )}

      {isLoading ? (
        <p className="text-xs text-[var(--text-muted)]">{t("common.loading")}</p>
      ) : links.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)]">
          {t("project.noGitlabLinksTitle")}{canEdit ? " " + t("project.noGitlabLinksClickHint") : ""}
        </p>
      ) : (
        <ul className="space-y-1.5">
          {links.map((link: ProjectGitLabLink) => (
            <li
              key={link.id}
              className="flex items-center justify-between gap-2 rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] px-3 py-2"
            >
              <div className="flex items-center gap-2 min-w-0 flex-1">
                <LinkHealthIcon link={link} integrationActive={integrationActive} />
                <span className={`text-2xs px-1.5 py-0.5 rounded shrink-0 ${
                  link.kind === "group"
                    ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                    : "bg-[var(--cyan)]/10 text-[var(--cyan)]"
                }`}>
                  {link.kind === "group" ? t("project.linkKindGroup") : t("project.linkKindRepo")}
                </span>
                <span className="text-xs text-[var(--text-secondary)] truncate font-mono">
                  {baseHost}/{link.gitlab_path}
                </span>
                {link.ref_name && (
                  <span className="text-2xs text-[var(--text-muted)] shrink-0">@ {link.ref_name}</span>
                )}
              </div>
              {canEdit && (
                <button
                  type="button"
                  onClick={() => deleteMutation.mutate(link.id)}
                  className="text-[var(--text-muted)] hover:text-[var(--danger)] transition-colors text-sm"
                  aria-label={t("project.removeLinkAria")}
                  disabled={deleteMutation.isPending}
                >
                  &times;
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}

function IntegrationStatusBadge({ enabled, configured }: { enabled: boolean; configured: boolean }) {
  const { t } = useLocale();
  const active = enabled && configured;
  const label = active ? t("project.codeMgmtActiveBadge") : !enabled ? t("project.codeMgmtDisabledBadge") : t("project.noServiceTokenBadge");
  const classes = active
    ? "bg-[var(--success)]/10 text-[var(--success)] border-[var(--success)]/30"
    : "bg-[var(--warning)]/10 text-[var(--warning)] border-[var(--warning)]/30";
  return (
    <span className={`inline-flex items-center gap-1 text-2xs font-medium px-1.5 py-0.5 rounded border ${classes}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${active ? "bg-[var(--success)]" : "bg-[var(--warning)]"}`} />
      {label}
    </span>
  );
}

function LinkHealthIcon({ link, integrationActive }: { link: ProjectGitLabLink; integrationActive: boolean }) {
  const { t } = useLocale();
  // When the integration isn't active we can't verify — show a neutral dot.
  if (!integrationActive || link.reachable === undefined) {
    return (
      <StatusDot color="muted"
        title={t("project.linkNotVerifiedTitle")} />
    );
  }
  if (link.reachable) {
    return (
      <StatusDot className="bg-[var(--success)]"
        title={t("project.linkResolvedTitle")} />
    );
  }
  return (
    <StatusDot className="bg-[var(--danger)]"
      title={link.health_error || t("project.linkNotReachableTitle")} />
  );
}
