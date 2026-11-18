import { test } from "node:test";
import assert from "node:assert/strict";
import { containerRows, scanLayout, scanWorkloads, sectionState, siteFacts } from "./scanSections.ts";
import type { VMInfoType } from "./api.ts";

const scan = (v: Partial<VMInfoType>) => v as VMInfoType;

// SAPASP-AVALIE04 after a background scan: one container, no web server.
const avalie = scan({
  os: "Ubuntu 22.04.1 LTS",
  cpu_usage: "12%",
  ports: ["22", "3000"],
  port_owners: [{ port: 22 }, { port: 3000, owner_type: "container", owner_name: "form-totem-nextjs" }],
  parsed_containers: [{ id: "4b82", name: "form-totem-nextjs", image: "img", status: "Up 13 months", ports: "0.0.0.0:3000->3000/tcp" }],
  containers_known: true,
  process_details: [{ pid: "915", user: "root", cpu: "0.0%", mem: "0.1%", command: "/usr/bin/python3" }],
  git_remotes: [{ path: "/home/sead/app", name: "origin", url: "https://gitlab/x.git" }],
  ssh_auth_policy: { password_auth: "yes" },
  deep_facts: { patch: { manager: "apt" }, firewall: { iptables_rules: 23 } },
} as Partial<VMInfoType>);

test("groups hold only the sections that have data, in order", () => {
  const { groups } = scanLayout(avalie);
  assert.deepEqual(groups, [
    { group: "resources", sections: ["resources"] },
    { group: "workloads", sections: ["workloads", "ports", "git"] },
    { group: "access", sections: ["security", "firewall"] },
    { group: "system", sections: ["system", "patch"] },
  ]);
});

test("a section the scan looked at and found nothing in is listed as empty", () => {
  const { empty, notRun } = scanLayout(avalie);
  assert.ok(empty.includes("webservers") && empty.includes("traffic") && empty.includes("users"));
  assert.deepEqual(notRun, []);
});

test("deep-fact sections of a quick test are not checked, not empty", () => {
  const { notRun, empty } = scanLayout(scan({ os: "Debian 12", deep_facts: null, git_remotes: null }));
  for (const s of ["traffic", "code", "panels", "roles", "owner", "outbound", "patch", "git"] as const) {
    assert.ok(notRun.includes(s), s);
    assert.ok(!empty.includes(s), s);
  }
});

test("workloads are not checked when docker ps did not answer and nothing else runs", () => {
  assert.equal(sectionState(scan({ containers_known: false }), "workloads"), "notRun");
  assert.equal(sectionState(scan({ containers_known: true }), "workloads"), "empty");
  // Scans from before the flag: an empty list is all there is to go on.
  assert.equal(sectionState(scan({}), "workloads"), "empty");
});

test("the ports card holds only the ports no workload or agent claims", () => {
  assert.deepEqual(scanWorkloads(avalie).otherPorts, ["22"]);
  const claimed = scan({ ports: ["3000"], port_owners: avalie.port_owners, parsed_containers: avalie.parsed_containers });
  assert.equal(sectionState(claimed, "ports"), "empty");
  const agent = scan({ ports: ["10050"], agents: [{ name: "zabbix-agent2", label: "Zabbix Agent 2", category: "monitoring", ports: [10050] }] });
  assert.equal(sectionState(agent, "ports"), "empty");
  assert.equal(sectionState(agent, "agents"), "data");
});

test("the firewall section also shows the quick test's raw status", () => {
  assert.equal(sectionState(scan({ firewall_status: "Status: inactive", deep_facts: null }), "firewall"), "data");
  assert.equal(sectionState(scan({ deep_facts: null }), "firewall"), "notRun");
  assert.equal(sectionState(scan({ deep_facts: {} }), "firewall"), "empty");
});

test("a git list cut short with nothing found is not checked", () => {
  assert.equal(sectionState(scan({ git_remotes: [], git_remotes_partial: true }), "git"), "notRun");
  assert.equal(sectionState(scan({ git_remotes: [] }), "git"), "empty");
});

// ── one row per workload ──

test("a container is one workload carrying the port it publishes", () => {
  const { workloads } = scanWorkloads(avalie);
  assert.deepEqual(workloads.map((w) => [w.kind, w.name, w.ports, !!w.system]), [
    ["container", "form-totem-nextjs", [3000], false],
    ["process", "python3", [], true],
  ]);
});

test("a host service takes its processes and ports; they get no row of their own", () => {
  const { workloads } = scanWorkloads(scan({
    service_inventory: [{ name: "postgresql", label: "PostgreSQL", kind: "database", state: "active", unit: "postgresql@14-main.service", host_running: true, ports: [5432, 5433], host_ports: [5432] }],
    process_details: [
      { pid: "700", user: "postgres", cpu: "1.0%", mem: "2.0%", command: "/usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main", cwd: "/var/lib/postgresql/14/main", started_via: "systemd", unit: "postgresql@14-main.service" },
      { pid: "900", user: "app", cpu: "3.0%", mem: "4.0%", command: "/opt/app/venv/bin/gunicorn app:wsgi", cwd: "/opt/app", started_via: "manual", ports: "8000" },
    ],
  }));
  assert.deepEqual(workloads.map((w) => [w.kind, w.name, w.ports, w.processes.map((p) => p.pid)]), [
    ["service", "PostgreSQL", [5432], ["700"]],
    ["process", "gunicorn", [8000], ["900"]],
  ]);
});

test("a containerized service is the container's row, not a second one", () => {
  const { workloads } = scanWorkloads(scan({
    parsed_containers: [{ id: "abc123def456", name: "db", image: "postgres:16", status: "Up 2 days", ports: "0.0.0.0:5432->5432/tcp" }],
    service_inventory: [{ name: "postgresql", label: "PostgreSQL", kind: "database", container_id: "abc123def456aaaa", container_image: "postgres:16", ports: [5432] }],
  }));
  assert.equal(workloads.length, 1);
  assert.equal(workloads[0].kind, "container");
  assert.equal(workloads[0].service?.label, "PostgreSQL");
  assert.deepEqual(workloads[0].ports, [5432]);
});

test("a service running both on the host and in a container gets both rows", () => {
  const { workloads } = scanWorkloads(scan({
    parsed_containers: [{ id: "abc123def456", name: "cache", image: "redis:7", status: "Up 1 hour", ports: "" }],
    service_inventory: [{ name: "redis", label: "Redis", kind: "cache", state: "active", container_id: "abc123def456", container_image: "redis:7", host_running: true, host_ports: [6379] }],
  }));
  assert.deepEqual(workloads.map((w) => [w.kind, w.name]), [["container", "cache"], ["service", "Redis"]]);
});

test("a stopped container is not running", () => {
  const { workloads } = scanWorkloads(scan({ parsed_containers: [{ id: "a", name: "old", image: "x", status: "Exited (0) 3 weeks ago", ports: "" }] }));
  assert.equal(workloads[0].running, false);
});

test("the oldest scans, with process names only, still list them", () => {
  const { workloads } = scanWorkloads(scan({ services: ["/usr/sbin/nginx"], service_details: ["/usr/sbin/nginx  CPU:0.5%  MEM:1.0%  RSS:12MB"] }));
  assert.deepEqual(workloads.map((w) => [w.kind, w.name, w.cpu, w.mem]), [["process", "nginx", "0.5%", "1.0%"]]);
});

// ── docker ps + docker stats ──

const ps = { id: "4b82b86b1b62", name: "form-totem-nextjs", image: "img", status: "Up 13 months", ports: "0.0.0.0:3000->3000/tcp" };

// SAPASP-AVALIE04: docker ps answered, docker stats did not.
test("a container without stats still gets a row", () => {
  const rows = containerRows({ parsed_containers: [ps], container_stats: null });
  assert.equal(rows.length, 1);
  assert.equal(rows[0].name, "form-totem-nextjs");
  assert.equal(rows[0].container, ps);
  assert.equal(rows[0].cpu, "");
});

test("stats join the docker ps row by name", () => {
  const rows = containerRows({ parsed_containers: [ps], container_stats: ["form-totem-nextjs  CPU:1.5%  MEM:30MiB / 1.9GiB  NET:1kB / 2kB"] });
  assert.equal(rows.length, 1);
  assert.equal(rows[0].cpuNum, 1.5);
  assert.equal(rows[0].container, ps);
});

test("a stats row with no docker ps match still renders", () => {
  const rows = containerRows({ container_stats: ["old  CPU:0.1%  MEM:1MiB / 1GiB  NET:0B / 0B"] });
  assert.equal(rows.length, 1);
  assert.equal(rows[0].name, "old");
  assert.equal(rows[0].container, undefined);
});

// ── one row per site ──

const web = scan({
  web_servers: [{ kind: "nginx", sites: [
    { names: ["portal.pi.gov.br", "www.portal.pi.gov.br"], root: "/var/www/portal/public" },
    { names: ["api.pi.gov.br"], proxy: ["127.0.0.1:8000"] },
  ] }],
  deep_facts: {
    sites: [{ site: "www.portal.pi.gov.br", requests: 900, per_day: 30 }, { site: "legado.pi.gov.br", requests: 0 }],
    code: [{ docroot: "/var/www/portal", apps: [{ framework: "laravel", version: "10" }] }, { docroot: "/srv/outro" }],
  },
} as Partial<VMInfoType>);

test("a site's traffic and code ride on its row", () => {
  const facts = siteFacts(web);
  const [portal, api] = web.web_servers![0].sites!;
  assert.equal(facts.bySite.get(portal)?.traffic?.per_day, 30);
  // The docroot above the served `public/` directory is still that site's code.
  assert.equal(facts.bySite.get(portal)?.code?.docroot, "/var/www/portal");
  assert.equal(facts.bySite.get(api), undefined);
});

test("traffic and code no site claims keep their own cards", () => {
  const facts = siteFacts(web);
  assert.deepEqual(facts.traffic.map((s) => s.site), ["legado.pi.gov.br"]);
  assert.deepEqual(facts.code.map((d) => d.docroot), ["/srv/outro"]);
  assert.equal(sectionState(web, "traffic"), "data");
});

test("facts shown on site rows are neither a card nor 'nothing found'", () => {
  const all = scan({
    web_servers: [{ kind: "nginx", sites: [{ names: ["a.gov.br"], root: "/var/www/a" }] }],
    deep_facts: { sites: [{ site: "a.gov.br", requests: 5 }], code: [{ docroot: "/var/www/a" }] },
  } as Partial<VMInfoType>);
  assert.equal(sectionState(all, "traffic"), "elsewhere");
  assert.equal(sectionState(all, "code"), "elsewhere");
  const { groups, empty } = scanLayout(all);
  assert.deepEqual(groups.find((g) => g.group === "web")?.sections, ["webservers"]);
  assert.ok(!empty.includes("traffic") && !empty.includes("code"));
});

test("a docroot is not matched by a sibling directory with the same prefix", () => {
  const facts = siteFacts(scan({
    web_servers: [{ kind: "apache", sites: [{ names: ["a.gov.br"], root: "/var/www/app" }] }],
    deep_facts: { code: [{ docroot: "/var/www/app2" }] },
  } as Partial<VMInfoType>));
  assert.equal(facts.bySite.size, 0);
  assert.equal(facts.code.length, 1);
});
