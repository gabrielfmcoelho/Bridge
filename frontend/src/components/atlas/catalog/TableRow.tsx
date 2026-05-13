"use client";

import { useLocale } from "@/contexts/LocaleContext";
import type { TableRecord } from "@/lib/atlas/types";
import LayerBadge from "../shared/LayerBadge";
import RoleBadge from "../shared/RoleBadge";
import { qualifiedName } from "./TableCard";

interface Props {
  table: TableRecord;
  selected?: boolean;
  onClick: () => void;
}

/** Compact horizontal row for the Tree+List view. */
export default function TableRow({ table, selected, onClick }: Props) {
  const { t } = useLocale();
  const fq = qualifiedName(table);

  return (
    <button
      onClick={onClick}
      className={`group w-full text-left flex items-center gap-3 px-3 py-2 rounded-[var(--radius-md)] border transition-all ${
        selected
          ? "bg-[var(--accent-muted)] border-[var(--accent)]/40"
          : "bg-[var(--bg-surface)] border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] hover:border-[var(--border-default)]"
      }`}
    >
      <div className="flex items-center gap-2 min-w-0 flex-1">
        <LayerBadge layer={table.layer} size="sm" />
        <span
          className={`text-[12px] font-mono truncate ${
            selected ? "text-[var(--accent)]" : "text-[var(--text-primary)]"
          }`}
          title={fq}
        >
          {table.node.label}
        </span>
        {table.hasWarning && <span className="w-1.5 h-1.5 rounded-full bg-red-500 shrink-0" />}
      </div>

      <span className="text-[10px] text-[var(--text-muted)] font-mono truncate min-w-0 hidden md:inline">{fq}</span>

      <span className="text-[10px] text-[var(--text-muted)] tabular-nums shrink-0">
        <span className="text-[var(--text-secondary)] font-semibold">{table.columnCount}</span>{" "}
        {t("atlas.catalog.list.columnsLabel")}
      </span>
      <RoleBadge role={table.role} size="sm" />
      <svg
        className="w-3 h-3 text-[var(--text-faint)] opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
      </svg>
    </button>
  );
}
