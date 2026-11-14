"use client";

import SectionCard from "@/components/ui/SectionCard";
import { useLocale } from "@/contexts/LocaleContext";
import type { PanelDomain, VMInfoType } from "@/lib/api";
import { Detail, Row, Rows, SubBlock, type T } from "./shared";

/**
 * A6 control panels: Virtualmin / Webmin — hosted domains (with the disable
 * event), panel users and failed-login counts. The "who disabled host 35's
 * SEADPREV domains and when" signal, from the panel's own records. Counts and
 * names only; no password or client IP. Background scan only.
 */
export default function ScanPanels({ info, t }: { info: VMInfoType; t: T }) {
  const p = info.deep_facts?.panels;
  const empty = info.deep_facts == null
    ? t("scan.deep.notRun")
    : !p
      ? t("scan.pane.empty.panels")
      : undefined;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.panels")} empty={empty}>
      {p && !empty && (
        <div className="space-y-4">
          <p className="text-2xs text-[var(--text-muted)]">{t("scan.deep.panels.summary")}</p>
          {p.installed && p.installed.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="text-xs text-[var(--text-muted)]">{t("scan.deep.panels.installed")}</span>
              {p.installed.map((n) => (
                <span key={n} className="rounded px-1.5 py-0.5 text-2xs font-mono bg-[var(--bg-elevated)] text-[var(--text-secondary)]">{n}</span>
              ))}
              {p.failed_logins && (p.failed_logins.public || p.failed_logins.private) ? (
                <span className="ml-auto rounded px-2 py-0.5 text-xs bg-[var(--warning)]/10 text-[var(--warning)]" title={t("scan.deep.panels.failed")}>
                  {t("scan.deep.panels.failedPubPriv", {
                    public: (p.failed_logins.public ?? 0).toLocaleString(),
                    private: (p.failed_logins.private ?? 0).toLocaleString(),
                  })}
                </span>
              ) : null}
            </div>
          )}
          {p.domains && p.domains.length > 0 && (
            <SubBlock title={t("scan.deep.panels.domains")} aside={String(p.domains.length)}>
              <Rows>
                {p.domains.map((d, i) => <DomainRow key={i} d={d} t={t} />)}
              </Rows>
            </SubBlock>
          )}
          {p.users && p.users.length > 0 && (
            <SubBlock title={t("scan.deep.panels.users")}>
              <div className="flex flex-wrap gap-1.5">
                {p.users.map((u) => (
                  <span key={u} className="rounded px-1.5 py-0.5 text-2xs font-mono bg-[var(--bg-elevated)] text-[var(--text-secondary)]">{u}</span>
                ))}
              </div>
            </SubBlock>
          )}
        </div>
      )}
    </SectionCard>
  );
}

function DomainRow({ d, t }: { d: PanelDomain; t: T }) {
  const { formatDate, formatDateTime } = useLocale();
  return (
    <Row
      muted={d.disabled}
      summary={<>
        <span className="min-w-0 truncate font-mono text-[var(--text-primary)]">{d.name}</span>
        {d.letsencrypt && <span className="shrink-0 rounded px-1.5 text-2xs bg-[var(--success)]/10 text-[var(--success)]">{t("scan.deep.panels.letsencrypt")}</span>}
        {d.disabled && (
          <span className="ml-auto shrink-0 rounded px-1.5 text-2xs bg-[var(--danger)]/10 text-[var(--danger)]">
            {d.disabled_time ? t("scan.deep.panels.disabledAt", { date: formatDate(new Date(d.disabled_time * 1000)) }) : t("scan.deep.panels.disabled")}
          </span>
        )}
      </>}
    >
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
        {d.owner && <Detail label={t("scan.deep.panels.owner")}>{d.owner}</Detail>}
        {d.contact && <Detail label={t("scan.deep.panels.contact")}>{d.contact}</Detail>}
        {d.created ? <Detail label={t("scan.deep.panels.created")} mono={false}>{formatDate(new Date(d.created * 1000))}</Detail> : null}
        {d.disabled && d.disabled_time ? (
          <Detail label={t("scan.deep.panels.disabled")} mono={false}>{formatDateTime(new Date(d.disabled_time * 1000))}</Detail>
        ) : null}
        {d.disabled_reason && <Detail label={t("scan.deep.panels.reason")} mono={false}>{d.disabled_reason}</Detail>}
      </dl>
    </Row>
  );
}
