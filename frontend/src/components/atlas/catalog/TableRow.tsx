"use client";

import { useLocale } from "@/contexts/LocaleContext";
import type { TableRecord } from "@/lib/atlas/types";
import LayerBadge from "../shared/LayerBadge";
import RoleBadge from "../shared/RoleBadge";
import { qualifiedName } from "./TableCard";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import StatusDot from "@/components/ui/StatusDot";

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
      className={`group w-full text-left flex items-center gap-3 px-3 py-2 rounded-[var(--radius-md)] border transition ${
        selected
          ? "bg-[var(--accent-muted)] border-[var(--accent)]/40"
          : "bg-[var(--bg-surface)] border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] hover:border-[var(--border-default)]"
      }`}
    >
      <div className="flex items-center gap-2 min-w-0 flex-1">
        <LayerBadge layer={table.layer} size="sm" />
        <span
          className={`text-xs font-mono truncate ${
            selected ? "text-[var(--accent)]" : "text-[var(--text-primary)]"
          }`}
          title={fq}
        >
          {table.node.label}
        </span>
        {table.hasWarning && <StatusDot size="xs" className="bg-red-500" />}
      </div>

      <span className="text-2xs text-[var(--text-muted)] font-mono truncate min-w-0 hidden md:inline">{fq}</span>

      <span className="text-2xs text-[var(--text-muted)] tabular-nums shrink-0">
        <span className="text-[var(--text-secondary)] font-semibold">{table.columnCount}</span>{" "}
        {t("atlas.catalog.list.columnsLabel")}
      </span>
      <RoleBadge role={table.role} size="sm" />
      <Icon path={ICON_PATHS.chevronRight} className="w-3 h-3 text-[var(--text-faint)] opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
    </button>
  );
}
