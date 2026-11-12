import { test } from "node:test";
import assert from "node:assert/strict";
import { applySuggestion, backupPreview, covered, DOCKER_NON_VOLUMES, formatBytes, under } from "./backupPreview.ts";

const GiB = 1 << 30;
const info = {
  filesystems: [
    { mount: "/", device: "/dev/sda1", fstype: "ext4", size_bytes: 50 * GiB, used_bytes: 20 * GiB, avail_bytes: 30 * GiB, used_pct: 40 },
    { mount: "/data", device: "/dev/sdb1", fstype: "xfs", size_bytes: 200 * GiB, used_bytes: 120 * GiB, avail_bytes: 80 * GiB, used_pct: 60 },
    { mount: "/tmp", device: "/dev/sda2", fstype: "ext4", size_bytes: GiB, used_bytes: GiB / 2, avail_bytes: GiB / 2, used_pct: 50 },
    { mount: "/mnt/nfs", device: "srv:/x", fstype: "nfs4", size_bytes: 0, used_bytes: 0, avail_bytes: 0, used_pct: 0, network: true },
  ],
  service_inventory: [
    { name: "postgresql", label: "PostgreSQL", kind: "database", host_running: true },
    { name: "redis", label: "Redis", kind: "cache", host_running: true },
    { name: "mysql", label: "MySQL", kind: "database", container_id: "abc" },
  ],
  parsed_containers: [{ id: "1", name: "web", image: "nginx", status: "Up", ports: "" }],
  resource_top: { top_disk: [{ path: "/data/dump.sql", size_bytes: 3 * GiB }, { path: "/tmp/x.iso", size_bytes: 2 * GiB }, { path: "/var/log/a.log", size_bytes: 100 }] },
};

test("under and covered follow tar: an exclude wins over an include beneath it", () => {
  assert.ok(under("/etc/nginx", "/"));
  assert.ok(under("/var/lib/docker", "/var/lib/docker/"));
  assert.ok(!under("/var/lib/dockerx", "/var/lib/docker"));
  assert.ok(covered("/etc", ["/"], ["/tmp"]));
  assert.ok(!covered("/var/lib/docker/volumes", ["/", "/var/lib/docker/volumes"], ["/var/lib/docker"]));
});

test("filesystems: included, excluded, network, partial, outside", () => {
  const p = backupPreview(info, ["/"], ["/tmp", "/var/lib/docker"]);
  assert.deepEqual(p.filesystems.map((f) => f.state), ["included", "included", "excluded", "network"]);
  assert.equal(p.upperBound, 140 * GiB);

  const etc = backupPreview(info, ["/etc"], []);
  assert.deepEqual(etc.filesystems.map((f) => f.state), ["partial", "outside", "outside", "network"]);
});

test("suggestions: a host database, docker volumes, big files", () => {
  const exclude = ["/tmp", "/var/lib/docker"];
  const p = backupPreview(info, ["/"], exclude);
  assert.deepEqual(p.suggestions.map((s) => `${s.kind}:${s.subject}`), ["database:PostgreSQL", "docker:1", "bigfile:/data/dump.sql"]);

  const docker = p.suggestions.find((s) => s.kind === "docker")!;
  const after = applySuggestion(exclude, docker);
  assert.deepEqual(after, ["/tmp", ...DOCKER_NON_VOLUMES]);
  assert.ok(covered("/var/lib/docker/volumes/v1", ["/"], after));

  // Taken suggestions go away.
  const db = applySuggestion(after, p.suggestions[0]);
  assert.ok(!backupPreview(info, ["/"], db).suggestions.some((s) => s.kind === "database" || s.kind === "docker"));
});

test("no scan, no preview", () => {
  assert.deepEqual(backupPreview(undefined, ["/"], []), { filesystems: [], upperBound: 0, suggestions: [] });
});

test("formatBytes", () => {
  assert.equal(formatBytes(3.4e9), "3.2 GiB");
  assert.equal(formatBytes(1000), "1 MiB");
  assert.equal(formatBytes(0), "0");
  assert.equal(formatBytes(2 * 1024 ** 4), "2.0 TiB");
});
