"use client";

import Button from "./Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface PaginationProps {
  page: number;
  totalPages: number;
  total: number;
  perPage: number;
  onChange: (page: number) => void;
}

export default function Pagination({ page, totalPages, total, perPage, onChange }: PaginationProps) {
  if (totalPages <= 1) return null;

  const start = (page - 1) * perPage + 1;
  const end = Math.min(page * perPage, total);

  return (
    <div className="flex items-center justify-between pt-3 mt-3 border-t border-[var(--border-subtle)]">
      <span className="text-xs text-[var(--text-faint)]">
        {start}–{end} of {total}
      </span>
      <div className="flex items-center gap-1">
        <Button size="sm" variant="ghost" disabled={page <= 1} onClick={() => onChange(page - 1)} className="!p-2">
          <Icon path={ICON_PATHS.back} />
        </Button>
        <span className="text-xs text-[var(--text-secondary)] px-2 font-mono">
          {page}/{totalPages}
        </span>
        <Button size="sm" variant="ghost" disabled={page >= totalPages} onClick={() => onChange(page + 1)} className="!p-2">
          <Icon path={ICON_PATHS.chevronRight} />
        </Button>
      </div>
    </div>
  );
}
