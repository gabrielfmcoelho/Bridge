"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import { dnsAPI, gitlabReposAPI, hostsAPI, integrationsAPI, projectGitlabAPI, projectsAPI, servicesAPI } from "@/lib/api";
import type { GitLabInventoryRepo, GitLabRepoRef } from "@/lib/types";
import SectionCard from "@/components/ui/SectionCard";
import Input from "@/components/ui/Input";
import PillButton from "@/components/ui/PillButton";
import Button from "@/components/ui/Button";
import Tag from "@/components/ui/Tag";
import Icon from "@/components/ui/Icon";
import Modal from "@/components/ui/Modal";
import AsyncPicker, { type AsyncPickerItem } from "@/components/ui/AsyncPicker";
import StatusAlert from "@/components/ui/StatusAlert";
import EmptyState from "@/components/ui/EmptyState";
import { ICON_PATHS } from "@/lib/icon-paths";
import { buildTree, repoMatches, repoTotal, type GroupNode, type RepoFilter } from "./gitlabRepoTree";

type Kind = "projetos" | "services" | "dns" | "hosts";
const KINDS: Kind[] = ["projetos", "services", "dns", "hosts"];
const FILTERS: RepoFilter[] = ["all", "linked", "unlinked", ...KINDS];

// Search fetchers for the attach picker: the first 20 matches of each list.
const FETCHERS: Record<Kind, (q: string) => Promise<AsyncPickerItem[]>> = {
  projetos: async (q) => (await projectsAPI.listPaginated({ search: q, per_page: "20" })).data.map((p) => ({ id: p.id, label: p.name })),
  services: async (q) => (await servicesAPI.listPaginated({ search: q, per_page: "20" })).data.map((s) => ({ id: s.id, label: s.nickname, secondary: s.repository_url || undefined })),
  dns: async (q) => (await dnsAPI.listPaginated({ search: q, per_page: "20" })).data.map((d) => ({ id: d.id, label: d.domain })),
  hosts: async (q) => (await hostsAPI.listPaginated({ search: q, per_page: "20" })).data.map((h) => ({ id: h.id, label: h.nickname, secondary: h.hostname || undefined })),
};

/** Every group, subgroup and repo the GitLab service token sees, with where
 *  each repo is attached in Bridge. Attach / detach go to each entity's own
 *  home: projeto links, the service's repository_url, DNS/host repo links. */
export default function GitLabReposTab() {
  const { t } = useLocale();
  const confirm = useConfirm();
  const flag = useFlag();
  const qc = useQueryClient();
  const { data: integrations } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const configured = integrations?.gitlab?.gitlab_integration_enabled === "true" && !!integrations?.gitlab?.gitlab_code_service_token;
  const { data: inv, isLoading, error } = useQuery({ queryKey: ["gitlab-repos"], queryFn: gitlabReposAPI.list, enabled: configured, retry: false });

  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<RepoFilter>("all");
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [attachTo, setAttachTo] = useState<GitLabInventoryRepo | null>(null);

  const keep = (r: GitLabInventoryRepo) => repoMatches(r, query, filter);
  const filtering = filter !== "all" || query.trim() !== "";
  const tree = useMemo(
    () => (inv ? buildTree(inv.groups, inv.repos, (r) => repoMatches(r, query, filter), filtering) : { roots: [], loose: [] }),
    [inv, query, filter, filtering],
  );
  const unlisted = (inv?.unlisted ?? []).filter(keep);

  const refresh = () => qc.invalidateQueries({ queryKey: ["gitlab-repos"] });
  const detach = useMutation({
    mutationFn: async ({ kind, ref }: { kind: Kind; ref: GitLabRepoRef }) => {
      if (kind === "projetos") return projectGitlabAPI.deleteLink(ref.id, ref.link_id!);
      if (kind === "services") return servicesAPI.update(ref.id, { repository_url: "" });
      return gitlabReposAPI.unlink(ref.link_id!);
    },
    onSuccess: refresh,
    onError: (e: Error) => flag({ appearance: "error", title: t("settings.gitlabRepos.detachFailed"), description: e.message }),
  });
  const askDetach = async (kind: Kind, ref: GitLabRepoRef, repo: string) => {
    if (await confirm({ title: t("settings.gitlabRepos.detachConfirm", { name: ref.name, repo }), danger: true, confirmLabel: t("settings.gitlabRepos.detach") })) {
      detach.mutate({ kind, ref });
    }
  };

  if (!configured) {
    return (
      <SectionCard as="h3" title={t("settings.gitlabRepos.title")} description={t("settings.gitlabRepos.intro")}>
        <StatusAlert variant="info">{t("settings.gitlabRepos.notConfigured")}</StatusAlert>
      </SectionCard>
    );
  }

  const chips = (r: GitLabInventoryRepo) => (
    <div className="flex flex-wrap gap-1.5">
      {KINDS.flatMap((kind) => r[kind].map((ref) => {
        // A service repo written by the Coolify sync (or the legacy field) is
        // not editable here.
        const removable = kind !== "services" || ref.field === "repository_url";
        const label = `${t(`settings.gitlabRepos.kind.${kind}`)}: ${ref.name}${ref.field === "git_repository" ? " (Coolify)" : ""}`;
        return (
          <Tag key={`${kind}-${ref.link_id ?? ref.id}`} onRemove={removable ? () => askDetach(kind, ref, r.path_with_namespace) : undefined}
            removeLabel={t("settings.gitlabRepos.detach")}>{label}</Tag>
        );
      }))}
    </div>
  );

  const repoRow = (r: GitLabInventoryRepo, depth: number) => (
    <li key={r.id || r.path_with_namespace} className="flex flex-col gap-1.5 py-2 sm:flex-row sm:items-start sm:gap-3" style={{ paddingLeft: `${depth * 1.25}rem` }}>
      <div className="flex min-w-0 items-center gap-2 sm:w-64 sm:shrink-0">
        <Icon path={ICON_PATHS.code} className="w-4 h-4 shrink-0 text-[var(--text-muted)]" />
        <a href={r.web_url} target="_blank" rel="noopener noreferrer" className="truncate text-sm text-[var(--accent)] hover:underline" title={r.path_with_namespace}>{r.name}</a>
      </div>
      <div className="min-w-0 flex-1">
        {r.projetos.length + r.services.length + r.dns.length + r.hosts.length > 0
          ? chips(r)
          : <span className="text-xs text-[var(--text-muted)]">{t("settings.gitlabRepos.noLinks")}</span>}
      </div>
      {r.id !== 0 && (
        <Button size="sm" variant="secondary" onClick={() => setAttachTo(r)}>{t("settings.gitlabRepos.attach")}</Button>
      )}
    </li>
  );

  const groupNode = (n: GroupNode, depth: number): React.ReactNode => {
    const open = filtering || !collapsed.has(n.group.id);
    const toggle = () => setCollapsed((s) => {
      const next = new Set(s);
      if (next.has(n.group.id)) next.delete(n.group.id); else next.add(n.group.id);
      return next;
    });
    return (
      <li key={`g${n.group.id}`}>
        <div className="flex flex-wrap items-center gap-2 py-1.5" style={{ paddingLeft: `${depth * 1.25}rem` }}>
          <button type="button" onClick={toggle} aria-expanded={open} className="inline-flex items-center gap-1.5 text-sm font-medium text-[var(--text-primary)]">
            <Icon path={open ? ICON_PATHS.chevronDown : ICON_PATHS.chevronRight} className="w-3.5 h-3.5 text-[var(--text-muted)]" />
            <Icon path={ICON_PATHS.folder} className="w-4 h-4 text-[var(--text-muted)]" />
            {n.group.name}
          </button>
          <span className="text-xs text-[var(--text-muted)]">{t("settings.gitlabRepos.repoCount", { n: String(repoTotal(n)) })}</span>
          {n.group.projetos.map((p) => (
            <Tag key={p.link_id} onRemove={() => askDetach("projetos", p, n.group.full_path)} removeLabel={t("settings.gitlabRepos.detach")}>
              {`${t("settings.gitlabRepos.kind.projetos")}: ${p.name}`}
            </Tag>
          ))}
        </div>
        {open && (
          <ul>
            {n.children.map((c) => groupNode(c, depth + 1))}
            {n.repos.map((r) => repoRow(r, depth + 1))}
          </ul>
        )}
      </li>
    );
  };

  const empty = tree.roots.length === 0 && tree.loose.length === 0;
  return (
    <div className="space-y-6">
      <SectionCard as="h3" title={t("settings.gitlabRepos.title")} description={t("settings.gitlabRepos.intro")}>
        <div className="space-y-3">
          <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("settings.gitlabRepos.search")} aria-label={t("settings.gitlabRepos.search")} />
          <div className="flex flex-wrap gap-1.5">
            {FILTERS.map((f) => (
              <PillButton key={f} active={filter === f} onClick={() => setFilter(f)}>{t(`settings.gitlabRepos.filter.${f}`)}</PillButton>
            ))}
          </div>
          {error && <StatusAlert variant="error">{(error as Error).message}</StatusAlert>}
          {inv?.truncated && <StatusAlert variant="warning">{t("settings.gitlabRepos.truncated")}</StatusAlert>}
          {isLoading ? (
            <div className="animate-pulse space-y-2">
              {[...Array(4)].map((_, i) => <div key={i} className="h-8 rounded-[var(--radius-md)] bg-[var(--bg-elevated)]" />)}
            </div>
          ) : inv && empty ? (
            <EmptyState compact icon="search" title={filtering ? t("settings.gitlabRepos.emptyFiltered") : t("settings.gitlabRepos.empty")} />
          ) : (
            <ul className="divide-y divide-[var(--border-subtle)]">
              {tree.roots.map((n) => groupNode(n, 0))}
              {tree.loose.map((r) => repoRow(r, 0))}
            </ul>
          )}
        </div>
      </SectionCard>

      {unlisted.length > 0 && (
        <SectionCard as="h3" title={t("settings.gitlabRepos.unlistedTitle")} description={t("settings.gitlabRepos.unlistedHint")}>
          <ul className="divide-y divide-[var(--border-subtle)]">
            {unlisted.map((r) => (
              <li key={r.path_with_namespace} className="flex flex-col gap-1.5 py-2 sm:flex-row sm:items-start sm:gap-3">
                <span className="font-mono text-xs text-[var(--text-secondary)] sm:w-64 sm:shrink-0 break-all">{r.path_with_namespace}</span>
                <div className="min-w-0 flex-1">{chips(r)}</div>
              </li>
            ))}
          </ul>
        </SectionCard>
      )}

      {attachTo && <AttachModal repo={attachTo} onClose={() => setAttachTo(null)} onDone={() => { setAttachTo(null); refresh(); }} />}
    </div>
  );
}

function AttachModal({ repo, onClose, onDone }: { repo: GitLabInventoryRepo; onClose: () => void; onDone: () => void }) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const flag = useFlag();
  const [kind, setKind] = useState<Kind>("dns");
  const [picked, setPicked] = useState<AsyncPickerItem | null>(null);

  const attach = useMutation({
    mutationFn: async () => {
      const id = picked!.id;
      if (kind === "projetos") return projectGitlabAPI.addLink(id, { kind: "project", path: repo.path_with_namespace });
      if (kind === "services") {
        // A service holds one repo: replacing it is a choice, not a side effect.
        const current = picked!.secondary;
        if (current && current !== repo.web_url &&
          !(await confirm({ title: t("settings.gitlabRepos.replaceConfirm", { name: picked!.label, current }), confirmLabel: t("settings.gitlabRepos.replace") }))) {
          return null;
        }
        return servicesAPI.update(id, { repository_url: repo.web_url });
      }
      return gitlabReposAPI.link({ entity_type: kind === "dns" ? "dns" : "host", entity_id: id, path: repo.path_with_namespace });
    },
    onSuccess: (res) => {
      if (res === null) return;
      flag({ appearance: "success", title: t("settings.gitlabRepos.attached", { name: picked!.label, repo: repo.path_with_namespace }) });
      onDone();
    },
    onError: (e: Error) => flag({ appearance: "error", title: t("settings.gitlabRepos.attachFailed"), description: e.message }),
  });

  return (
    <Modal open onClose={onClose} title={t("settings.gitlabRepos.attachTitle", { repo: repo.path_with_namespace })}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={() => attach.mutate()} disabled={!picked} loading={attach.isPending}>{t("settings.gitlabRepos.attach")}</Button>
        </div>
      }>
      <div className="space-y-4">
        <div className="flex flex-wrap gap-1.5">
          {KINDS.map((k) => (
            <PillButton key={k} active={kind === k} onClick={() => { setKind(k); setPicked(null); }}>{t(`settings.gitlabRepos.kind.${k}`)}</PillButton>
          ))}
        </div>
        <AsyncPicker key={kind} label={t(`settings.gitlabRepos.kind.${kind}`)} fetcher={FETCHERS[kind]}
          value={picked?.id ?? null} selectedLabel={picked?.label} onChange={setPicked} placeholder={t("settings.gitlabRepos.pick")} />
      </div>
    </Modal>
  );
}
