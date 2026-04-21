"use client";

import { useState, useMemo } from "react";
import Card from "@/components/ui/Card";
import SortDropdown from "@/components/ui/SortDropdown";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import { parseServiceRow, parseContainerRow, portIcon } from "@/lib/utils";
import type { ParsedContainer } from "@/lib/api";

/* ─── Generic types ─── */

interface Column<T> {
  key: string;
  label: string;
  getValue: (row: T) => string;
  getNumeric: (row: T) => number;
  colorFn?: (row: T) => string;
}

interface SortableResourceListProps<T> {
  title: string;
  rows: T[];
  columns: Column<T>[];
  getIcon: (row: T) => React.ReactNode;
  getName: (row: T) => string;
  defaultSort?: string;
  /** Optional card-mode element rendered to the right of the title (aligned
   *  with ml-auto). Used for e.g. a container hash badge. */
  renderTitleExtra?: (row: T) => React.ReactNode;
  /** Optional card-mode block rendered between the title and the metric
   *  grid. Good for secondary identity info (image, status). */
  renderCardMeta?: (row: T) => React.ReactNode;
  /** Optional card-mode footer rendered below the metric grid. */
  renderCardExtra?: (row: T) => React.ReactNode;
}

/* ─── Generic sortable resource list ─── */

function SortableResourceList<T>({ title, rows, columns, getIcon, getName, defaultSort = "name", renderTitleExtra, renderCardMeta, renderCardExtra }: SortableResourceListProps<T>) {
  const [sortKey, setSortKey] = useState(defaultSort);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");

  const sortOptions = [{ key: "name", label: "Name" }, ...columns.map((c) => ({ key: c.key, label: c.label }))];

  const sorted = useMemo(() => {
    const arr = [...rows];
    arr.sort((a, b) => {
      let cmp = 0;
      if (sortKey === "name") {
        cmp = getName(a).localeCompare(getName(b));
      } else {
        const col = columns.find((c) => c.key === sortKey);
        if (col) cmp = col.getNumeric(a) - col.getNumeric(b);
      }
      return sortDir === "desc" ? -cmp : cmp;
    });
    return arr;
  }, [rows, sortKey, sortDir, columns, getName]);

  // Build SortableTable column config
  const tableColumns = [
    { key: "name" as const, label: columns.length > 0 ? "Name" : "Name" },
    ...columns.map((c) => ({ key: c.key as string, label: c.label, align: "right" as const })),
  ];

  // Build sort comparators for table mode
  const comparators: Record<string, (a: T, b: T) => number> = {
    name: (a, b) => getName(a).localeCompare(getName(b)),
  };
  columns.forEach((c) => {
    comparators[c.key] = (a, b) => c.getNumeric(a) - c.getNumeric(b);
  });

  return (
    <>
    <div className="flex items-center justify-between mb-3">
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">{title}</h3>
      <div className="flex items-center gap-2">
        {viewMode === "cards" && <SortDropdown options={sortOptions} value={sortKey} direction={sortDir} onChange={(k, d) => { setSortKey(k); setSortDir(d); }} />}
        <ViewToggle
          value={viewMode}
          onChange={(v) => setViewMode(v as "cards" | "table")}
          options={[
            { key: "cards", label: "Cards", icon: VIEW_ICONS.cards },
            { key: "table", label: "Table", icon: VIEW_ICONS.table },
          ]}
        />
      </div>
    </div>
    {viewMode === "cards" ? (
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-2">
        {sorted.map((row, i) => (
          <Card key={i} hover={false} className="!p-3 flex flex-col">
            <div className="flex items-center gap-2 mb-1">
              {getIcon(row)}
              <span className="text-sm font-medium text-[var(--text-primary)] truncate flex-1 min-w-0" style={{ fontFamily: "var(--font-mono)" }}>{getName(row)}</span>
              {renderTitleExtra && renderTitleExtra(row)}
            </div>
            {renderCardMeta && renderCardMeta(row)}
            <div className={`grid grid-cols-${columns.length} gap-3 text-xs ${renderCardMeta ? "pt-2 border-t border-[var(--border-subtle)]/50" : "mt-1"}`}>
              {columns.map((col) => (
                <div key={col.key}>
                  <span className="text-[var(--text-faint)] block mb-0.5">{col.label}</span>
                  <span className={`font-medium ${col.colorFn ? col.colorFn(row) : "text-[var(--text-secondary)]"}`} style={{ fontFamily: "var(--font-mono)" }}>{col.getValue(row)}</span>
                </div>
              ))}
            </div>
            {renderCardExtra && renderCardExtra(row)}
          </Card>
        ))}
      </div>
    ) : (
      <SortableTable columns={tableColumns} defaultSort="name">
        {(sk, sd) => {
          const s = sortRows(sorted, sk, sd, comparators);
          return s.map((row, i) => (
            <tr key={i} className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}>
              <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>
                <span className="flex items-center gap-1.5">{getIcon(row)} {getName(row)}</span>
              </td>
              {columns.map((col) => (
                <td key={col.key} className={`px-4 py-2.5 text-right ${col.colorFn ? col.colorFn(row) : "text-[var(--text-secondary)]"}`} style={{ fontFamily: "var(--font-mono)" }}>{col.getValue(row)}</td>
              ))}
            </tr>
          ));
        }}
      </SortableTable>
    )}
    </>
  );
}

/* ─── Service-specific types ─── */

type ServiceRow = ReturnType<typeof parseServiceRow>;
type ContainerRow = ReturnType<typeof parseContainerRow>;

function pctColor(pct: number): string {
  return pct >= 80 ? "text-red-400" : pct >= 50 ? "text-amber-400" : "text-[var(--text-secondary)]";
}

/* ─── ServicesList convenience wrapper ─── */

export function ServicesList({ details, title }: { details: string[]; title: string }) {
  const rows = useMemo(() => details.map(parseServiceRow), [details]);

  const columns: Column<ServiceRow>[] = [
    { key: "cpu", label: "CPU", getValue: (r) => r.cpu, getNumeric: (r) => r.cpuNum, colorFn: (r) => pctColor(r.cpuNum) },
    { key: "mem", label: "MEM", getValue: (r) => r.mem, getNumeric: (r) => r.memNum, colorFn: (r) => pctColor(r.memNum) },
    { key: "rss", label: "RSS", getValue: (r) => r.rss, getNumeric: (r) => r.rssNum, colorFn: () => "text-[var(--text-muted)]" },
  ];

  return (
    <SortableResourceList
      title={title}
      rows={rows}
      columns={columns}
      getIcon={(row) => <span className="text-purple-400">{portIcon(row.name)}</span>}
      getName={(row) => row.name}
    />
  );
}

/* ─── ContainersList convenience wrapper ─── */

export function ContainersList({ stats, parsedContainers = [], title }: { stats: string[]; parsedContainers?: ParsedContainer[]; title: string }) {
  const rows = useMemo(() => stats.map(parseContainerRow), [stats]);

  const columns: Column<ContainerRow>[] = [
    { key: "cpu", label: "CPU", getValue: (r) => r.cpu, getNumeric: (r) => r.cpuNum, colorFn: (r) => pctColor(r.cpuNum) },
    { key: "mem", label: "Memory", getValue: (r) => r.mem, getNumeric: (r) => r.memNum, colorFn: () => "text-[var(--text-secondary)]" },
    { key: "net", label: "Net I/O", getValue: (r) => r.net, getNumeric: (r) => r.name.charCodeAt(0), colorFn: () => "text-[var(--text-muted)]" },
  ];

  // Index parsed containers by name for O(1) lookup. `stats` (docker stats)
  // and `parsedContainers` (docker ps) come from separate commands, so we
  // tolerate missing matches — the card falls back to the stats row only.
  const containersByName = useMemo(() => {
    const m = new Map<string, ParsedContainer>();
    for (const c of parsedContainers) m.set(c.name, c);
    return m;
  }, [parsedContainers]);

  return (
    <SortableResourceList
      title={title}
      rows={rows}
      columns={columns}
      getIcon={() => <span className="text-base">{"\uD83D\uDC33"}</span>}
      getName={(row) => row.name}
      renderTitleExtra={(row) => {
        const c = containersByName.get(row.name);
        // Short container ID in the PID-badge style — 12 chars is the
        // conventional "short ID" Docker shows in its CLI output.
        const shortId = c?.id ? c.id.slice(0, 12) : null;
        if (!shortId) return null;
        return (
          <span
            className="ml-auto shrink-0 px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[10px] text-[var(--text-faint)] border border-[var(--border-subtle)]"
            style={{ fontFamily: "var(--font-mono)" }}
            title={c?.id}
          >
            {shortId}
          </span>
        );
      }}
      renderCardMeta={(row) => {
        const c = containersByName.get(row.name);
        if (!c) return null;
        const uptime = parseContainerUptime(c.status);
        const up = uptime.up;
        return (
          <div className="mb-2 space-y-1">
            {c.image && (
              <p
                className="text-[10px] text-[var(--text-muted)] break-all leading-relaxed line-clamp-2"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {c.image}
              </p>
            )}
            {c.status && (
              <div className="flex items-center gap-1.5 text-[10px]">
                <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${up ? "bg-emerald-400" : "bg-[var(--text-faint)]"}`} />
                <span className={up ? "text-emerald-400" : "text-[var(--text-muted)]"} style={{ fontFamily: "var(--font-mono)" }}>
                  {uptime.display}
                </span>
              </div>
            )}
          </div>
        );
      }}
      renderCardExtra={(row) => {
        const c = containersByName.get(row.name);
        const bindings = parseContainerPortBindings(c?.ports ?? "");
        if (bindings.length === 0) return null;
        return (
          <div className="mt-2 pt-2 border-t border-[var(--border-subtle)]/50 flex flex-wrap gap-1">
            {bindings.map((b, i) => (
              <span
                key={i}
                className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full border text-[10px] bg-cyan-500/15 text-cyan-200 light:text-cyan-800 border-cyan-500/40"
                style={{ fontFamily: "var(--font-mono)" }}
                title={`host :${b.hostPort} → container :${b.containerPort}/${b.proto}`}
              >
                :{b.hostPort}
                <span className="opacity-70">→{b.containerPort}</span>
              </span>
            ))}
          </div>
        );
      }}
    />
  );
}

/**
 * Parses Docker's status column into a clean display value and a boolean
 * indicating whether the container is currently running. Status looks like:
 *   "Up 3 hours"
 *   "Up 12 minutes (healthy)"
 *   "Exited (0) 2 hours ago"
 *   "Restarting (1) 30 seconds ago"
 * We normalize the prefix ("Up") to uppercase-first, keep the rest verbatim.
 */
function parseContainerUptime(status: string): { up: boolean; display: string } {
  const trimmed = status.trim();
  if (!trimmed) return { up: false, display: "" };
  const up = /^up\b/i.test(trimmed);
  return { up, display: trimmed };
}

type ContainerPortBinding = { hostPort: string; containerPort: string; proto: string };

/**
 * Parses the `Ports` column from `docker ps` into a deduplicated list of
 * host→container bindings. Handles the common formats:
 *   0.0.0.0:8080->80/tcp
 *   0.0.0.0:8080->80/tcp, :::8080->80/tcp
 *   0.0.0.0:8080-8085->80-85/tcp
 * Bindings with no host publish (pure expose) are ignored.
 */
function parseContainerPortBindings(raw: string): ContainerPortBinding[] {
  if (!raw) return [];
  const seen = new Set<string>();
  const out: ContainerPortBinding[] = [];
  for (const part of raw.split(",")) {
    const m = part.trim().match(/(?:[\d.]+|\[?::\]?|\*):(\d+(?:-\d+)?)->(\d+(?:-\d+)?)\/(\w+)/);
    if (!m) continue;
    const [, host, cont, proto] = m;
    const key = `${host}|${cont}|${proto}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ hostPort: host, containerPort: cont, proto });
  }
  return out;
}

export default SortableResourceList;
