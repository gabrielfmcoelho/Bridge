"use client";

import { Fragment, useState } from "react";
import Link from "next/link";
import Icon from "@/components/ui/Icon";
import Tag from "@/components/ui/Tag";
import { tableClasses } from "@/components/ui/Table";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import { APP_LINK_TYPES, appDepsHref, linkRows, type AppDepIndex, type AppDepRow, type AppLinkCounts } from "@/lib/appDeps";
import AppDepLinksTable from "./AppDepLinksTable";
import CriticalityBadge from "./CriticalityBadge";

const MAX_TECH = 4;
const LINK_ICONS = { host: ICON_PATHS.server, service: ICON_PATHS.serverStack, dns: ICON_PATHS.globeMeridian, project: ICON_PATHS.folder };
const dash = <span className="text-[var(--text-faint)]">–</span>;

/** One row per application (CI); a row unfolds into the links it declares. */
export default function AppDepsReport({ rows, index, counts }: { rows: AppDepRow[]; index: AppDepIndex; counts: Map<string, AppLinkCounts> }) {
  const { t } = useLocale();
  const [open, setOpen] = useState<Set<string>>(new Set());
  const toggle = (ci: string) =>
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(ci)) next.delete(ci);
      else next.add(ci);
      return next;
    });

  const th = tableClasses.th;
  const td = `${tableClasses.td} align-top`;
  return (
    <div className={tableClasses.wrapper}>
      <table className={tableClasses.table}>
        <thead>
          <tr className={tableClasses.headRow}>
            <th className={`${th} w-8`}><span className="sr-only">{t("appDeps.report.expand")}</span></th>
            <th className={th}>{t("appDeps.report.ci")}</th>
            <th className={th}>{t("appDeps.report.entity")}</th>
            <th className={th}>{t("appDeps.report.owners")}</th>
            <th className={th}>{t("appDeps.report.technologies")}</th>
            <th className={`${th} text-right`} title={t("appDeps.report.upstreamHint")}>{t("appDeps.report.upstream")}</th>
            <th className={`${th} text-right`} title={t("appDeps.report.downstreamHint")}>{t("appDeps.report.downstream")}</th>
            <th className={th}>{t("appDeps.report.criticality")}</th>
            <th className={th} title={t("appDeps.report.inBridgeHint")}>{t("appDeps.report.inBridge")}</th>
            <th className={th}>{t("appDeps.report.repository")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => {
            const isOpen = open.has(r.ci);
            const entry = index.byCi.get(r.ci);
            return (
              <Fragment key={r.ci}>
                <tr className={`${tableClasses.row} ${i % 2 ? tableClasses.rowAlt : ""}`}>
                  <td className={td}>
                    <button
                      type="button"
                      onClick={() => toggle(r.ci)}
                      aria-expanded={isOpen}
                      aria-label={t(isOpen ? "appDeps.report.collapseRow" : "appDeps.report.expandRow", { ci: r.ci })}
                      className="p-1 rounded-[var(--radius-sm)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-overlay)]"
                    >
                      <Icon path={ICON_PATHS.chevronRight} className={`w-3.5 h-3.5 transition-transform duration-150 ${isOpen ? "rotate-90" : ""}`} />
                    </button>
                  </td>
                  <td className={`${td} min-w-56`}>
                    <Link href={appDepsHref(r.ci)} className="font-mono text-sm font-semibold text-[var(--accent)] hover:underline" title={t("appDeps.report.openDiagram")}>
                      {r.ci}
                    </Link>
                    <p className="text-xs text-[var(--text-secondary)] mt-0.5">{r.description || dash}</p>
                  </td>
                  <td className={`${td} text-xs`}>{r.entities.length ? r.entities.join(", ") : dash}</td>
                  <td className={`${td} text-xs`}>{r.owners.length ? r.owners.join(", ") : dash}</td>
                  <td className={td}>
                    {r.technologies.length ? (
                      <div className="flex flex-wrap gap-1 max-w-64">
                        {r.technologies.slice(0, MAX_TECH).map((tech) => <Tag key={tech}>{tech}</Tag>)}
                        {r.technologies.length > MAX_TECH && <span className="text-2xs text-[var(--text-muted)] self-center">+{r.technologies.length - MAX_TECH}</span>}
                      </div>
                    ) : dash}
                  </td>
                  <td className={`${td} text-right font-mono`}>{r.upstream}</td>
                  <td className={`${td} text-right font-mono`}>{r.downstream}</td>
                  <td className={td}><CriticalityBadge value={r.criticality} /></td>
                  <td className={td}>
                    <span className="inline-flex items-center gap-2.5 whitespace-nowrap">
                      {APP_LINK_TYPES.map((tp) => {
                        const n = counts.get(r.ci)?.[tp] ?? 0;
                        return (
                          <span key={tp} title={t(`appDeps.entity.${tp}`)} className={`inline-flex items-center gap-1 font-mono text-xs ${n ? "text-[var(--text-secondary)]" : "text-[var(--text-faint)]"}`}>
                            <Icon path={LINK_ICONS[tp]} className="w-3.5 h-3.5" />
                            <span aria-label={`${t(`appDeps.entity.${tp}`)}: ${n}`}>{n}</span>
                          </span>
                        );
                      })}
                    </span>
                  </td>
                  <td className={`${td} text-xs`}>
                    <a href={r.source.file_url || r.source.web_url} target="_blank" rel="noopener noreferrer" className="font-mono text-[var(--text-secondary)] hover:text-[var(--accent)] inline-flex items-center gap-1">
                      {r.source.project_path}
                      <Icon path={ICON_PATHS.externalLink} className="w-3 h-3 shrink-0" />
                    </a>
                    <p className="font-mono text-2xs text-[var(--text-muted)]">
                      {r.source.ref}{r.source.commit_id ? ` @ ${r.source.commit_id.slice(0, 8)}` : ""}
                    </p>
                  </td>
                </tr>
                {isOpen && entry && (
                  <tr className="bg-[var(--bg-elevated)]/40">
                    <td />
                    <td colSpan={9} className="px-3 pb-3">
                      <AppDepLinksTable rows={linkRows(entry.catalog, index)} />
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
