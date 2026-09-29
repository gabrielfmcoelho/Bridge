import { test } from "node:test";
import assert from "node:assert/strict";
import { serviceInsights, serviceBreakdowns, isStale } from "./serviceInsights.ts";
import type { Service } from "@/lib/types";

const t = (k: string, v?: Record<string, string>) => (v ? `${k}(${Object.values(v).join(",")})` : k);
const svc = (p: Partial<Service>) => ({ id: 1, nickname: "s", source: "auto", discovery_kind: "container", container_status: "online", tags: [], ...p }) as Service;
const old = new Date(Date.now() - 10 * 86_400_000).toISOString();

const services = [
  svc({ id: 1, service_kind: "database", service_subtype: "PostgreSQL", host_ids: [7], main_responsavel_name: "Ana" }),
  svc({ id: 2, service_kind: "database", service_subtype: "MySQL", container_status: "offline", host_ids: [7, 8] }),
  svc({ id: 3, service_kind: "web", discovery_kind: "host", last_seen_at: old, tags: ["core"] }),
  svc({ id: 4, source: "manual", discovery_kind: "", container_status: "", service_kind: "" }),
];

test("stale needs a scan-owned row not seen for STALE_DAYS", () => {
  assert.equal(isStale(services[2]), true);
  assert.equal(isStale(services[0]), false);
  assert.equal(isStale({ ...services[3], last_seen_at: old }), false);
});

test("insights count and carry filters", () => {
  const by = Object.fromEntries(serviceInsights(services, t).map((i) => [i.key, i]));
  assert.equal(by.total.value, 4);
  assert.equal(by.total.hint, "service.kpi.onlineOffline(2,1)");
  assert.deepEqual(by.offline.filter, { status: "offline" });
  assert.equal(by.offline.value, 1);
  assert.equal(by.autoReview.value, 3);
  assert.equal(by.noResponsavel.value, 3);
  assert.equal(by["kind:database"].value, 2);
  assert.deepEqual(by["kind:database"].filter, { kind: "database" });
  assert.equal(by["kind:"], undefined, "unclassified is not a kind insight");
  assert.equal(by["tag:core"].value, 1);
});

test("breakdowns: kind, origin, status, engines, hosts", () => {
  const b = serviceBreakdowns(services, (id) => (id === 7 ? "db-host" : undefined));
  assert.deepEqual(b.kind.map((r) => [r.key, r.count]), [["database", 2], ["web", 1], ["service.kind.none", 1]]);
  assert.deepEqual(b.kind.at(-1)?.filter, { kind: "none" });
  const origin = Object.fromEntries(b.origin.map((r) => [r.key, r]));
  assert.equal(origin.container.count, 2);
  assert.deepEqual(origin.host.filter, { origin: "host" });
  assert.deepEqual(origin.manual.filter, { origin: "manual" });
  const status = Object.fromEntries(b.status.map((r) => [r.key, r]));
  assert.equal(status.stale.count, 1);
  assert.equal(status.stale.filter, undefined);
  assert.deepEqual(status.offline.filter, { status: "offline" });
  assert.deepEqual(b.engines.map((r) => r.key).sort(), ["MySQL", "PostgreSQL"]);
  assert.deepEqual(b.hosts.map((r) => [r.label, r.count]), [["db-host", 2], ["#8", 1]]);
});
