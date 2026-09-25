// Grouping inventory lists by a related entity (hosts by service, DNS by
// project, ...). Links come from GET /api/relations; names come from the
// related entity's own list, which is also what makes an id groupable: a link
// to something the caller can't name (deleted, out of scope) is dropped.

export type GroupEntity = "host" | "dns" | "service" | "project" | "contact" | "entidade";

export interface Relation {
  a: string;
  a_id: number;
  b: string;
  b_id: number;
}

export interface ItemGroup<T> {
  /** Related entity id, or null for the items linked to none of them. */
  id: number | null;
  label: string;
  items: T[];
}

/**
 * Buckets items (of type `entity`) under each related `by` entity they link
 * to. An item linked to several appears in each; items keep their incoming
 * order (the page's sort), groups are sorted by name, and the unlinked bucket
 * comes last under `unlinkedLabel`.
 */
export function groupItems<T extends { id: number }>(
  items: T[],
  entity: GroupEntity,
  by: GroupEntity,
  relations: Relation[],
  names: Map<number, string>,
  unlinkedLabel: string,
): ItemGroup<T>[] {
  const related = new Map<number, Set<number>>();
  const link = (itemId: number, groupId: number) => {
    if (!names.has(groupId)) return;
    if (!related.has(itemId)) related.set(itemId, new Set());
    related.get(itemId)!.add(groupId);
  };
  for (const r of relations) {
    if (r.a === entity && r.b === by) link(r.a_id, r.b_id);
    else if (r.b === entity && r.a === by) link(r.b_id, r.a_id);
  }

  const groups = new Map<number, T[]>();
  const unlinked: T[] = [];
  for (const item of items) {
    const ids = related.get(item.id);
    if (!ids) {
      unlinked.push(item);
      continue;
    }
    for (const id of ids) {
      if (!groups.has(id)) groups.set(id, []);
      groups.get(id)!.push(item);
    }
  }

  const out: ItemGroup<T>[] = [...groups]
    .map(([id, groupItems]) => ({ id, label: names.get(id)!, items: groupItems }))
    .sort((x, y) => x.label.localeCompare(y.label));
  if (unlinked.length > 0) out.push({ id: null, label: unlinkedLabel, items: unlinked });
  return out;
}
