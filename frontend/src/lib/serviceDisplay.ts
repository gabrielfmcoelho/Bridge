// How a service reads in cards, headers and relation rows. Scan-born rows
// are named after their container ("a1rwyyxk4rby396m…-1433"); when the scan
// recognised the software, its catalog label ("PostgreSQL") is the title and
// the container name steps down to a mono id. An operator-edited nickname
// always wins.
import type { Service } from "@/lib/types";

/** Categories, in the order filters and dashboards list them. Keys match
 *  services.service_kind (the scan catalog's Kind + container rules). */
export const SERVICE_KINDS = [
  "database", "cache", "queue", "web", "proxy", "runtime", "orchestration",
  "analytics", "monitoring", "logging", "mail", "dns", "directory",
  "file-sharing", "platform", "agent", "app",
] as const;

type Named = Pick<Service, "nickname" | "service_subtype" | "discovery_kind" | "container_name" | "discovery_key" | "source">;

export function serviceTitle(s: Named): { title: string; mono: boolean; id?: string } {
  const scanNamed = s.source !== "manual" && s.discovery_kind === "container" &&
    (s.nickname === s.container_name || s.nickname === s.discovery_key);
  if (!scanNamed) return { title: s.nickname, mono: false };
  const id = s.container_name || s.discovery_key;
  return s.service_subtype ? { title: s.service_subtype, mono: false, id } : { title: s.nickname, mono: true };
}

/** i18n key of a service's origin: scan-found container/host, manual, fixed. */
export function originKey(s: Pick<Service, "source" | "discovery_kind">): string {
  if (s.source === "manual" || !s.source) return "service.origin.manual";
  if (s.source === "fixed") return "service.origin.fixed";
  return s.discovery_kind === "host" ? "service.origin.host" : "service.origin.container";
}
