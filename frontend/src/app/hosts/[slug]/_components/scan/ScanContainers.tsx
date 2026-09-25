"use client";

import { useMemo, useState } from "react";
import SectionCard from "@/components/ui/SectionCard";
import SortDropdown from "@/components/ui/SortDropdown";
import StatusDot from "@/components/ui/StatusDot";
import { parseContainerRow } from "@/lib/utils";
import type { VMInfoType, ParsedContainer } from "@/lib/api";
import { Rows, Row, Detail, pctColor, type T } from "./shared";

type SortKey = "name" | "cpu" | "mem";

/**
 * Docker containers as compact rows — state, name, image, CPU, memory, short
 * id — sortable; a row expands to status, network I/O and published ports.
 * `container_stats` (docker stats) and `parsed_containers` (docker ps) come
 * from separate commands, so a stats row without a ps match still renders.
 */
export default function ScanContainers({ info, t }: { info: VMInfoType; t: T }) {
  const [sort, setSort] = useState<{ key: SortKey; dir: "asc" | "desc" }>({ key: "name", dir: "asc" });
  const byName = useMemo(() => new Map((info.parsed_containers ?? []).map((c) => [c.name, c])), [info.parsed_containers]);
  const rows = useMemo(() => {
    const list = (info.container_stats ?? []).map(parseContainerRow);
    list.sort((a, b) => {
      const cmp = sort.key === "name" ? a.name.localeCompare(b.name) : sort.key === "cpu" ? a.cpuNum - b.cpuNum : a.memNum - b.memNum;
      return sort.dir === "desc" ? -cmp : cmp;
    });
    return list;
  }, [info.container_stats, sort]);

  return (
    <SectionCard
      as="h3"
      title={t("scan.pane.group.containers")}
      count={rows.length || undefined}
      body="flush"
      empty={rows.length === 0 ? t("scan.pane.empty.containers") : undefined}
      controls={rows.length > 1 && (
        <SortDropdown<SortKey>
          options={[{ key: "name", label: t("sort.name") }, { key: "cpu", label: t("vm.cpu") }, { key: "mem", label: t("vm.ram") }]}
          value={sort.key}
          direction={sort.dir}
          onChange={(key, dir) => setSort({ key, dir })}
        />
      )}
    >
      <Rows>
        {rows.map((r) => {
          const c: ParsedContainer | undefined = byName.get(r.name);
          const up = !!c?.status && /^up\b/i.test(c.status.trim());
          const bindings = parseContainerPortBindings(c?.ports ?? "");
          return (
            <Row
              key={r.name}
              summary={
                <>
                  <StatusDot size="xs" color={up ? "success" : "muted"} title={c?.status} />
                  <span className="w-40 shrink-0 font-mono font-medium text-[var(--text-primary)] truncate">{r.name}</span>
                  <span className="flex-1 min-w-0 truncate font-mono text-[var(--text-muted)]" title={c?.image}>{c?.image || "–"}</span>
                  <span className={`shrink-0 w-14 text-right font-mono tabular-nums ${pctColor(r.cpuNum)}`}>{r.cpu || "–"}</span>
                  <span className="hidden sm:inline shrink-0 w-32 text-right font-mono tabular-nums text-[var(--text-secondary)] truncate">{r.mem || "–"}</span>
                  <span className="hidden md:inline shrink-0 w-24 text-right font-mono text-[var(--text-faint)]" title={c?.id}>{c?.id ? c.id.slice(0, 12) : "–"}</span>
                </>
              }
            >
              <Detail label={t("common.status")}>{c?.status || "–"}</Detail>
              <Detail label="Net I/O">{r.net || "–"}</Detail>
              {bindings.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {bindings.map((b, i) => (
                    <span
                      key={i}
                      className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full border text-2xs bg-[var(--cyan)]/15 text-[var(--cyan)] border-[var(--cyan)]/40 font-mono"
                      title={t("host.ops.portBindingTooltip", { hostPort: b.hostPort, containerPort: b.containerPort, proto: b.proto })}
                    >
                      :{b.hostPort}<span className="opacity-70">→{b.containerPort}</span>
                    </span>
                  ))}
                </div>
              )}
            </Row>
          );
        })}
      </Rows>
    </SectionCard>
  );
}

type ContainerPortBinding = { hostPort: string; containerPort: string; proto: string };

/**
 * `docker ps` Ports column → deduplicated host→container bindings, e.g.
 * "0.0.0.0:8080->80/tcp, :::8080->80/tcp". Pure exposes are ignored.
 */
function parseContainerPortBindings(raw: string): ContainerPortBinding[] {
  if (!raw) return [];
  const seen = new Set<string>();
  const out: ContainerPortBinding[] = [];
  for (const part of raw.split(",")) {
    const m = part.trim().match(/(?:[\d.]+|\[?::\]?|\*):(\d+(?:-\d+)?)->(\d+(?:-\d+)?)\/(\w+)/);
    if (!m) continue;
    const [, hostPort, containerPort, proto] = m;
    const key = `${hostPort}|${containerPort}|${proto}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ hostPort, containerPort, proto });
  }
  return out;
}
