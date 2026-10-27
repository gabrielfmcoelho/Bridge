// Pure helpers for the GitLab repos tab: the group tree and its filters.
// Runtime-import free so `node --test` can run gitlabRepoTree.test.ts.
import type { GitLabInventoryGroup, GitLabInventoryRepo, GitLabRepoRef } from "../../../lib/types";

export type RepoFilter = "all" | "linked" | "unlinked" | "projetos" | "services" | "dns" | "hosts";

export interface GroupNode {
  group: GitLabInventoryGroup;
  children: GroupNode[];
  repos: GitLabInventoryRepo[];
}

/** A projeto or host link a repo or subgroup gets from a group above it
 *  (or, for a repo, its own group); via is that group's full path. */
export interface InheritedRef {
  kind: "projetos" | "hosts";
  ref: GitLabRepoRef;
  via: string;
}

/** For every group, the projeto / host links of the group itself and of all
 *  its ancestors, outermost first: what each repo inside it belongs to. */
export function inheritedByGroup(groups: GitLabInventoryGroup[]): Map<number, InheritedRef[]> {
  const byId = new Map(groups.map((g) => [g.id, g]));
  const memo = new Map<number, InheritedRef[]>();
  const of = (id: number, depth: number): InheritedRef[] => {
    const hit = memo.get(id);
    if (hit) return hit;
    const g = byId.get(id);
    if (!g || depth > 50) return [];
    const own: InheritedRef[] = [
      ...g.projetos.map((ref) => ({ kind: "projetos" as const, ref, via: g.full_path })),
      ...(g.hosts ?? []).map((ref) => ({ kind: "hosts" as const, ref, via: g.full_path })),
    ];
    const out = [...(g.parent_id != null ? of(g.parent_id, depth + 1) : []), ...own];
    memo.set(id, out);
    return out;
  };
  for (const g of groups) of(g.id, 0);
  return memo;
}

export function attachmentCount(r: GitLabInventoryRepo, inherited: InheritedRef[] = []): number {
  return r.projetos.length + r.services.length + r.dns.length + r.hosts.length + inherited.length;
}

/** Search matches the repo path or the name of anything it is attached to —
 *  links inherited from its groups included. */
export function repoMatches(r: GitLabInventoryRepo, query: string, filter: RepoFilter, inherited: InheritedRef[] = []): boolean {
  const n = attachmentCount(r, inherited);
  if (filter === "linked" && n === 0) return false;
  if (filter === "unlinked" && n > 0) return false;
  if (filter !== "all" && filter !== "linked" && filter !== "unlinked" &&
    r[filter].length + inherited.filter((i) => i.kind === filter).length === 0) return false;
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return [r.path_with_namespace, ...[r.projetos, r.services, r.dns, r.hosts].flat().map((x) => x.name), ...inherited.map((i) => i.ref.name)]
    .some((s) => s.toLowerCase().includes(q));
}

/** Nests groups by parent_id and puts each kept repo under its namespace.
 *  With `prune`, groups with no kept repo below them are dropped. Repos whose
 *  namespace is not a listed group (a user namespace) come back as loose. */
export function buildTree(groups: GitLabInventoryGroup[], repos: GitLabInventoryRepo[], keep: (r: GitLabInventoryRepo) => boolean, prune: boolean): { roots: GroupNode[]; loose: GitLabInventoryRepo[] } {
  const nodes = new Map<number, GroupNode>();
  for (const g of groups) nodes.set(g.id, { group: g, children: [], repos: [] });
  const loose: GitLabInventoryRepo[] = [];
  for (const r of repos) {
    if (!keep(r)) continue;
    const n = nodes.get(r.namespace_id);
    if (n) n.repos.push(r);
    else loose.push(r);
  }
  const roots: GroupNode[] = [];
  for (const n of nodes.values()) {
    const parent = n.group.parent_id != null ? nodes.get(n.group.parent_id) : undefined;
    (parent ? parent.children : roots).push(n);
  }
  const byPath = (a: GroupNode, b: GroupNode) => a.group.full_path.localeCompare(b.group.full_path);
  const tidy = (list: GroupNode[]): GroupNode[] => {
    for (const n of list) n.children = tidy(n.children);
    const kept = prune ? list.filter((n) => n.repos.length > 0 || n.children.length > 0) : list;
    return kept.sort(byPath);
  };
  return { roots: tidy(roots), loose };
}

/** Repos in a node and all its subgroups. */
export function repoTotal(n: GroupNode): number {
  return n.repos.length + n.children.reduce((s, c) => s + repoTotal(c), 0);
}
