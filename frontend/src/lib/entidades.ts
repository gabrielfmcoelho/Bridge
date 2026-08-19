import type { Entidade, User } from "./types";

/** Entidade plus its depth in the tree (root = 0). */
export type EntidadeNode = Entidade & { depth: number };

/**
 * Adds depth to a flat entidade list. The API returns parents before children
 * (ordered by path), so a single pass suffices; an orphan (unknown parent) is
 * treated as a root.
 */
export function withDepth(entidades: Entidade[]): EntidadeNode[] {
  const depth = new Map<number, number>();
  return entidades.map((e) => {
    const d = e.parent_id != null && depth.has(e.parent_id) ? depth.get(e.parent_id)! + 1 : 0;
    depth.set(e.id, d);
    return { ...e, depth: d };
  });
}

/** Ids of every entidade in `roots` plus all their descendants. */
export function descendantIds(entidades: Entidade[], roots: Iterable<number>): Set<number> {
  const out = new Set<number>(roots);
  // Parents come first, so one forward pass closes the set.
  for (const e of entidades) {
    if (e.parent_id != null && out.has(e.parent_id)) out.add(e.id);
  }
  return out;
}

/**
 * Entidades the user may pick as creator of a new asset: admins see all;
 * everyone else only their own memberships and descendants (the server
 * enforces the same rule — this just keeps the picker honest).
 */
export function creatorOptions(entidades: Entidade[], user: Pick<User, "role" | "entidades"> | null): EntidadeNode[] {
  const nodes = withDepth(entidades);
  if (!user || user.role === "admin") return nodes;
  const allowed = descendantIds(entidades, (user.entidades ?? []).map((e) => e.id));
  return nodes.filter((e) => allowed.has(e.id));
}

/** Label indented by depth for flat <Select> option lists. */
export function indentedLabel(e: EntidadeNode): string {
  return `${"  ".repeat(e.depth)}${e.depth > 0 ? "↳ " : ""}${e.name}`;
}
