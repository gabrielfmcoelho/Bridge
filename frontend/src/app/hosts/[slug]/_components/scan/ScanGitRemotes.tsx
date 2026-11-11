"use client";

import SectionCard from "@/components/ui/SectionCard";
import { useLocale } from "@/contexts/LocaleContext";
import type { GitRemote, VMInfoType } from "@/lib/api";
import { Detail, Row, Rows, SubBlock, type T } from "./shared";

/**
 * Git checkouts the scan found on the host and in its containers, each remote
 * on a row (credentials are cut on the VM). Remotes on the configured GitLab
 * link this host to their repo (Settings › Repositórios GitLab). A row opens to
 * the checkout's commit, branch and last clone / pull / fetch.
 */
export default function ScanGitRemotes({ info, t }: { info: VMInfoType; t: T }) {
  const { formatDateTime } = useLocale();
  const remotes = info.git_remotes;
  const empty = remotes == null ? t("scan.git.notRun")
    : remotes.length === 0 ? t(info.git_remotes_partial ? "scan.git.partial" : "scan.pane.empty.git")
      : undefined;
  const byWhere = new Map<string, GitRemote[]>();
  for (const r of remotes ?? []) byWhere.set(r.container ?? "", [...(byWhere.get(r.container ?? "") ?? []), r]);

  return (
    <SectionCard as="h3" title={t("scan.pane.group.git")} empty={empty}>
      {!empty && <p className="text-2xs text-[var(--text-muted)] mb-3">{t("scan.git.summary")}</p>}
      {!empty && info.git_remotes_partial && <p className="text-xs text-[var(--warning)] mb-3">{t("scan.git.partial")}</p>}
      {[...byWhere].map(([container, rs]) => (
        <SubBlock key={container} title={container ? t("scan.web.inContainer", { name: container }) : t("scan.web.onHost")} aside={rs[0].container_image}>
          <Rows>
            {rs.map((r, i) => (
              <Row key={i} summary={<>
                <span className="min-w-0 truncate font-mono text-[var(--text-primary)]" title={r.path}>{r.path}</span>
                <span className="shrink-0 text-[var(--text-muted)]">{r.name}</span>
                {r.commit && <span className="shrink-0 font-mono text-[var(--text-muted)]" title={r.commit}>{r.commit.slice(0, 8)}</span>}
                <span className="ml-auto min-w-0 truncate font-mono text-[var(--text-secondary)]" title={r.url}>{r.url}</span>
              </>}>
                {(r.commit || r.pulled_at) ? <>
                  {r.branch && <Detail label={t("scan.git.branch")}>{r.branch}</Detail>}
                  {r.commit && <Detail label={t("scan.git.commit")}>{r.commit}</Detail>}
                  {r.pulled_at && r.pull_kind && (
                    <Detail label={t(`scan.git.kind.${r.pull_kind}`)} mono={false}>{formatDateTime(new Date(r.pulled_at * 1000))}</Detail>
                  )}
                </> : undefined}
              </Row>
            ))}
          </Rows>
        </SubBlock>
      ))}
    </SectionCard>
  );
}
