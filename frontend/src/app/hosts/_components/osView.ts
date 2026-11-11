// Pure helpers of the hosts page's OS tab (runtime-import free, so Node's
// test runner can load them).
import type { DependencyGroup } from "@/lib/types";

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
