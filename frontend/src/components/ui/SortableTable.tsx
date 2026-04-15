"use client";

import { useState, useMemo, type ReactNode } from "react";

export interface SortableColumn<K extends string> {
  key: K;
  label: string;
  align?: "left" | "right" | "center";
}

interface SortableTableProps<K extends string> {
  columns: SortableColumn<K>[];
  defaultSort?: K;
  defaultDir?: "asc" | "desc";
  children: (sortKey: K, sortDir: "asc" | "desc") => ReactNode;
}

export default function SortableTable<K extends string>({
  columns,
  defaultSort,
  defaultDir = "asc",
  children,
}: SortableTableProps<K>) {
  const [sortKey, setSortKey] = useState<K>(defaultSort ?? columns[0].key);
  const [sortDir, setSortDir] = useState<"asc" | "desc">(defaultDir);

  const toggleSort = (key: K) => {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir(key === ("name" as K) || key === ("nickname" as K) || key === ("title" as K) || key === ("user" as K) ? "asc" : "desc");
    }
  };

  return (
    <div className="border border-[var(--border-default)] rounded-[var(--radius-lg)] overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-[var(--bg-elevated)] text-[var(--text-muted)] text-[11px] uppercase tracking-wider">
            {columns.map((col) => (
              <th
                key={col.key}
                onClick={() => toggleSort(col.key)}
                className={`${col.align === "right" ? "text-right" : col.align === "center" ? "text-center" : "text-left"} px-4 py-3 font-semibold cursor-pointer select-none hover:text-[var(--text-secondary)] transition-colors`}
              >
                <span className="inline-flex items-center gap-1">
                  {col.label}
                  {sortKey === col.key && (
                    <svg className={`w-3 h-3 text-[var(--accent)] transition-transform ${sortDir === "desc" ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M5 15l7-7 7 7" />
                    </svg>
                  )}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children(sortKey, sortDir)}</tbody>
      </table>
    </div>
  );
}

/** Generic sort helper */
export function sortRows<T, K extends string>(
  rows: T[],
  key: K,
  dir: "asc" | "desc",
  comparators: Record<K, (a: T, b: T) => number>
): T[] {
  const arr = [...rows];
  const cmp = comparators[key];
  if (cmp) arr.sort((a, b) => dir === "desc" ? -cmp(a, b) : cmp(a, b));
  return arr;
}
