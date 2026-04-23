"use client";

import { useMemo, useRef, useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import type { OutlineWorkspaceCollection } from "@/lib/api";

interface Props {
  label?: string;
  collections: OutlineWorkspaceCollection[];
  value: string[];
  onChange: (ids: string[]) => void;
  loading?: boolean;
  disabled?: boolean;
  emptyHint?: string;
}

export default function CollectionMultiSelect({
  label,
  collections,
  value,
  onChange,
  loading,
  disabled,
  emptyHint,
}: Props) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const byId = useMemo(() => {
    const m = new Map<string, OutlineWorkspaceCollection>();
    for (const c of collections) m.set(c.id, c);
    return m;
  }, [collections]);

  const selected = useMemo(() => {
    // Keep admin's ordering; fall back to bare id if collection metadata is missing.
    return value.map((id) => byId.get(id) ?? { id, name: id, url_id: "" } as OutlineWorkspaceCollection);
  }, [value, byId]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return collections;
    return collections.filter((c) =>
      c.name.toLowerCase().includes(q) || (c.description ?? "").toLowerCase().includes(q)
    );
  }, [collections, search]);

  const toggle = (id: string) => {
    if (value.includes(id)) onChange(value.filter((v) => v !== id));
    else onChange([...value, id]);
  };

  const remove = (id: string) => onChange(value.filter((v) => v !== id));

  return (
    <div className="space-y-1.5">
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}

      <div
        className={`w-full min-h-[42px] bg-[var(--bg-elevated)] border rounded-[var(--radius-md)] px-2 py-1.5 text-sm flex items-center flex-wrap gap-1.5 ${
          disabled ? "opacity-50" : ""
        } ${open ? "border-[var(--accent)] ring-2 ring-[var(--accent-muted)]" : "border-[var(--border-default)]"}`}
      >
        {selected.length === 0 && (
          <span className="text-[var(--text-faint)] px-1">
            {loading ? "Loading collections…" : emptyHint ?? "No collections selected"}
          </span>
        )}
        {selected.map((c) => (
          <span
            key={c.id}
            className="inline-flex items-center gap-1.5 bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] pl-1.5 pr-0.5 py-0.5 text-xs"
          >
            {c.color && (
              <span
                className="inline-block w-2 h-2 rounded-full shrink-0"
                style={{ backgroundColor: c.color }}
              />
            )}
            <span className="text-[var(--text-primary)] truncate max-w-[180px]">{c.name}</span>
            {!disabled && (
              <button
                type="button"
                onClick={() => remove(c.id)}
                className="w-4 h-4 inline-flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
                aria-label={`Remove ${c.name}`}
              >
                ×
              </button>
            )}
          </span>
        ))}

        <Popover.Root
          open={open}
          onOpenChange={(v) => {
            setOpen(v);
            if (!v) setSearch("");
          }}
        >
          <Popover.Trigger asChild disabled={disabled}>
            <button
              type="button"
              disabled={disabled}
              className="inline-flex items-center gap-1 text-xs text-[var(--accent)] hover:underline px-2 py-0.5 disabled:cursor-not-allowed"
            >
              + Add collection
            </button>
          </Popover.Trigger>
          <Popover.Portal>
            <Popover.Content
              side="bottom"
              align="start"
              sideOffset={6}
              className="z-[100] w-[min(420px,90vw)] bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg overflow-hidden animate-fade-in"
              onOpenAutoFocus={(e) => {
                e.preventDefault();
                inputRef.current?.focus();
              }}
            >
              <div className="p-1.5 border-b border-[var(--border-subtle)]">
                <input
                  ref={inputRef}
                  type="text"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Search collections…"
                  className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2.5 py-1.5 text-sm focus:outline-none focus:border-[var(--accent)]"
                />
              </div>
              <div className="max-h-72 overflow-y-auto">
                {filtered.length === 0 ? (
                  <div className="px-3 py-3 text-xs text-[var(--text-faint)]">
                    {loading ? "Loading…" : "No collections found"}
                  </div>
                ) : (
                  filtered.map((c) => {
                    const checked = value.includes(c.id);
                    return (
                      <button
                        key={c.id}
                        type="button"
                        onClick={() => toggle(c.id)}
                        className={`w-full text-left px-3 py-2 text-sm flex items-start gap-2 transition-colors ${
                          checked
                            ? "bg-[var(--accent-muted)]"
                            : "hover:bg-[var(--bg-elevated)]"
                        }`}
                      >
                        <span
                          className={`mt-0.5 inline-flex items-center justify-center w-4 h-4 rounded-[var(--radius-sm)] border ${
                            checked
                              ? "bg-[var(--accent)] border-[var(--accent)] text-white"
                              : "border-[var(--border-default)]"
                          }`}
                          aria-hidden
                        >
                          {checked && (
                            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                            </svg>
                          )}
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex items-center gap-1.5">
                            {c.color && (
                              <span
                                className="inline-block w-2 h-2 rounded-full shrink-0"
                                style={{ backgroundColor: c.color }}
                              />
                            )}
                            <span className="text-[var(--text-primary)] truncate">{c.name}</span>
                          </span>
                          {c.description && (
                            <span className="block text-[11px] text-[var(--text-muted)] truncate">
                              {c.description}
                            </span>
                          )}
                        </span>
                      </button>
                    );
                  })
                )}
              </div>
            </Popover.Content>
          </Popover.Portal>
        </Popover.Root>
      </div>
    </div>
  );
}
