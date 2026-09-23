import type { DNSRecord } from "./types";

// ── TLS certificate state for DNS records ──────────────────────────────────
// Pure functions only — no React. Days-left is computed here at read time
// from the stored expiry, so the buckets stay correct between scans.
// matchesCertFilter MUST mirror the `cert` buckets in dnsWhere
// (internal/store/dns.go): the card view filters client-side, the table view
// server-side, and both have to show the same records.

type CertFields = Pick<DNSRecord, "has_https" | "cert_expires_at" | "cert_error" | "cert_checked_at">;

export type CertState = "none" | "unscanned" | "unreachable" | "expired" | "critical" | "warning" | "untrusted" | "ok";

const DAY = 86_400_000;

export function certState(d: CertFields, now = Date.now()): CertState {
  if (!d.cert_checked_at) return d.has_https ? "unscanned" : "none";
  if (!d.cert_expires_at) return "unreachable";
  const ms = Date.parse(d.cert_expires_at) - now;
  if (ms < 0) return "expired";
  if (ms < 7 * DAY) return "critical";
  if (ms < 30 * DAY) return "warning";
  return d.cert_error ? "untrusted" : "ok";
}

/** Badge / indicator tone for a state. */
export function certTone(state: CertState): "success" | "warning" | "danger" | "default" {
  switch (state) {
    case "ok": return "success";
    case "warning":
    case "untrusted": return "warning";
    case "critical":
    case "expired":
    case "unreachable": return "danger";
    default: return "default";
  }
}

export function certDaysLeft(expiresAt: string, now = Date.now()): number {
  return Math.floor((Date.parse(expiresAt) - now) / DAY);
}

function expiresWithin(d: CertFields, days: number, now: number): boolean {
  if (!d.cert_expires_at) return false;
  const exp = Date.parse(d.cert_expires_at);
  return exp >= now && exp < now + days * DAY;
}

/** filter ∈ "" | expired | 7 | 30 | error | unscanned — "30" includes the 7-day ones. */
export function matchesCertFilter(d: CertFields, filter: string, now = Date.now()): boolean {
  switch (filter) {
    case "expired": return !!d.cert_expires_at && Date.parse(d.cert_expires_at) < now;
    case "7": return expiresWithin(d, 7, now);
    case "30": return expiresWithin(d, 30, now);
    case "error": return !!d.cert_error;
    case "unscanned": return d.has_https && !d.cert_checked_at;
    default: return true;
  }
}

/** Sort by expiry with records that have none last, in both directions. */
export function compareCertExpiry(a: CertFields, b: CertFields, dir: "asc" | "desc"): number {
  const x = a.cert_expires_at ? Date.parse(a.cert_expires_at) : null;
  const y = b.cert_expires_at ? Date.parse(b.cert_expires_at) : null;
  if (x === null || y === null) return x === y ? 0 : x === null ? 1 : -1;
  return dir === "desc" ? y - x : x - y;
}
