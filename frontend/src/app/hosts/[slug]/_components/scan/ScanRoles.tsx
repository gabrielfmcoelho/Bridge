"use client";

import SectionCard from "@/components/ui/SectionCard";
import type { VMInfoType } from "@/lib/api";
import { Detail, type T } from "./shared";

/**
 * A11 mail / DNS role: what a generic "infrastructure" VM actually is — a mail
 * server (Postfix/Dovecot), a DNS server (BIND), both or neither. Only config
 * domain names and counts: never a recipient address, a client IP or a mailbox
 * username. Background scan only.
 */
export default function ScanRoles({ info, t }: { info: VMInfoType; t: T }) {
  const r = info.deep_facts?.roles;
  const empty = info.deep_facts == null
    ? t("scan.deep.notRun")
    : !r
      ? t("scan.pane.empty.roles")
      : undefined;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.roles")} empty={empty}>
      {r && !empty && (
        <div className="space-y-3">
          <p className="text-2xs text-[var(--text-muted)]">{t("scan.deep.roles.summary")}</p>
          <div className="flex flex-wrap gap-1.5">
            {r.mail && <RoleChip label={t("scan.deep.roles.mail")} tone="info" />}
            {r.dns && (
              <RoleChip
                label={r.dns.authoritative && r.dns.authoritative.length > 0
                  ? t("scan.deep.roles.dnsAuthoritative")
                  : t("scan.deep.roles.dnsResolver")}
                tone="info"
              />
            )}
          </div>

          {r.mail && (
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
              <Detail label={t("scan.deep.roles.daemons")}>
                <span className="inline-flex flex-wrap gap-1.5">
                  {r.mail.postfix && <Chip>Postfix</Chip>}
                  {r.mail.dovecot && <Chip>Dovecot</Chip>}
                </span>
              </Detail>
              {r.mail.destinations && r.mail.destinations.length > 0 && (
                <Detail label={t("scan.deep.roles.destinations")}>
                  <span className="inline-flex flex-wrap gap-1.5">
                    {r.mail.destinations.map((d) => <Chip key={d} mono>{d}</Chip>)}
                  </span>
                </Detail>
              )}
              {r.mail.relayhost && (
                <Detail label={t("scan.deep.roles.relayhost")}>
                  <span className="font-mono text-2xs">{r.mail.relayhost}</span>
                </Detail>
              )}
              {(r.mail.sent || r.mail.bounced || r.mail.deferred) ? (
                <Detail label={t("scan.deep.roles.delivery")}>
                  {t("scan.deep.roles.deliveryCounts", {
                    sent: (r.mail.sent ?? 0).toLocaleString(),
                    bounced: (r.mail.bounced ?? 0).toLocaleString(),
                    deferred: (r.mail.deferred ?? 0).toLocaleString(),
                  })}
                </Detail>
              ) : null}
              {(r.mail.imap_logins || r.mail.pop3_logins) ? (
                <Detail label={t("scan.deep.roles.logins")}>
                  {t("scan.deep.roles.loginCounts", {
                    imap: (r.mail.imap_logins ?? 0).toLocaleString(),
                    pop3: (r.mail.pop3_logins ?? 0).toLocaleString(),
                  })}
                </Detail>
              ) : null}
            </dl>
          )}

          {r.dns && (
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
              {r.dns.authoritative && r.dns.authoritative.length > 0 && (
                <Detail label={t("scan.deep.roles.zones")}>
                  <span className="inline-flex flex-wrap gap-1.5">
                    {r.dns.authoritative.map((z) => <Chip key={z} mono>{z}</Chip>)}
                  </span>
                </Detail>
              )}
              <Detail label={t("scan.deep.roles.recursion")}>
                {t(r.dns.recursion ? "scan.deep.roles.recursionOn" : "scan.deep.roles.recursionOff")}
              </Detail>
            </dl>
          )}
        </div>
      )}
    </SectionCard>
  );
}

function RoleChip({ label, tone }: { label: string; tone: "info" }) {
  const cls = tone === "info" ? "bg-[var(--info)]/10 text-[var(--info)]" : "";
  return <span className={`rounded px-2 py-0.5 text-xs ${cls}`}>{label}</span>;
}

function Chip({ children, mono }: { children: React.ReactNode; mono?: boolean }) {
  return (
    <span className={`rounded px-1.5 text-2xs bg-[var(--bg-elevated)] text-[var(--text-secondary)] ${mono ? "font-mono" : ""}`}>
      {children}
    </span>
  );
}
