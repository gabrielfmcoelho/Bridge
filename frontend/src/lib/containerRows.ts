// No runtime imports: Node's test runner loads this file directly.
import type { ParsedContainer } from "./api";

/** Parse a container_stats entry. */
export function parseContainerRow(raw: string) {
  const nameMatch = raw.match(/^(.+?)\s+CPU:/);
  const name = nameMatch?.[1]?.trim() || raw.split(/\s+/)[0] || raw;
  const cpuMatch = raw.match(/CPU:([\d.]+%?)/);
  const cpu = cpuMatch?.[1] || "";
  const memMatch = raw.match(/MEM:(.+?)(?:\s+NET:|$)/);
  const mem = memMatch?.[1]?.trim() || "";
  const netMatch = raw.match(/NET:(.+)$/);
  const net = netMatch?.[1]?.trim() || "";
  return { name, cpu, mem, net, cpuNum: parseFloat(cpu) || 0, memNum: parseFloat(mem) || 0 };
}

export type ContainerRow = ReturnType<typeof parseContainerRow> & { container?: ParsedContainer };

/**
 * One row per container: `docker ps` (parsed_containers) is the list,
 * `docker stats` (container_stats) only adds CPU / memory / network. The two
 * come from separate commands, so either may be missing: a container without
 * stats still gets a row, and so does a stats line with no ps match.
 */
export function containerRows(info: { parsed_containers?: ParsedContainer[] | null; container_stats?: string[] | null }): ContainerRow[] {
  const stats = new Map((info.container_stats ?? []).map((raw) => {
    const r = parseContainerRow(raw);
    return [r.name, r] as const;
  }));
  const rows: ContainerRow[] = (info.parsed_containers ?? []).map((c) => {
    const s = stats.get(c.name);
    stats.delete(c.name);
    return { ...(s ?? { name: c.name, cpu: "", mem: "", net: "", cpuNum: 0, memNum: 0 }), container: c };
  });
  return [...rows, ...stats.values()];
}
