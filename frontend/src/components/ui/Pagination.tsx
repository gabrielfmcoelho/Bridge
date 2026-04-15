"use client";

import Button from "./Button";

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
        <Button size="sm" variant="ghost" disabled={page <= 1} onClick={() => onChange(page - 1)}>
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
        </Button>
        <span className="text-xs text-[var(--text-secondary)] px-2" style={{ fontFamily: "var(--font-mono)" }}>
          {page}/{totalPages}
        </span>
        <Button size="sm" variant="ghost" disabled={page >= totalPages} onClick={() => onChange(page + 1)}>
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
          </svg>
        </Button>
      </div>
    </div>
  );
}
