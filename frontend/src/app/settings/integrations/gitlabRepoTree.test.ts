import { test } from "node:test";
import assert from "node:assert/strict";
import { buildTree, inheritedByGroup, repoMatches, repoTotal } from "./gitlabRepoTree.ts";
import type { GitLabInventoryGroup, GitLabInventoryRepo } from "../../../lib/types";

const group = (id: number, full_path: string, parent_id: number | null): GitLabInventoryGroup =>
  ({ id, name: full_path.split("/").pop()!, full_path, parent_id, web_url: "", projetos: [], hosts: [] });
const repo = (id: number, path: string, namespace_id: number, dns: string[] = []): GitLabInventoryRepo => ({
  id, name: path.split("/").pop()!, path_with_namespace: path, namespace_id, web_url: "", default_branch: "main",
  projetos: [], services: [], dns: dns.map((name, i) => ({ id: i + 1, link_id: i + 1, name })), hosts: [],
});

const groups = [group(2, "sead/dl", 1), group(1, "sead", null), group(3, "sead/empty", 1)];
const repos = [repo(7, "sead/app", 1, ["app.sead.pi.gov.br"]), repo(8, "sead/dl/lake", 2), repo(9, "joao/scratch", 99)];

test("nests groups, keeps empty ones unpruned, sorts by path", () => {
  const { roots, loose } = buildTree(groups, repos, () => true, false);
  assert.equal(roots.length, 1);
  assert.deepEqual(roots[0].children.map((c) => c.group.full_path), ["sead/dl", "sead/empty"]);
  assert.equal(repoTotal(roots[0]), 2);
  assert.deepEqual(loose.map((r) => r.id), [9]);
});

test("prune drops groups left without repos", () => {
  const { roots } = buildTree(groups, repos, (r) => repoMatches(r, "", "unlinked"), true);
  assert.deepEqual(roots[0].children.map((c) => c.group.full_path), ["sead/dl"]);
  assert.equal(roots[0].repos.length, 0);
});

test("filters and search", () => {
  const [app, lake] = repos;
  assert.ok(repoMatches(app, "", "linked") && !repoMatches(lake, "", "linked"));
  assert.ok(repoMatches(app, "", "dns") && !repoMatches(app, "", "hosts"));
  assert.ok(repoMatches(app, "APP.SEAD", "all"), "matches an attachment name");
  assert.ok(repoMatches(lake, "dl/la", "all") && !repoMatches(lake, "sgi", "all"));
});

test("a group's projeto / host links reach every repo and subgroup below it", () => {
  const gs = [group(1, "sead", null), group(2, "sead/dl", 1), group(4, "sead/dl/raw", 2)];
  gs[1].projetos = [{ id: 5, link_id: 50, name: "Lake" }];
  gs[1].hosts = [{ id: 9, link_id: 90, name: "vm-dl" }];
  gs[0].projetos = [{ id: 6, link_id: 60, name: "Plataforma" }];
  const inh = inheritedByGroup(gs);
  assert.deepEqual(inh.get(4)!.map((i) => `${i.kind}:${i.ref.name}@${i.via}`),
    ["projetos:Plataforma@sead", "projetos:Lake@sead/dl", "hosts:vm-dl@sead/dl"]);
  assert.equal(inh.get(1)!.length, 1);

  const raw = repo(10, "sead/dl/raw/ingest", 4);
  assert.ok(!repoMatches(raw, "", "linked"), "no inheritance passed: unlinked");
  assert.ok(repoMatches(raw, "", "linked", inh.get(4)) && !repoMatches(raw, "", "unlinked", inh.get(4)));
  assert.ok(repoMatches(raw, "", "hosts", inh.get(4)) && !repoMatches(raw, "", "dns", inh.get(4)));
  assert.ok(repoMatches(raw, "vm-dl", "all", inh.get(4)), "search finds the inherited host");
});
