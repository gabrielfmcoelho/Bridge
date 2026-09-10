// The table skin SortableTable draws, exported so hand-rolled tables use the
// same head, zebra and cell padding instead of copying the strings.
export const tableClasses = {
  wrapper: "border border-[var(--border-default)] rounded-[var(--radius-lg)] overflow-x-auto",
  table: "w-full text-sm",
  headRow: "bg-[var(--bg-elevated)] text-[var(--text-muted)] text-xs uppercase tracking-wider",
  th: "text-left px-4 py-3 font-semibold",
  row: "border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors",
  rowAlt: "bg-[var(--bg-surface)]",
  td: "px-4 py-2.5",
  // Inline variant for tables inside a Card: hairline head, tight cells, no wrapper.
  compact: {
    table: "w-full text-sm",
    headRow: "border-b border-[var(--border-subtle)]",
    th: "text-left py-2 px-2 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider",
    row: "border-b border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors",
    td: "py-2 px-2",
  },
};
