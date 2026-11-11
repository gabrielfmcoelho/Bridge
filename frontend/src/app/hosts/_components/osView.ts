// Pure helpers of the hosts page's OS tab (runtime-import free, so Node's
// test runner can load them).
import type { DependencyGroup, Host } from "@/lib/types";

/** By distro name, then newest version first ("24.04" before "22.04", "12" before "9"). */
export function sortOSGroups(groups: DependencyGroup[]): DependencyGroup[] {
  return [...groups].sort((a, b) => a.name.localeCompare(b.name) || b.version.localeCompare(a.version, undefined, { numeric: true }));
}

export interface OSFamily { name: string; hosts: number; eolHosts: number }

/** Hosts per distro (each host has one OS row, so version counts add up), most hosts first. */
export function osFamilies(groups: DependencyGroup[]): OSFamily[] {
  const m = new Map<string, OSFamily>();
  for (const g of groups) {
    const f = m.get(g.name) ?? { name: g.name, hosts: 0, eolHosts: 0 };
    f.hosts += g.hosts;
    if (g.eol) f.eolHosts += g.hosts;
    m.set(g.name, f);
  }
  return [...m.values()].sort((a, b) => b.hosts - a.hosts || a.name.localeCompare(b.name));
}

/** "ubuntu|22.04": the key of an OS group and of the hosts that run it. */
export const osKey = (name: string, version: string) => `${name}|${version}`;

/** Host ids per OS group key, from the full host list (group instances stop
 *  at 50, so a run's targets come from here). */
export function hostIdsByOS(hosts: Host[]): Map<string, number[]> {
  const m = new Map<string, number[]>();
  for (const h of hosts) {
    if (!h.os?.id) continue;
    const k = osKey(h.os.id, h.os.version ?? "");
    m.set(k, [...(m.get(k) ?? []), h.id]);
  }
  return m;
}
