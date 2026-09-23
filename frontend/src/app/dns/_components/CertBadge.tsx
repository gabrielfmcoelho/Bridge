"use client";

import Badge from "@/components/ui/Badge";
import { useLocale } from "@/contexts/LocaleContext";
import { certState, certTone, certDaysLeft } from "@/lib/dnsCert";
import type { DNSRecord } from "@/lib/types";

type T = (key: string, vars?: Record<string, string>) => string;

/** Plain-text cert status; "" for a record with no https and no scan. Literal
 *  keys on purpose — i18n-check only sees t("…") literals. */
export function certLabel(d: DNSRecord, t: T): string {
  const days = d.cert_expires_at ? String(certDaysLeft(d.cert_expires_at)) : "";
  switch (certState(d)) {
    case "ok": return t("dns.certValid", { days });
    case "warning":
    case "critical": return t("dns.certExpiresIn", { days });
    case "expired": return t("dns.certExpired");
    case "unreachable": return t("dns.certUnreachable");
    case "untrusted": return t("dns.certUntrusted");
    case "unscanned": return t("dns.certUnscanned");
    case "none": return "";
  }
}

export default function CertBadge({ dns }: { dns: DNSRecord }) {
  const { t, formatDate } = useLocale();
  const state = certState(dns);
  if (state === "none") return <span className="text-[var(--text-faint)]">-</span>;
  const title = dns.cert_error || (dns.cert_expires_at ? formatDate(dns.cert_expires_at) : undefined);
  return (
    <span title={title} className="inline-flex">
      <Badge color={certTone(state)}>{certLabel(dns, t)}</Badge>
    </span>
  );
}
