"use client";

import SectionCard from "@/components/ui/SectionCard";
import { useLocale } from "@/contexts/LocaleContext";
import type { CodeFingerprint, VMInfoType } from "@/lib/api";
import { formatBytes } from "@/lib/backupPreview";
import { siteFacts } from "@/lib/scanSections";
import { Detail, Row, Rows, type T } from "./shared";

/**
 * A7 code fingerprint: per web docroot, which framework and version runs
 * (CodeIgniter, Laravel, WordPress…), the newest code change, and the upload
 * directories (size, file count, newest) — the "what runs here / how much data
 * is at rest" signal. Background scan only. A docroot a web-server site serves
 * shows on that site's row (ScanWebServers); this card keeps the rest.
 */
export default function ScanCode({ info, t }: { info: VMInfoType; t: T }) {
  const docs = siteFacts(info).code;
  const empty = info.deep_facts == null
    ? t("scan.deep.notRun")
    : docs.length === 0
      ? t("scan.pane.empty.code")
      : undefined;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.code")} empty={empty}>
      {docs.length > 0 && (
        <div className="space-y-3">
          <p className="text-2xs text-[var(--text-muted)]">{t("scan.deep.code.summary")}</p>
          <Rows>
            {docs.map((d, i) => <DocRow key={i} d={d} t={t} />)}
          </Rows>
        </div>
      )}
    </SectionCard>
  );
}

function DocRow({ d, t }: { d: CodeFingerprint; t: T }) {
  return (
    <Row
      summary={<>
        <span className="min-w-0 truncate font-mono text-[var(--text-primary)]" title={d.docroot}>{d.docroot}</span>
        <span className="ml-auto flex shrink-0 flex-wrap justify-end gap-1"><CodeChips d={d} /></span>
      </>}
    >
      <CodeDetails d={d} t={t} />
    </Row>
  );
}

/** The frameworks found under a docroot, as chips (four at most). */
export function CodeChips({ d }: { d: CodeFingerprint }) {
  return (
    <>
      {(d.apps ?? []).slice(0, 4).map((a, i) => (
        <span key={i} className="shrink-0 rounded px-1.5 text-2xs bg-[var(--accent)]/10 text-[var(--accent)]">
          {a.framework}{a.version ? ` ${a.version}` : ""}
        </span>
      ))}
    </>
  );
}

/** A docroot's frameworks per path, newest code change and upload directories. */
export function CodeDetails({ d, t }: { d: CodeFingerprint; t: T }) {
  const { formatDate } = useLocale();
  const apps = d.apps ?? [];
  return (
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
        {apps.length > 0 ? (
          apps.map((a, i) => (
            <Detail key={i} label={a.path ? `/${a.path}` : t("scan.deep.code.root")}>
              {a.framework}{a.version ? ` ${a.version}` : ""}
            </Detail>
          ))
        ) : (
          <Detail label="" mono={false}><span className="text-[var(--text-muted)]">{t("scan.deep.code.noApps")}</span></Detail>
        )}
        {d.newest_mtime ? (
          <Detail label={t("scan.deep.code.newest")} mono={false}>{formatDate(new Date(d.newest_mtime * 1000))}</Detail>
        ) : null}
        {d.uploads && d.uploads.length > 0 && (
          <Detail label={t("scan.deep.code.uploads")}>
            <span className="inline-flex flex-col gap-0.5">
              {d.uploads.map((u, i) => (
                <span key={i} className="flex flex-wrap gap-2">
                  <span className="min-w-0 truncate">{u.dir}</span>
                  {u.size_bytes ? <span className="tabular-nums text-[var(--text-muted)]">{formatBytes(u.size_bytes)}</span> : null}
                  {u.files ? <span className="tabular-nums text-[var(--text-faint)]">{t("scan.deep.code.files", { count: u.files.toLocaleString() })}</span> : null}
                  {u.newest ? <span className="text-[var(--text-faint)]">{formatDate(new Date(u.newest * 1000))}</span> : null}
                </span>
              ))}
            </span>
          </Detail>
        )}
      </dl>
  );
}
