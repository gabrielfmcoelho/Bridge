"use client";

import { useState } from "react";

interface CheckboxListProps {
  label: string;
  items: { id: number; name: string }[];
  selected: number[];
  onChange: (selected: number[]) => void;
}

export default function CheckboxList({ label, items, selected, onChange }: CheckboxListProps) {
  const [search, setSearch] = useState("");

  if (items.length === 0) return null;

  const toggle = (id: number, checked: boolean) => {
    onChange(checked ? [...selected, id] : selected.filter((v) => v !== id));
  };

  const filtered = search
    ? items.filter((item) => item.name.toLowerCase().includes(search.toLowerCase()))
    : items;

  const selectedCount = selected.length;

  return (
    <div className="space-y-1.5">
      {label && (
        <div className="flex items-center justify-between">
          <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">{label}</label>
          {selectedCount > 0 && (
            <span className="text-[10px] text-[var(--accent)] font-semibold">{selectedCount} selected</span>
          )}
        </div>
      )}
      {items.length > 8 && (
        <div className="relative">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[var(--text-faint)] pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={`Search ${label.toLowerCase()}...`}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] pl-8 pr-2 py-1.5 text-xs focus:outline-none focus:border-[var(--accent)] placeholder:text-[var(--text-faint)]"
          />
        </div>
      )}
      <div className="flex flex-wrap gap-1.5 max-h-40 overflow-y-auto">
        {filtered.map((item) => (
          <label key={item.id} className={`flex items-center gap-2 text-sm md:text-xs bg-[var(--bg-elevated)] border rounded-[var(--radius-md)] px-3 py-2 md:px-2.5 md:py-1.5 cursor-pointer hover:border-[var(--border-default)] transition-colors ${
            selected.includes(item.id)
              ? "border-[var(--accent)]/30 text-[var(--accent)]"
              : "border-[var(--border-subtle)] text-[var(--text-secondary)]"
          }`}>
            <input
              type="checkbox"
              checked={selected.includes(item.id)}
              onChange={(e) => toggle(item.id, e.target.checked)}
              className="w-4 h-4 md:w-3.5 md:h-3.5 rounded accent-[var(--accent)]"
            />
            {item.name}
          </label>
        ))}
        {filtered.length === 0 && (
          <span className="text-xs text-[var(--text-faint)] py-2">No results</span>
        )}
      </div>
    </div>
  );
}
