import { test } from "node:test";
import assert from "node:assert/strict";
import { coolifyName, serviceTitle } from "./serviceDisplay.ts";

test("coolifyName: application, stack member, database", () => {
  assert.equal(coolifyName("Gestor - API", "h04g8gc8k8g8cocccokkwwso-115942864960"), "Gestor - API");
  assert.equal(coolifyName("Infra - Airflow", "airflow-scheduler-rtb37u69bbkepzukr9vclrd7-123344967269"), "Infra - Airflow · airflow-scheduler");
  assert.equal(coolifyName("gateway", "keycloak-db-kefmb9um8tyct5itlwaml1tp"), "gateway · keycloak-db");
  assert.equal(coolifyName("Gestor - DB - Postgres", "jcog8ow0c0wcgw08k8k8s0cs"), "Gestor - DB - Postgres");
});

test("serviceTitle: Coolify name over the hash, edited nickname wins", () => {
  const base = { service_subtype: "PostgreSQL", discovery_kind: "container" as const, source: "auto" as const,
    container_name: "postgres-m88c84c4ww48kgwocsos0gsk", discovery_key: "postgres-m88c84c4ww48kgwocsos0gsk",
    coolify_stack: "Tool - Wiki - Outline" };
  assert.deepEqual(serviceTitle({ ...base, nickname: base.container_name }),
    { title: "Tool - Wiki - Outline · postgres", mono: false, id: base.container_name });
  assert.equal(serviceTitle({ ...base, nickname: "Outline DB" }).title, "Outline DB");
  assert.equal(serviceTitle({ ...base, coolify_stack: "", nickname: base.container_name }).title, "PostgreSQL");
});
