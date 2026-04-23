"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projectGitlabAPI, type ProjectGitLabLink } from "@/lib/api";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";

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
    onError: (err: Error) => setError(err.message || "Failed to add link"),
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
    <div className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] p-4">
      <div className="flex items-center justify-between mb-3 gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h4 className="text-xs font-semibold text-[var(--text-primary)]">Linked GitLab sources</h4>
            <IntegrationStatusBadge enabled={integrationEnabled} configured={integrationConfigured} />
          </div>
          <p className="text-[11px] text-[var(--text-muted)] mt-0.5">
            Paste a GitLab URL or path. Individual repos or entire subgroups (subgroups fan out to all their repos).
          </p>
        </div>
        {canEdit && !adding && (
          <Button type="button" size="sm" variant="secondary" onClick={() => { setAdding(true); setError(null); }}>
            + Add link
          </Button>
        )}
      </div>

      {!integrationActive && (
        <p className="mb-3 text-[11px] text-amber-400">
          {!integrationEnabled
            ? "GitLab Code Management is disabled — ask an admin to enable it in Settings → Integrations → GitLab → Code Management."
            : "GitLab service token is not configured — ask an admin to set one in Settings → Integrations → GitLab → Code Management."}
        </p>
      )}

      {adding && (
        <div className="mb-3 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] p-3 space-y-3">
          <div className="flex gap-3">
            <label className="inline-flex items-center gap-2 text-xs cursor-pointer">
              <input type="radio" name="gl-link-kind" checked={kind === "project"} onChange={() => setKind("project")} />
              Repository
            </label>
            <label className="inline-flex items-center gap-2 text-xs cursor-pointer">
              <input type="radio" name="gl-link-kind" checked={kind === "group"} onChange={() => setKind("group")} />
              Subgroup
            </label>
          </div>
          <Input
            label={kind === "group" ? "Group URL or path" : "Project URL or path"}
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
            placeholder={kind === "group" ? "https://gitlab.com/groups/org/subgroup or org/subgroup" : "https://gitlab.com/org/repo or org/repo"}
          />
          <Input
            label="Branch (optional)"
            value={refName}
            onChange={(e) => setRefName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                if (path.trim() && !addMutation.isPending) addMutation.mutate();
              }
            }}
            placeholder="leave blank for all branches"
          />
          {error && <p className="text-xs text-red-400">{error}</p>}
          <div className="flex gap-2">
            <Button type="button" size="sm" onClick={() => addMutation.mutate()} loading={addMutation.isPending} disabled={!path.trim()}>
              Add
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => { setAdding(false); setError(null); }}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      {isLoading ? (
        <p className="text-xs text-[var(--text-muted)]">Loading...</p>
      ) : links.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)]">
          No GitLab sources linked yet.{canEdit ? " Click Add link above to start tracking commits." : ""}
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
                <span className={`text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded shrink-0 ${
                  link.kind === "group"
                    ? "bg-purple-500/10 text-purple-400"
                    : "bg-cyan-500/10 text-cyan-400"
                }`}>
                  {link.kind === "group" ? "group" : "repo"}
                </span>
                <span className="text-xs text-[var(--text-secondary)] truncate font-mono">
                  {baseHost}/{link.gitlab_path}
                </span>
                {link.ref_name && (
                  <span className="text-[10px] text-[var(--text-muted)] shrink-0">@ {link.ref_name}</span>
                )}
              </div>
              {canEdit && (
                <button
                  type="button"
                  onClick={() => deleteMutation.mutate(link.id)}
                  className="text-[var(--text-muted)] hover:text-red-400 transition-colors text-sm"
                  aria-label="Remove link"
                  disabled={deleteMutation.isPending}
                >
                  &times;
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function IntegrationStatusBadge({ enabled, configured }: { enabled: boolean; configured: boolean }) {
  const active = enabled && configured;
  const label = active ? "code mgmt active" : !enabled ? "code mgmt disabled" : "no service token";
  const classes = active
    ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/30"
    : "bg-amber-500/10 text-amber-400 border-amber-500/30";
  return (
    <span className={`inline-flex items-center gap-1 text-[10px] font-medium uppercase tracking-wide px-1.5 py-0.5 rounded border ${classes}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${active ? "bg-emerald-400" : "bg-amber-400"}`} />
      {label}
    </span>
  );
}

function LinkHealthIcon({ link, integrationActive }: { link: ProjectGitLabLink; integrationActive: boolean }) {
  // When the integration isn't active we can't verify — show a neutral dot.
  if (!integrationActive || link.reachable === undefined) {
    return (
      <span
        className="w-2 h-2 rounded-full bg-[var(--text-faint)] shrink-0"
        title="Link not verified (GitLab integration inactive)"
      />
    );
  }
  if (link.reachable) {
    return (
      <span
        className="w-2 h-2 rounded-full bg-emerald-400 shrink-0"
        title="Resolved on GitLab"
      />
    );
  }
  return (
    <span
      className="w-2 h-2 rounded-full bg-red-400 shrink-0"
      title={link.health_error || "Not reachable on GitLab"}
    />
  );
}
