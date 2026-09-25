import { test } from "node:test";
import assert from "node:assert/strict";
import { hostInsights, hostBreakdowns, peakUsage } from "./hostInsights.ts";
import type { Host } from "@/lib/types";

const t = (k: string, v?: Record<string, string>) => (v ? `${k}(${Object.values(v).join(",")})` : k);
const host = (p: Partial<Host>) => ({ id: 1, situacao: "active", tags: [], ...p }) as Host;

const hosts = [
  host({ id: 1, has_scan: true, idle: true, tags: ["web"], scan_resources: { cpu_usage: "91", ram_percent: "40%" }, alerts: [{ level: "critical" }] as Host["alerts"] }),
  host({ id: 2, has_scan: true, tags: ["web", "db"], scan_resources: { cpu_usage: "10", disk_percent: "12%" }, has_key: true }),
  host({ id: 3, situacao: "maintenance", alerts: [{ level: "warning" }] as Host["alerts"], has_password: true }),
];

test("peak usage takes the highest reading, null without a scan", () => {
  assert.equal(peakUsage(hosts[0]), 91);
  assert.equal(peakUsage(hosts[1]), 12);
  assert.equal(peakUsage(hosts[2]), null);
});

test("counts, hints and filters", () => {
  const by = Object.fromEntries(hostInsights(hosts, t).map((i) => [i.key, i]));
  assert.equal(by.total.value, 3);
  assert.equal(by.total.hint, "2 common.active · 1 common.maintenance");
  assert.equal(by.critical.value, 1);
  assert.equal(by.critical.hint, "host.kpi.withAnyAlert(2)");
  assert.deepEqual(by.critical.filter, { alert_level: "critical" });
  assert.equal(by.idle.value, 1);
  assert.equal(by.highUsage.value, 1);
  assert.equal(by.noScan.value, 1);
  assert.equal(by.noCreds.value, 1);
  assert.equal(by.maintenance.hint, "33%");
});

test("tags become insights, most used first", () => {
  const tags = hostInsights(hosts, t).filter((i) => i.key.startsWith("tag:"));
  assert.deepEqual(tags.map((i) => [i.label, i.value]), [["web", 2], ["db", 1]]);
  assert.deepEqual(tags[0].filter, { tag: "web" });
});

test("breakdowns count per value, most frequent first, with filters", () => {
  const b = hostBreakdowns([
    host({ id: 1, situacao: "active", hospedagem: "ETIPI", tags: ["web"], has_scan: true, scan_resources: { cpu_usage: "91" }, alerts: [{ level: "critical" }] as Host["alerts"] }),
    host({ id: 2, situacao: "active", hospedagem: "ETIPI", tags: ["web", "db"], has_scan: true, scan_resources: { cpu_usage: "60" } }),
    host({ id: 3, situacao: "maintenance", hospedagem: "", tags: [] }),
  ]);
  assert.deepEqual(b.situacao.map((r) => [r.key, r.count]), [["active", 2], ["maintenance", 1]]);
  assert.deepEqual(b.situacao[0].filter, { situacao: "active" });
  assert.deepEqual(b.hospedagem.map((r) => [r.key, r.count, !!r.labelKey]), [["ETIPI", 2, false], ["host.dash.none", 1, true]]);
  assert.deepEqual(b.tags.map((r) => [r.key, r.count]), [["web", 2], ["db", 1]]);
  assert.deepEqual(b.usage.map((r) => [r.key, r.count]), [["high", 1], ["mid", 1], ["low", 0], ["none", 1]]);
  assert.deepEqual(b.alerts.map((r) => [r.key, r.count]), [["critical", 1], ["warning", 0], ["info", 0]]);
});
