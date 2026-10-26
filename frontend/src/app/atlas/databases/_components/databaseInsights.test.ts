import { test } from "node:test";
import assert from "node:assert/strict";
import { databaseInsights, databaseBreakdowns, matchesDatabaseFilters, emptyDatabaseFilters, endpointLabel, lacksBackup } from "./databaseInsights.ts";
import type { Database } from "@/lib/types";

const t = (k: string, v?: Record<string, string>) => (v ? `${k}(${Object.values(v).join(",")})` : k);
const db = (p: Partial<Database>) =>
  ({ id: 1, name: "a", engine: "postgres", instance_service_id: 9, sensitivity: "interno", environment: "prod", backup_policy: "",
    consumer_service_ids: [], project_ids: [], host: "", port: null, ...p }) as Database;

const dbs = [
  db({ id: 1, sensitivity: "sensivel", consumer_service_ids: [3], project_ids: [7], backup_policy: "pg_dump diário" }),
  db({ id: 2, engine: "oracle", instance_service_id: null, sensitivity: "pessoal", environment: "homolog" }),
  db({ id: 3, engine: "mysql" }),
];

test("insights count and carry filters", () => {
  const by = Object.fromEntries(databaseInsights(dbs, t).map((i) => [i.key, i]));
  assert.equal(by.total.value, 3);
  assert.equal(by.total.hint, "atlas.databases.kpi.totalHint(1)");
  assert.equal(by.personalData.value, 2);
  assert.equal(by.prodNoBackup.value, 1, "only prod rows count; homolog is not required to have a policy");
  assert.equal(by.noConsumers.value, 2);
  assert.equal(by.external.value, 1);
  assert.deepEqual(by["engine:postgres"].filter, { engine: "postgres" });
});

test("filters", () => {
  const ids = (f: Partial<typeof emptyDatabaseFilters>) => dbs.filter((d) => matchesDatabaseFilters(d, { ...emptyDatabaseFilters, ...f })).map((d) => d.id);
  assert.deepEqual(ids({}), [1, 2, 3]);
  assert.deepEqual(ids({ where: "external" }), [2]);
  assert.deepEqual(ids({ where: "instance" }), [1, 3]);
  assert.deepEqual(ids({ lgpd: "personal" }), [1, 2]);
  assert.deepEqual(ids({ backup: "missing" }), [3]);
  assert.deepEqual(ids({ consumers: "none" }), [2, 3]);
  assert.deepEqual(ids({ service: "3" }), [1]);
  assert.deepEqual(ids({ project: "7", engine: "postgres" }), [1]);
});

test("breakdowns label rows and keep the no-project bucket", () => {
  const b = databaseBreakdowns(dbs, (id) => (id === 7 ? "Portal" : undefined));
  assert.deepEqual(b.projects.map((r) => [r.label, r.count]), [["Portal", 1], ["atlas.databases.dash.noProject", 2]]);
  assert.deepEqual(b.engine.map((r) => r.label), ["atlas.databases.engine.mysql", "atlas.databases.engine.oracle", "atlas.databases.engine.postgres"], "ties sort by name");
  assert.deepEqual(b.engine[2].filter, { engine: "postgres" });
});

test("endpoint prefers the override, then the instance", () => {
  assert.equal(endpointLabel(db({ host: "10.0.0.5", port: 5433 })), "10.0.0.5:5433");
  assert.equal(endpointLabel(db({ instance: { host_nickname: "vm-db01", port: "5432" } as Database["instance"] })), "vm-db01:5432");
  assert.equal(endpointLabel(db({})), "");
  assert.equal(lacksBackup(db({ backup_policy: "  " })), true);
});
