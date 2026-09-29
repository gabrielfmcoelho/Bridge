import { test } from "node:test";
import assert from "node:assert/strict";
import { projectInsights, projectBreakdowns } from "./projectInsights.ts";
import type { Project } from "@/lib/types";

const t = (k: string, v?: Record<string, string>) => (v ? `${k}(${Object.values(v).join(",")})` : k);
const proj = (p: Partial<Project>) => ({ id: 1, name: "p", situacao: "Ativa", tags: [], ...p }) as Project;

const projects = [
  proj({ id: 1, name: "Atlas", services_count: 3, issues_count: 2, main_responsavel_name: "Ana", setor_responsavel: "NTGD", repos_count: 1, tags: ["core"] }),
  proj({ id: 2, name: "Bridge", services_count: 1, issues_count: 0, tem_empresa_externa_responsavel: true }),
  proj({ id: 3, name: "Caduf", situacao: "Em investigação" }),
];

test("insights count and carry filters", () => {
  const by = Object.fromEntries(projectInsights(projects, t).map((i) => [i.key, i]));
  assert.equal(by.total.value, 3);
  assert.equal(by.total.hint, "project.kpi.servicesTotal(4)");
  assert.equal(by.openIssues.value, 2);
  assert.equal(by.openIssues.hint, "project.kpi.inProjects(1)");
  assert.equal(by.noOwner.value, 2);
  assert.equal(by.noServices.value, 1);
  assert.equal(by.external.value, 1);
  assert.equal(by.noRepo.value, 2);
  assert.deepEqual(by["sit:Em investigação"].filter, { situacao: "Em investigação" });
  assert.equal(by["tag:core"].value, 1);
});

test("breakdowns: situação, setor, top by services and issues", () => {
  const b = projectBreakdowns(projects);
  assert.deepEqual(b.situacao.map((r) => [r.key, r.count]), [["Ativa", 2], ["Em investigação", 1]]);
  assert.deepEqual(b.setor.map((r) => [r.key, r.count]), [["inventory.dash.none", 2], ["NTGD", 1]]);
  assert.deepEqual(b.byServices.map((r) => [r.label, r.count]), [["Atlas", 3], ["Bridge", 1]]);
  assert.deepEqual(b.byIssues.map((r) => r.label), ["Atlas"]);
});
