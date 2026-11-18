import { test } from "node:test";
import assert from "node:assert/strict";
import { scanLayout, sectionState } from "./scanSections.ts";
import type { VMInfoType } from "./api.ts";

const scan = (v: Partial<VMInfoType>) => v as VMInfoType;

// SAPASP-AVALIE04 after a background scan: one container, no web server.
const avalie = scan({
  os: "Ubuntu 22.04.1 LTS",
  cpu_usage: "12%",
  ports: ["22", "3000"],
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
    { group: "workloads", sections: ["ports", "containers", "processes", "git"] },
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

test("containers are not checked when docker ps did not answer", () => {
  assert.equal(sectionState(scan({ containers_known: false }), "containers"), "notRun");
  assert.equal(sectionState(scan({ containers_known: true }), "containers"), "empty");
  // Scans from before the flag: an empty list is all there is to go on.
  assert.equal(sectionState(scan({}), "containers"), "empty");
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
