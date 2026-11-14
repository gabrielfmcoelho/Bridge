"use client";

import SectionCard from "@/components/ui/SectionCard";
import type { VMInfoType } from "@/lib/api";
import { Row, Rows, SubBlock, type T } from "./shared";

/**
 * A3 outbound flows: where this VM sends data. Transfer destinations parsed
 * from the scripts cron runs (credentials stripped on the host) and the
 * host's established outbound TCP connections. A public peer is flagged — the
 * signal behind a VM quietly pushing data off-site. Background scan only.
 */
export default function ScanOutbound({ info, t }: { info: VMInfoType; t: T }) {
  const ob = info.deep_facts?.outbound;
  const scripts = ob?.scripts ?? [];
  const conns = ob?.connections ?? [];
  const empty = info.deep_facts == null
    ? t("scan.deep.notRun")
    : scripts.length === 0 && conns.length === 0
      ? t("scan.pane.empty.outbound")
      : undefined;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.outbound")} empty={empty}>
      {!empty && <p className="text-2xs text-[var(--text-muted)] mb-3">{t("scan.deep.out.summary")}</p>}
      {scripts.length > 0 && (
        <SubBlock title={t("scan.deep.out.scripts")}>
          <Rows>
            {scripts.map((f, i) => (
              <Row key={i} summary={<>
                <span className="min-w-0 truncate font-mono text-[var(--text-primary)]" title={f.target}>{f.target}</span>
                <span className="shrink-0 text-[var(--text-muted)]">{f.program}</span>
                <span className="ml-auto min-w-0 truncate font-mono text-[var(--text-secondary)]" title={f.script}>{f.script}</span>
              </>} />
            ))}
          </Rows>
        </SubBlock>
      )}
      {conns.length > 0 && (
        <SubBlock title={t("scan.deep.out.connections")}>
          <Rows>
            {conns.map((c, i) => (
              <Row key={i} summary={<>
                <span className="min-w-0 truncate font-mono text-[var(--text-primary)]">
                  {c.peer}{c.port ? `:${c.port}` : ""}
                </span>
                {c.public && (
                  <span className="shrink-0 rounded px-1.5 text-2xs bg-[var(--warning)]/10 text-[var(--warning)]">
                    {t("scan.deep.out.external")}
                  </span>
                )}
                {c.process && <span className="ml-auto shrink-0 font-mono text-[var(--text-muted)]">{c.process}</span>}
              </>} />
            ))}
          </Rows>
        </SubBlock>
      )}
    </SectionCard>
  );
}
