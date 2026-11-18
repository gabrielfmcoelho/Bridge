// No runtime imports: Node's test runner loads this file directly.
import type { CodeFingerprint, DiscoveredService, ParsedContainer, ProcessDetail, SiteTraffic, VMInfoType, WebSite } from "./api";

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
  workloads: ["workloads", "agents", "ports", "git", "roles"],
  web: ["webservers", "traffic", "code", "panels"],
  access: ["users", "owner", "security", "firewall", "outbound"],
  system: ["system", "patch", "packages"],
} as const;

export type ScanGroup = keyof typeof GROUP_SECTIONS;
export type ScanSection = (typeof GROUP_SECTIONS)[ScanGroup][number];
export const SCAN_GROUPS = Object.keys(GROUP_SECTIONS) as ScanGroup[];

/**
 * `empty`: the scan looked and found nothing. `notRun`: it did not look.
 * `elsewhere`: it has data, all of it shown on another card's rows.
 */
export type SectionState = "data" | "empty" | "notRun" | "elsewhere";

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
    case "workloads":
      if (scanWorkloads(info).workloads.length > 0) return "data";
      return info.containers_known === false ? "notRun" : "empty";
    case "agents": return data(has(info.agents) || (!info.service_inventory && has(info.systemd_services)));
    // Only the ports no workload or agent row already shows.
    case "ports": return data(has(scanWorkloads(info).otherPorts));
    case "git":
      if (has(info.git_remotes)) return "data";
      return info.git_remotes == null || info.git_remotes_partial ? "notRun" : "empty";
    case "roles": return deep(info, (d) => !!d.roles);
    case "webservers": return data(has(info.web_servers));
    // What a web-server site row already carries is not repeated in a card.
    case "traffic":
      return has(info.deep_facts?.sites) && siteFacts(info).traffic.length === 0 ? "elsewhere" : deep(info, (d) => has(d.sites));
    case "code":
      return has(info.deep_facts?.code) && siteFacts(info).code.length === 0 ? "elsewhere" : deep(info, (d) => has(d.code));
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
      if (state === "data") sections.push(s);
      else if (state === "empty") empty.push(s);
      else if (state === "notRun") notRun.push(s);
    }
    if (sections.length > 0) groups.push({ group, sections });
  }
  return { groups, empty, notRun };
}

// ── docker ps + docker stats ──

/** Parse a container_stats entry. */
export function parseContainerRow(raw: string) {
  const nameMatch = raw.match(/^(.+?)\s+CPU:/);
  const name = nameMatch?.[1]?.trim() || raw.split(/\s+/)[0] || raw;
  const cpuMatch = raw.match(/CPU:([\d.]+%?)/);
  const cpu = cpuMatch?.[1] || "";
  const memMatch = raw.match(/MEM:(.+?)(?:\s+NET:|$)/);
  const mem = memMatch?.[1]?.trim() || "";
  const netMatch = raw.match(/NET:(.+)$/);
  const net = netMatch?.[1]?.trim() || "";
  return { name, cpu, mem, net, cpuNum: parseFloat(cpu) || 0, memNum: parseFloat(mem) || 0 };
}

export type ContainerRow = ReturnType<typeof parseContainerRow> & { container?: ParsedContainer };

/**
 * One row per container: `docker ps` (parsed_containers) is the list,
 * `docker stats` (container_stats) only adds CPU / memory / network. The two
 * come from separate commands, so either may be missing: a container without
 * stats still gets a row, and so does a stats line with no ps match.
 */
export function containerRows(info: { parsed_containers?: ParsedContainer[] | null; container_stats?: string[] | null }): ContainerRow[] {
  const stats = new Map((info.container_stats ?? []).map((raw) => {
    const r = parseContainerRow(raw);
    return [r.name, r] as const;
  }));
  const rows: ContainerRow[] = (info.parsed_containers ?? []).map((c) => {
    const s = stats.get(c.name);
    stats.delete(c.name);
    return { ...(s ?? { name: c.name, cpu: "", mem: "", net: "", cpuNum: 0, memNum: 0 }), container: c };
  });
  return [...rows, ...stats.values()];
}

// ── one row per workload ──

/**
 * Something running on the host, as one row: a container, a classified
 * service (with the processes behind it) or a process nothing else claims.
 * Each carries the ports it listens on, so a port is read next to its owner.
 */
export type Workload = {
  key: string;
  kind: "container" | "service" | "process";
  name: string;
  running: boolean;
  /** OS housekeeping (networkd-dispatcher, unattended-upgrade…): dimmed, last. */
  system?: boolean;
  ports: number[];
  cpu?: string;
  mem?: string;
  container?: ContainerRow;
  /** The classified service: a service row's own, or what a container runs. */
  service?: DiscoveredService;
  processes: Partial<ProcessDetail>[];
};

const SYSTEM_PATHS = ["/usr/bin/", "/usr/sbin/", "/usr/lib/", "/usr/share/", "/lib/systemd/", "/sbin/"];
const SYSTEM_COMMANDS = [
  "networkd-dispatcher", "unattended-upgrade", "check-new-release",
  "packagekitd", "snapd", "thermald", "accounts-daemon", "polkitd",
  "systemd-", "udisksd", "fwupd", "colord", "modemmanager",
  "irqbalance", "multipathd", "atd", "cron",
];

function isSystemProcess(command: string, cwd?: string): boolean {
  if (SYSTEM_PATHS.some((p) => command.startsWith(p))) return true;
  const lower = command.toLowerCase();
  if (SYSTEM_COMMANDS.some((c) => lower.includes(c))) return true;
  return cwd === "/" || cwd === "/root";
}

/** "/opt/app/venv/bin/gunicorn app:wsgi" → "gunicorn". */
const binaryName = (command: string) => command.split(/\s+/)[0].split("/").pop() || command;
const unitName = (u?: string) => (u ?? "").replace(/\.service$/, "");
const numbers = (s?: string) => (s?.match(/\d+/g) ?? []).map(Number);
/** Host ports a container publishes: "0.0.0.0:8080->80/tcp" → 8080. */
const publishedPorts = (ports?: string) => [...(ports ?? "").matchAll(/:(\d+)->/g)].map((m) => Number(m[1]));
const sorted = (ports: Iterable<number>) => [...new Set(ports)].sort((a, b) => a - b);
const sameContainer = (a?: string, b?: string) => !!a && !!b && (a.startsWith(b) || b.startsWith(a));

export function scanWorkloads(info: VMInfoType): { workloads: Workload[]; otherPorts: string[] } {
  const inventory = info.service_inventory ?? [];
  const inContainer = new Set<DiscoveredService>();

  const containers: Workload[] = containerRows(info).map((r) => {
    const c = r.container;
    const service = inventory.find((s) => sameContainer(s.container_id, c?.id))
      ?? inventory.find((s) => !s.container_id && !!s.container_image && s.container_image === c?.image);
    if (service) inContainer.add(service);
    const owned = (info.port_owners ?? []).filter((po) => po.owner_type === "container" && po.owner_name === r.name).map((po) => po.port);
    return {
      key: `container:${r.name}`, kind: "container", name: r.name,
      running: c ? /^up\b/i.test(c.status.trim()) : true,
      ports: sorted([...owned, ...publishedPorts(c?.ports)]),
      cpu: r.cpu, mem: r.mem, container: r, service, processes: [],
    };
  });

  const claimed = new Set<Partial<ProcessDetail>>();
  const details = info.process_details ?? [];
  const services: Workload[] = [];
  inventory.forEach((s, i) => {
    // A service that only runs in a container is that container's row.
    if (inContainer.has(s) && !s.host_running) return;
    const processes = details.filter((p) => (!!s.unit && unitName(p.unit) === unitName(s.unit)) || (!!s.pid && p.pid === String(s.pid)));
    processes.forEach((p) => claimed.add(p));
    services.push({
      key: `service:${s.name}:${i}`, kind: "service", name: s.label || s.name,
      running: !!s.host_running || s.state === "active" || s.state === "running",
      ports: sorted(s.host_ports ?? (inContainer.has(s) ? [] : s.ports ?? [])),
      cpu: processes[0]?.cpu, mem: processes[0]?.mem, service: s, processes,
    });
  });

  // Scans from before process_details carry names (and maybe CPU/MEM) only.
  const legacy: Partial<ProcessDetail>[] = details.length > 0 ? [] : (info.service_details?.length
    ? info.service_details.map((raw) => {
      const m = raw.match(/^(.+?)\s+CPU:([\d.]+%)\s+MEM:([\d.]+%)/);
      return { command: m?.[1] ?? raw, cpu: m?.[2], mem: m?.[3] };
    })
    : (info.services ?? []).map((command) => ({ command })));
  const processes: Workload[] = [...details.filter((p) => !claimed.has(p)), ...legacy].map((p, i) => ({
    key: `process:${p.pid ?? i}`, kind: "process", name: binaryName(p.command ?? ""),
    running: true, system: details.length > 0 && isSystemProcess(p.command ?? "", p.cwd),
    ports: sorted(numbers(p.ports)), cpu: p.cpu, mem: p.mem, processes: [p],
  }));
  processes.sort((a, b) => Number(!!a.system) - Number(!!b.system));

  const workloads = [...containers, ...services, ...processes];
  const shown = new Set<number>([...workloads.flatMap((w) => w.ports), ...(info.agents ?? []).flatMap((a) => a.ports ?? [])]);
  const otherPorts = (info.ports ?? []).filter((p) => !shown.has(parseInt(p, 10)));
  return { workloads, otherPorts };
}

// ── one row per site ──

const siteName = (n: string) => n.toLowerCase().replace(/^www\./, "");
/** Same directory, or one inside the other (a docroot above the served `public/`). */
const samePath = (a: string, b: string) => a === b || a.startsWith(b.replace(/\/$/, "") + "/") || b.startsWith(a.replace(/\/$/, "") + "/");

/**
 * The deep facts that belong to a web-server site — its access-log traffic
 * (by name, `www.` aside) and its docroot's code fingerprint (by path) — so
 * the site's row can carry them, plus the traffic and code no site claims,
 * which keep their own cards.
 */
export function siteFacts(info: VMInfoType): {
  bySite: Map<WebSite, { traffic?: SiteTraffic; code?: CodeFingerprint }>;
  traffic: SiteTraffic[];
  code: CodeFingerprint[];
} {
  const traffic = [...(info.deep_facts?.sites ?? [])];
  const code = [...(info.deep_facts?.code ?? [])];
  const bySite = new Map<WebSite, { traffic?: SiteTraffic; code?: CodeFingerprint }>();
  const take = <X,>(list: X[], match: (x: X) => boolean): X | undefined => {
    const i = list.findIndex(match);
    return i < 0 ? undefined : list.splice(i, 1)[0];
  };
  for (const server of info.web_servers ?? []) {
    for (const site of server.sites ?? []) {
      const names = new Set((site.names ?? []).map(siteName));
      const t = take(traffic, (x) => names.has(siteName(x.site)));
      const c = site.root ? take(code, (x) => samePath(x.docroot, site.root!)) : undefined;
      if (t || c) bySite.set(site, { traffic: t, code: c });
    }
  }
  return { bySite, traffic, code };
}
