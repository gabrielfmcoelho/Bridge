// How a service reads in cards, headers and relation rows. Scan-born rows
// are named after their container ("a1rwyyxk4rby396m…-1433"). When Coolify
// runs it, its Coolify name ("Gestor - API") is the title; else, when the scan
// recognised the software, its catalog label ("PostgreSQL") is. Either way the
// container name steps down to a mono id. An operator-edited nickname always
// wins.
import type { Service } from "@/lib/types";

/** Categories, in the order filters and dashboards list them. Keys match
 *  services.service_kind (the scan catalog's Kind + container rules). */
export const SERVICE_KINDS = [
  "database", "cache", "queue", "web", "proxy", "runtime", "orchestration",
  "analytics", "monitoring", "logging", "mail", "dns", "directory",
  "file-sharing", "platform", "agent", "app",
] as const;

type Named = Pick<Service, "nickname" | "service_subtype" | "discovery_kind" | "container_name" | "discovery_key" | "source" | "coolify_stack">;

// Coolify container names: optional role, the 24-char resource uuid, optional
// deploy timestamp (mirrors containerName in internal/integrations/coolify).
const COOLIFY_CONTAINER = /^(?:(.+)-)?[a-z0-9]{24}(?:-\d{12})?$/;

/** The name Coolify shows for a container: its resource ("Gestor - API"),
 *  plus the compose role for a stack member ("Infra - Airflow · flower"). */
export function coolifyName(stack: string, containerName: string): string {
  // "<uuid>-proxy": the container exposing a database's public port.
  const role = /^[a-z0-9]{24}-proxy$/.test(containerName) ? "proxy" : COOLIFY_CONTAINER.exec(containerName)?.[1];
  return role ? `${stack} · ${role}` : stack;
}

export function serviceTitle(s: Named): { title: string; mono: boolean; id?: string } {
  const scanNamed = s.source !== "manual" && s.discovery_kind === "container" &&
    (s.nickname === s.container_name || s.nickname === s.discovery_key);
  if (!scanNamed) return { title: s.nickname, mono: false };
  const id = s.container_name || s.discovery_key;
  if (s.coolify_stack) return { title: coolifyName(s.coolify_stack, id), mono: false, id };
  return s.service_subtype ? { title: s.service_subtype, mono: false, id } : { title: s.nickname, mono: true };
}

/** i18n key of a service's origin: scan-found container/host, manual, fixed. */
export function originKey(s: Pick<Service, "source" | "discovery_kind">): string {
  if (s.source === "manual" || !s.source) return "service.origin.manual";
  if (s.source === "fixed") return "service.origin.fixed";
  return s.discovery_kind === "host" ? "service.origin.host" : "service.origin.container";
}
