"use client";

import { useState, useMemo, type ReactNode } from "react";
import { tableClasses } from "./Table";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export interface SortableColumn<K extends string> {
  key: K;
  label: string;
  align?: "left" | "right" | "center";
  // sortable defaults to true. Set false for columns that can't be sorted
  // (e.g. computed/non-server-sortable columns under server-side pagination).
  sortable?: boolean;
}

interface SortableTableProps<K extends string> {
  columns: SortableColumn<K>[];
  defaultSort?: K;
  defaultDir?: "asc" | "desc";
  // Controlled mode: when onSortChange is provided, the parent owns the sort
  // state (sortKey/sortDir) and SortableTable does NOT sort internally — header
  // clicks just call onSortChange. Used for server-driven sort+pagination. When
  // omitted, the table is uncontrolled (internal state + the children callback
  // receives the live sort for client-side sortRows).
  sortKey?: K;
  sortDir?: "asc" | "desc";
  onSortChange?: (key: K, dir: "asc" | "desc") => void;
  children: (sortKey: K, sortDir: "asc" | "desc") => ReactNode;
}

const ascByDefault = (k: string) => k === "name" || k === "nickname" || k === "title" || k === "user";

export default function SortableTable<K extends string>({
  columns,
  defaultSort,
  defaultDir = "asc",
  sortKey: controlledKey,
  sortDir: controlledDir,
  onSortChange,
  children,
}: SortableTableProps<K>) {
  const controlled = onSortChange != null;
  const [internalKey, setInternalKey] = useState<K>(defaultSort ?? columns[0].key);
  const [internalDir, setInternalDir] = useState<"asc" | "desc">(defaultDir);

  const sortKey = controlled ? (controlledKey ?? columns[0].key) : internalKey;
  const sortDir = controlled ? (controlledDir ?? defaultDir) : internalDir;

  const toggleSort = (key: K) => {
    const nextDir: "asc" | "desc" =
      sortKey === key ? (sortDir === "asc" ? "desc" : "asc") : ascByDefault(key) ? "asc" : "desc";
    if (controlled) {
      onSortChange(key, nextDir);
    } else {
      setInternalKey(key);
      setInternalDir(nextDir);
    }
  };

  return (
    <div className={tableClasses.wrapper}>
      <table className={tableClasses.table}>
        <thead>
          <tr className={tableClasses.headRow}>
            {columns.map((col) => {
              const sortable = col.sortable !== false;
              return (
                <th
                  key={col.key}
                  onClick={sortable ? () => toggleSort(col.key) : undefined}
                  className={`${col.align === "right" ? "text-right" : col.align === "center" ? "text-center" : "text-left"} px-3 py-2 font-semibold whitespace-nowrap select-none transition-colors ${sortable ? "cursor-pointer hover:text-[var(--text-secondary)]" : ""}`}
                >
                  <span className="inline-flex items-center gap-1">
                    {col.label}
                    {sortable && sortKey === col.key && (
                      <Icon path={ICON_PATHS.chevronUp} className={`w-3 h-3 text-[var(--accent)] transition-transform ${sortDir === "desc" ? "rotate-180" : ""}`} />
                    )}
                  </span>
                </th>
              );
            })}
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
