// Web servers from the host scan ↔ the host's DNS records. Runtime-import
// free so node --test can run webDns.test.ts.
import type { FPMPool, WebServer } from "@/lib/api";

const DNS_NAME = /^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9-]{2,63}$/;

/**
 * A server_name normalized to a DNS name, or null when it can't be one
 * (`_`, localhost, an IP, a wildcard or regex). Twin of service.LinkableName
 * in internal/service/webdns.go — keep the rules identical.
 */
export function linkableName(raw: string): string | null {
  const n = raw.trim().toLowerCase().replace(/\.$/, "");
  if (!n || n === "_" || n === "localhost" || n.endsWith(".localhost") || /[*~]/.test(n)) return null;
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(n) || n.includes(":")) return null;
  return DNS_NAME.test(n) ? n : null;
}

export type NameStatus =
  | { kind: "linked"; dnsId: number }
  | { kind: "unlinked" } // valid but not linked: didn't resolve, in the trash, or stopped server
  | { kind: "ignored" }; // not a DNS name

/** How one served name stands against the DNS records linked to the host. */
export function nameStatus(raw: string, linked: Map<string, number>): NameStatus {
  const n = linkableName(raw);
  if (!n) return { kind: "ignored" };
  const id = linked.get(n);
  return id ? { kind: "linked", dnsId: id } : { kind: "unlinked" };
}

/** domain (lowercase) → id, for nameStatus. */
export function linkedIndex(records: { id: number; domain: string }[]): Map<string, number> {
  return new Map(records.map((r) => [r.domain.toLowerCase(), r.id]));
}

/** Linked DNS records that no web server on the host declares. */
export function undeclaredRecords<T extends { domain: string }>(records: T[], servers: WebServer[]): T[] {
  const served = new Set<string>();
  for (const s of servers) for (const site of s.sites ?? []) for (const n of site.names ?? []) {
    const l = linkableName(n);
    if (l) served.add(l);
  }
  return records.filter((r) => !served.has(r.domain.toLowerCase()));
}

/**
 * Names of the sites (across all servers) whose fastcgi_pass reaches this
 * pool: same socket path, or same port ("9000" ↔ "php:9000" / "127.0.0.1:9000").
 */
export function poolUsers(pool: FPMPool, servers: WebServer[]): string[] {
  const listen = (pool.listen ?? "").trim();
  if (!listen) return [];
  const port = listen.match(/(?:^|:)(\d+)$/)?.[1];
  const out: string[] = [];
  for (const s of servers) for (const site of s.sites ?? []) {
    const hit = (site.fastcgi ?? []).some((f) => {
      if (f.startsWith("unix:")) return f.slice(5) === listen;
      return !!port && f.match(/:(\d+)$/)?.[1] === port;
    });
    if (hit) for (const n of site.names ?? []) if (n !== "_" && !out.includes(n)) out.push(n);
  }
  return out;
}
