"use client";

import SectionCard from "@/components/ui/SectionCard";
import { useLocale } from "@/contexts/LocaleContext";
import type { SiteTraffic, VMInfoType } from "@/lib/api";
import { Detail, Row, Rows, type T } from "./shared";

/**
 * A2 site traffic: per web site, "is anyone actually using it?". 30-day
 * access-log aggregates (requests/day, status classes, browser-vs-bot share,
 * top path sections) and recent error-log categories — a live site shows
 * thousands of requests, an abandoned VM zero. Everything is a count or a
 * first-path-segment label; no client IP, query string or User-Agent ever
 * leaves the VM. Background scan only.
 */
export default function ScanTraffic({ info, t }: { info: VMInfoType; t: T }) {
  const sites = info.deep_facts?.sites ?? [];
  const empty = info.deep_facts == null
    ? t("scan.deep.notRun")
    : sites.length === 0
      ? t("scan.pane.empty.traffic")
      : undefined;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.traffic")} empty={empty}>
      {!empty && <p className="text-2xs text-[var(--text-muted)] mb-3">{t("scan.deep.traffic.summary")}</p>}
      {sites.length > 0 && (
        <Rows>
          {sites.map((s, i) => <SiteRow key={i} s={s} t={t} />)}
        </Rows>
      )}
    </SectionCard>
  );
}

function SiteRow({ s, t }: { s: SiteTraffic; t: T }) {
  const { formatDateTime } = useLocale();
  const req = s.requests ?? 0;
  const idle = req === 0;
  const chip = "shrink-0 rounded px-1.5 text-2xs";

  return (
    <Row
      summary={<>
        <span className="min-w-0 truncate font-mono text-[var(--text-primary)]" title={s.access_log}>{s.site}</span>
        {s.server && <span className={`${chip} bg-[var(--bg-elevated)] text-[var(--text-muted)]`}>{s.server}</span>}
        <span className="ml-auto shrink-0 tabular-nums text-[var(--text-secondary)]">
          {idle
            ? <span className="text-[var(--text-muted)]">{t("scan.deep.traffic.idle")}</span>
            : t("scan.deep.traffic.perDay", { count: (s.per_day ?? 0).toLocaleString() })}
        </span>
        {!idle && (
          <span className={`${chip} bg-[var(--success)]/10 text-[var(--success)]`}>
            {t("scan.deep.traffic.requests", { count: req.toLocaleString() })}
          </span>
        )}
      </>}
    >
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
        {!idle && (
          <Detail label={t("scan.deep.traffic.status")}>
            <span className="inline-flex flex-wrap gap-1.5">
              <StatusChip label="2xx" n={s.status_2xx} tone="success" />
              <StatusChip label="3xx" n={s.status_3xx} tone="muted" />
              <StatusChip label="4xx" n={s.status_4xx} tone="warning" />
              <StatusChip label="5xx" n={s.status_5xx} tone="danger" />
            </span>
          </Detail>
        )}
        {!idle && (
          <Detail label={t("scan.deep.traffic.who")}>
            {t("scan.deep.traffic.humanBot", {
              human: (s.human ?? 0).toLocaleString(),
              bot: (s.bot ?? 0).toLocaleString(),
            })}
          </Detail>
        )}
        {!idle && (s.public_ips || s.private_ips) ? (
          <Detail label={t("scan.deep.traffic.clients")}>
            {t("scan.deep.traffic.pubPriv", {
              pub: (s.public_ips ?? 0).toLocaleString(),
              priv: (s.private_ips ?? 0).toLocaleString(),
            })}
          </Detail>
        ) : null}
        {s.top_paths && s.top_paths.length > 0 && (
          <Detail label={t("scan.deep.traffic.paths")}>
            <span className="inline-flex flex-col gap-0.5">
              {s.top_paths.map((p, i) => (
                <span key={i} className="flex gap-2">
                  <span className="min-w-0 truncate">{p.path}</span>
                  <span className="tabular-nums text-[var(--text-muted)]">{p.count.toLocaleString()}</span>
                </span>
              ))}
            </span>
          </Detail>
        )}
        {s.errors && s.errors.length > 0 && (
          <Detail label={t("scan.deep.traffic.errors")}>
            <span className="inline-flex flex-wrap gap-1.5">
              {s.errors.map((e, i) => (
                <span key={i} className="rounded px-1.5 text-2xs bg-[var(--danger)]/10 text-[var(--danger)]">
                  {e.category} {e.count.toLocaleString()}
                </span>
              ))}
            </span>
          </Detail>
        )}
        {s.last_seen ? (
          <Detail label={t("scan.deep.traffic.lastSeen")} mono={false}>
            {formatDateTime(new Date(s.last_seen * 1000))}
          </Detail>
        ) : null}
        {s.access_log && <Detail label={t("scan.deep.traffic.log")}>{s.access_log}</Detail>}
        {s.partial && (
          <Detail label="" mono={false}>
            <span className="text-2xs text-[var(--text-muted)]">{t("scan.deep.traffic.partial")}</span>
          </Detail>
        )}
      </dl>
    </Row>
  );
}

// Literal class strings per tone — Tailwind's scanner can't see a class built
// from a template variable.
const STATUS_TONE: Record<"success" | "warning" | "danger" | "muted", string> = {
  success: "bg-[var(--success)]/10 text-[var(--success)]",
  warning: "bg-[var(--warning)]/10 text-[var(--warning)]",
  danger: "bg-[var(--danger)]/10 text-[var(--danger)]",
  muted: "bg-[var(--bg-elevated)] text-[var(--text-muted)]",
};

function StatusChip({ label, n, tone }: { label: string; n?: number; tone: keyof typeof STATUS_TONE }) {
  if (!n) return null;
  return <span className={`rounded px-1.5 text-2xs tabular-nums ${STATUS_TONE[tone]}`}>{label} {n.toLocaleString()}</span>;
}
