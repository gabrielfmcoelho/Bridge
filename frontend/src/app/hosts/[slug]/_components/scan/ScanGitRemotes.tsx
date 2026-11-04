"use client";

import SectionCard from "@/components/ui/SectionCard";
import type { GitRemote, VMInfoType } from "@/lib/api";
import { Row, Rows, SubBlock, type T } from "./shared";

/**
 * Git checkouts the scan found on the host and in its containers, each remote
 * on a row (credentials are cut on the VM). Remotes on the configured GitLab
 * link this host to their repo (Settings › Repositórios GitLab).
 */
export default function ScanGitRemotes({ info, t }: { info: VMInfoType; t: T }) {
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
                <span className="ml-auto min-w-0 truncate font-mono text-[var(--text-secondary)]" title={r.url}>{r.url}</span>
              </>} />
            ))}
          </Rows>
        </SubBlock>
      ))}
    </SectionCard>
  );
}
