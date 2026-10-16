"use client";

import Badge from "@/components/ui/Badge";
import Lozenge from "@/components/ui/Lozenge";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import { VULN_SEVERITY_BADGE, VULN_STATUS_LOZENGE } from "@/lib/alert-colors";
import { parseVia } from "@/lib/vulns";
import type { VulnFinding } from "@/lib/types";

/** Findings as rows, already ordered by the server (most severe first).
 *  showVia adds the "how it reached this entity" column of entity tabs. */
export default function VulnTable({ rows, onOpen, showVia }: { rows: VulnFinding[]; onOpen: (id: number) => void; showVia?: boolean }) {
  const { t, formatDate } = useLocale();
  return (
    <div className={tableClasses.wrapper}>
      <table className={tableClasses.table}>
        <thead>
          <tr className={tableClasses.headRow}>
            <th className={tableClasses.th}>{t("vuln.severity")}</th>
            <th className={tableClasses.th}>{t("vuln.finding")}</th>
            <th className={tableClasses.th}>{t("vuln.target")}</th>
            {showVia && <th className={tableClasses.th}>{t("vuln.viaColumn")}</th>}
            <th className={tableClasses.th}>{t("common.status")}</th>
            <th className={tableClasses.th}>{t("vuln.lastSeen")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((f, i) => {
            const via = parseVia(f.via);
            return (
              <tr
                key={f.id}
                className={`${tableClasses.row} cursor-pointer ${i % 2 === 1 ? tableClasses.rowAlt : ""}`}
                onClick={() => onOpen(f.id)}
              >
                <td className={tableClasses.td}>
                  <Badge color={VULN_SEVERITY_BADGE[f.severity]}>{t(`vuln.severityLevel.${f.severity}`)}</Badge>
                </td>
                <td className={`${tableClasses.td} max-w-0 w-[40%]`}>
                  <span className="block truncate font-medium text-[var(--text-primary)]" title={f.title}>{f.title}</span>
                  <span className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
                    {f.cve && <span className="font-mono">{f.cve}</span>}
                    {f.known_exploited && <span className="text-[var(--danger)] font-semibold">{t("vuln.knownExploited")}</span>}
                    <span>{f.scanner}</span>
                  </span>
                </td>
                <td className={`${tableClasses.td} max-w-0 w-[25%]`}>
                  <span className="block truncate font-mono text-xs text-[var(--text-secondary)]" title={f.target}>{f.target || "—"}</span>
                  {f.vm_hint && <span className="block truncate text-xs text-[var(--text-muted)]" title={f.vm_hint}>{f.vm_hint}</span>}
                </td>
                {showVia && (
                  <td className={`${tableClasses.td} text-xs text-[var(--text-muted)] max-w-0 w-[15%]`}>
                    <span className="block truncate" title={via?.name}>
                      {via ? t("vuln.via", { type: t(`vuln.entity.${via.type}`), name: via.name }) : t("vuln.direct")}
                    </span>
                  </td>
                )}
                <td className={tableClasses.td}>
                  <Lozenge appearance={VULN_STATUS_LOZENGE[f.status]}>{t(`vuln.status.${f.status}`)}</Lozenge>
                </td>
                <td className={`${tableClasses.td} text-[var(--text-muted)] whitespace-nowrap`}>{formatDate(f.last_seen_at)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
