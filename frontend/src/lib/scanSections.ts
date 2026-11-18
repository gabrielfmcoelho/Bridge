// No runtime imports: Node's test runner loads this file directly.
import type { VMInfoType } from "./api";

/**
 * The scan pane's groups, each answering one question, and the sections
 * (one card each) they hold, in display order:
 *   resources  how loaded is it?
 *   workloads  what runs here?
 *   web        which sites does it serve?
 *   access     who gets in, and what is exposed?
 *   system     which OS, and how well kept?
 */
export const GROUP_SECTIONS = {
  resources: ["resources"],
  workloads: ["ports", "containers", "services", "processes", "git", "roles"],
  web: ["webservers", "traffic", "code", "panels"],
  access: ["users", "owner", "security", "firewall", "outbound"],
  system: ["system", "patch", "packages"],
} as const;

export type ScanGroup = keyof typeof GROUP_SECTIONS;
export type ScanSection = (typeof GROUP_SECTIONS)[ScanGroup][number];
export const SCAN_GROUPS = Object.keys(GROUP_SECTIONS) as ScanGroup[];

/** `empty`: the scan looked and found nothing. `notRun`: it did not look. */
export type SectionState = "data" | "empty" | "notRun";

const has = (v: unknown[] | null | undefined) => (v?.length ?? 0) > 0;

/** Deep facts only come from a background scan: without them, not checked. */
function deep(info: VMInfoType, found: (d: NonNullable<VMInfoType["deep_facts"]>) => boolean): SectionState {
  if (info.deep_facts == null) return "notRun";
  return found(info.deep_facts) ? "data" : "empty";
}

/** Whether a section has something to show. Each rule mirrors its component. */
export function sectionState(info: VMInfoType, s: ScanSection): SectionState {
  const data = (ok: unknown): SectionState => (ok ? "data" : "empty");
  switch (s) {
    case "resources": {
      const top = info.resource_top;
      return data(info.cpu_usage || info.ram_percent || info.disk_percent || has(info.filesystems)
        || has(top?.top_cpu) || has(top?.top_mem) || has(top?.top_disk));
    }
    case "ports": return data(has(info.ports));
    case "containers":
      if (has(info.parsed_containers) || has(info.container_stats)) return "data";
      return info.containers_known === false ? "notRun" : "empty";
    case "services":
      return data(has(info.agents) || has(info.service_inventory) || (!info.service_inventory && has(info.systemd_services)));
    case "processes": return data(has(info.process_details) || has(info.service_details) || has(info.services));
    case "git":
      if (has(info.git_remotes)) return "data";
      return info.git_remotes == null || info.git_remotes_partial ? "notRun" : "empty";
    case "roles": return deep(info, (d) => !!d.roles);
    case "webservers": return data(has(info.web_servers));
    case "traffic": return deep(info, (d) => has(d.sites));
    case "code": return deep(info, (d) => has(d.code));
    case "panels": return deep(info, (d) => !!d.panels);
    case "users": return data(has(info.remote_users) || has(info.ssh_keys));
    case "owner": return deep(info, (d) => !!d.owner);
    case "security": return data(info.ssh_auth_policy);
    // The quick test's raw status shows here too, so a firewall has one card.
    case "firewall": return info.firewall_status ? "data" : deep(info, (d) => !!d.firewall);
    case "outbound": return deep(info, (d) => has(d.outbound?.scripts) || has(d.outbound?.connections));
    case "system":
      return data(info.os || info.kernel || info.arch || info.reboot_required || info.uptime || info.hostname_remote
        || info.public_ip || info.load_avg || info.logged_users || info.swap_total);
    case "patch": return deep(info, (d) => !!d.patch?.manager);
    case "packages": return data(has(info.installed_packages) || info.cron || has(info.cron_jobs));
  }
}

/**
 * What the pane draws: each group with the sections that have data (a group
 * with none is left out), then the sections with nothing to show, split by why.
 */
export function scanLayout(info: VMInfoType): {
  groups: { group: ScanGroup; sections: ScanSection[] }[];
  empty: ScanSection[];
  notRun: ScanSection[];
} {
  const groups: { group: ScanGroup; sections: ScanSection[] }[] = [];
  const empty: ScanSection[] = [];
  const notRun: ScanSection[] = [];
  for (const group of SCAN_GROUPS) {
    const sections: ScanSection[] = [];
    for (const s of GROUP_SECTIONS[group]) {
      const state = sectionState(info, s);
      (state === "data" ? sections : state === "empty" ? empty : notRun).push(s);
    }
    if (sections.length > 0) groups.push({ group, sections });
  }
  return { groups, empty, notRun };
}
