"use client";

import Textarea from "@/components/ui/Textarea";
import Lozenge, { type LozengeAppearance } from "@/components/ui/Lozenge";
import Button from "@/components/ui/Button";
import SectionHeading from "@/components/ui/SectionHeading";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import type { VMInfoType } from "@/lib/api";
import { applySuggestion, backupPreview, formatBytes, parsePaths, type FSState } from "@/lib/backupPreview";

const FS_TONE: Record<FSState, LozengeAppearance> = { included: "success", partial: "inprogress", excluded: "default", network: "moved", outside: "default" };

/** Include / exclude of a host backup, with what they resolve to and, from
 *  the host's latest scan, what the backup would take and what to leave out.
 *  An empty field inherits (`inherited`, from `from`). */
export default function BackupPathsEditor({ include, exclude, onChange, inherited, from, scan, lastSize }: {
  include: string;
  exclude: string;
  onChange: (include: string, exclude: string) => void;
  inherited: { include: string[]; exclude: string[] };
  /** Where an empty field's value comes from. */
  from: "defaults" | "host";
  /** The host's latest scan; none = no preview (several hosts, or never scanned). */
  scan?: Partial<VMInfoType>;
  /** The newest kept backup's size: the realistic estimate. */
  lastSize?: number;
}) {
  const { t } = useLocale();
  const inc = parsePaths(include);
  const exc = parsePaths(exclude);
  const effInclude = inc.length ? inc : inherited.include;
  const effExclude = exc.length ? exc : inherited.exclude;
  const relative = [...inc, ...exc].find((p) => !p.startsWith("/"));
  const preview = scan ? backupPreview(scan, effInclude, effExclude) : null;
  const inheritHint = t(`host.backup.inherits.${from}`);

  return (
    <div className="space-y-3">
      <Textarea label={t("settings.integrations.minio.include")} rows={2} value={include} placeholder={inherited.include.join(" ")}
        onChange={(e) => onChange(e.target.value, exclude)} hint={inheritHint} error={relative && inc.includes(relative) ? t("host.osRestore.absolute") : undefined} />
      <Textarea label={t("settings.integrations.minio.exclude")} rows={3} value={exclude} placeholder={inherited.exclude.join(" ")}
        onChange={(e) => onChange(include, e.target.value)} hint={`${inheritHint} ${t("host.backup.excludeWins")}`}
        error={relative && exc.includes(relative) ? t("host.osRestore.absolute") : undefined} />

      {preview && (
        <div className="space-y-2">
          <SectionHeading as="h3">{t("host.backup.preview")}</SectionHeading>
          {preview.filesystems.length > 0 && (
            <div className={tableClasses.wrapper}>
              <table className={tableClasses.table}>
                <tbody>
                  {preview.filesystems.map((f) => (
                    <tr key={f.mount} className={tableClasses.row}>
                      <td className={`${tableClasses.td} font-mono text-xs`}>{f.mount} <span className="text-[var(--text-muted)]">{f.fstype}</span></td>
                      <td className={`${tableClasses.td} text-xs tabular-nums text-right`}>{formatBytes(f.used)}</td>
                      <td className={tableClasses.td}><Lozenge appearance={FS_TONE[f.state]}>{t(`host.backup.fs.${f.state}`)}</Lozenge></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="text-xs text-[var(--text-secondary)]">
            {t("host.backup.estimate", { size: formatBytes(preview.upperBound) })}
            {lastSize != null && ` ${t("host.backup.lastSize", { size: formatBytes(lastSize) })}`}
          </p>
          {preview.suggestions.map((s) => (
            <div key={`${s.kind}:${s.subject}`} className="flex items-start justify-between gap-3 rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--warning)]/5 px-3 py-2">
              <p className="text-xs text-[var(--text-secondary)]">
                {t(`host.backup.suggest.${s.kind}`, { subject: s.subject, size: s.bytes ? formatBytes(s.bytes) : "", paths: s.addExclude.join(" ") })}
              </p>
              <Button size="sm" variant="secondary" onClick={() => onChange(include, applySuggestion(effExclude, s).join(" "))}>
                {t("host.backup.apply")}
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
