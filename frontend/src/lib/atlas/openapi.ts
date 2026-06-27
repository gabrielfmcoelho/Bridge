// Pure OpenAPI spec helpers — no React, no I/O — so they stay trivially
// unit-testable (and reusable by the share index sidebar).

export interface SpecOperation {
  method: string; // upper-case HTTP verb, e.g. "GET"
  path: string; // template path, e.g. "/things/{id}"
  summary: string; // human label; falls back to operationId, else ""
  tags: string[]; // OpenAPI tags; [] if untagged
}

// The HTTP methods OpenAPI defines on a Path Item Object. Anything else under a
// path (parameters, servers, $ref, …) is ignored.
const HTTP_METHODS = ["get", "put", "post", "delete", "patch", "options", "head"] as const;

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// operationsFromSpec parses an OpenAPI document's `paths` into a flat, ordered
// list of {method, path, summary}. Defensive against malformed/partial specs:
// non-object inputs yield []. Order follows the spec's path declaration order,
// then HTTP_METHODS order within each path.
export function operationsFromSpec(spec: unknown): SpecOperation[] {
  if (!isRecord(spec)) return [];
  const paths = spec.paths;
  if (!isRecord(paths)) return [];

  const out: SpecOperation[] = [];
  for (const [path, item] of Object.entries(paths)) {
    if (!isRecord(item)) continue;
    for (const method of HTTP_METHODS) {
      const op = item[method];
      if (!isRecord(op)) continue;
      const summary =
        (typeof op.summary === "string" && op.summary) ||
        (typeof op.operationId === "string" && op.operationId) ||
        "";
      const tags = Array.isArray(op.tags)
        ? op.tags.filter((x): x is string => typeof x === "string")
        : [];
      out.push({ method: method.toUpperCase(), path, summary, tags });
    }
  }
  return out;
}

export interface SpecTagGroup {
  tag: string | null; // null = untagged ("Other")
  operations: SpecOperation[];
}

// groupOperationsByTag buckets operations by their primary (first) tag,
// preserving first-appearance order; untagged operations collect under a single
// null group, which is sorted last for readability.
export function groupOperationsByTag(ops: SpecOperation[]): SpecTagGroup[] {
  const order: string[] = [];
  const byTag = new Map<string, SpecOperation[]>();
  for (const op of ops) {
    const key = op.tags[0] ?? "";
    if (!byTag.has(key)) {
      byTag.set(key, []);
      order.push(key);
    }
    byTag.get(key)!.push(op);
  }
  const groups: SpecTagGroup[] = order.map((key) => ({
    tag: key === "" ? null : key,
    operations: byTag.get(key)!,
  }));
  // Stable sort (ES2019+) keeps tagged groups in first-appearance order and
  // pushes the single untagged group to the end.
  groups.sort((a, b) => (a.tag === null ? 1 : 0) - (b.tag === null ? 1 : 0));
  return groups;
}
