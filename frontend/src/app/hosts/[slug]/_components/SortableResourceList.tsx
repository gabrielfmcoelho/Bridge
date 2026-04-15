"use client";

import { useState, useMemo } from "react";
import Card from "@/components/ui/Card";
import SortDropdown from "@/components/ui/SortDropdown";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import { parseServiceRow, parseContainerRow, portIcon } from "@/lib/utils";

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
}

/* ─── Generic sortable resource list ─── */

function SortableResourceList<T>({ title, rows, columns, getIcon, getName, defaultSort = "name" }: SortableResourceListProps<T>) {
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
          <Card key={i} hover={false} className="!p-3">
            <div className="flex items-center gap-2 mb-2">
              {getIcon(row)}
              <span className="text-sm font-medium text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{getName(row)}</span>
            </div>
            <div className={`grid grid-cols-${columns.length} gap-3 text-xs`}>
              {columns.map((col) => (
                <div key={col.key}>
                  <span className="text-[var(--text-faint)] block mb-0.5">{col.label}</span>
                  <span className={`font-medium ${col.colorFn ? col.colorFn(row) : "text-[var(--text-secondary)]"}`} style={{ fontFamily: "var(--font-mono)" }}>{col.getValue(row)}</span>
                </div>
              ))}
            </div>
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

export function ContainersList({ stats, title }: { stats: string[]; title: string }) {
  const rows = useMemo(() => stats.map(parseContainerRow), [stats]);

  const columns: Column<ContainerRow>[] = [
    { key: "cpu", label: "CPU", getValue: (r) => r.cpu, getNumeric: (r) => r.cpuNum, colorFn: (r) => pctColor(r.cpuNum) },
    { key: "mem", label: "Memory", getValue: (r) => r.mem, getNumeric: (r) => r.memNum, colorFn: () => "text-[var(--text-secondary)]" },
    { key: "net", label: "Net I/O", getValue: (r) => r.net, getNumeric: (r) => r.name.charCodeAt(0), colorFn: () => "text-[var(--text-muted)]" },
  ];

  return (
    <SortableResourceList
      title={title}
      rows={rows}
      columns={columns}
      getIcon={() => <span className="text-base">{"\uD83D\uDC33"}</span>}
      getName={(row) => row.name}
    />
  );
}

export default SortableResourceList;
