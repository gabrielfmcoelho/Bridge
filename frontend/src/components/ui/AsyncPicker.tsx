"use client";

import { useEffect, useRef, useState } from "react";
import * as Popover from "@radix-ui/react-popover";

export interface AsyncPickerItem {
  id: number;
  label: string;
  secondary?: string;
  color?: string;
}

// Single-select variant.
interface SingleProps {
  multi?: false;
  value: number | null;
  onChange: (item: AsyncPickerItem | null) => void;
  selectedLabel?: string;
  // Multi-only props are not valid here.
  selectedItems?: never;
}

// Multi-select variant — the caller owns the selection list so the full item
// metadata (label/color) survives re-renders without a re-fetch.
interface MultiProps {
  multi: true;
  selectedItems: AsyncPickerItem[];
  onChange: (items: AsyncPickerItem[]) => void;
  // Single-only props are not valid here.
  value?: never;
  selectedLabel?: never;
}

type Props = (SingleProps | MultiProps) & {
  label?: string;
  error?: string;
  fetcher: (query: string) => Promise<AsyncPickerItem[]>;
  placeholder?: string;
  emptyLabel?: string;
  disabled?: boolean;
};

export default function AsyncPicker(props: Props) {
  const {
    label,
    error,
    fetcher,
    placeholder,
    emptyLabel,
    disabled,
  } = props;
  const multi = props.multi === true;
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [items, setItems] = useState<AsyncPickerItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetchErr, setFetchErr] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const reqTokenRef = useRef(0);

  // Debounced search — new fetch on every keystroke, ignoring out-of-order results.
  useEffect(() => {
    if (!open) return;
    setLoading(true);
    const token = ++reqTokenRef.current;
    const timer = setTimeout(() => {
      fetcher(query)
        .then((rows) => {
          if (reqTokenRef.current !== token) return;
          setItems(rows);
          setFetchErr(null);
        })
        .catch((err) => {
          if (reqTokenRef.current !== token) return;
          setFetchErr(err instanceof Error ? err.message : "fetch failed");
          setItems([]);
        })
        .finally(() => {
          if (reqTokenRef.current === token) setLoading(false);
        });
    }, 200);
    return () => clearTimeout(timer);
  }, [query, open, fetcher]);

  const selectedItems = multi ? props.selectedItems : [];
  const selectedIDs = new Set(selectedItems.map((i) => i.id));
  const singleLabel = !multi && props.value != null ? props.selectedLabel || `#${props.value}` : "";

  const pickSingle = (item: AsyncPickerItem | null) => {
    if (multi) return;
    (props.onChange as (i: AsyncPickerItem | null) => void)(item);
    setOpen(false);
    setQuery("");
  };

  const toggleMulti = (item: AsyncPickerItem) => {
    if (!multi) return;
    const on = props.onChange as (items: AsyncPickerItem[]) => void;
    on(
      selectedIDs.has(item.id)
        ? selectedItems.filter((s) => s.id !== item.id)
        : [...selectedItems, item]
    );
  };

  const removeMulti = (id: number) => {
    if (!multi) return;
    (props.onChange as (items: AsyncPickerItem[]) => void)(
      selectedItems.filter((s) => s.id !== id)
    );
  };

  return (
    <div className="space-y-1.5">
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <Popover.Root
        open={open}
        onOpenChange={(o) => {
          setOpen(o);
          if (!o) setQuery("");
        }}
      >
        <Popover.Trigger asChild disabled={disabled}>
          <button
            type="button"
            disabled={disabled}
            className={`w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border rounded-[var(--radius-md)] px-3 py-2 text-sm text-left flex items-center justify-between gap-2 disabled:opacity-40 ${
              open
                ? "border-[var(--accent)] ring-2 ring-[var(--accent-muted)]"
                : error
                ? "border-red-500"
                : "border-[var(--border-default)]"
            }`}
          >
            {multi ? (
              <span className="flex-1 flex flex-wrap gap-1 min-w-0">
                {selectedItems.length === 0 ? (
                  <span className="text-[var(--text-faint)]">
                    {placeholder || "Selecione…"}
                  </span>
                ) : (
                  selectedItems.map((s) => (
                    <span
                      key={s.id}
                      className="inline-flex items-center gap-1.5 bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] pl-1.5 pr-0.5 py-0.5 text-xs"
                    >
                      {s.color && (
                        <span
                          className="inline-block w-2 h-2 rounded-full shrink-0"
                          style={{ backgroundColor: s.color }}
                        />
                      )}
                      <span className="truncate max-w-[180px]">{s.label}</span>
                      {!disabled && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            removeMulti(s.id);
                          }}
                          className="w-4 h-4 inline-flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
                          aria-label={`Remover ${s.label}`}
                        >
                          ×
                        </button>
                      )}
                    </span>
                  ))
                )}
              </span>
            ) : (
              <span className={props.value != null ? "" : "text-[var(--text-faint)]"}>
                {singleLabel || placeholder || "Selecione…"}
              </span>
            )}
            <svg className={`w-3.5 h-3.5 text-[var(--text-faint)] shrink-0 transition-transform ${open ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
            </svg>
          </button>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            side="bottom"
            sideOffset={4}
            align="start"
            className="z-[100] bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg overflow-hidden animate-fade-in"
            style={{ width: "var(--radix-popover-trigger-width)" }}
            onOpenAutoFocus={(e) => {
              e.preventDefault();
              inputRef.current?.focus();
            }}
          >
            <div className="p-1.5 border-b border-[var(--border-subtle)]">
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={placeholder || "Buscar…"}
                className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2.5 py-1.5 text-sm focus:outline-none focus:border-[var(--accent)]"
              />
            </div>
            <div className="max-h-64 overflow-y-auto">
              {!multi && props.value != null && (
                <button
                  type="button"
                  onClick={() => pickSingle(null)}
                  className="w-full text-left px-3 py-1.5 text-xs text-[var(--text-muted)] hover:bg-[var(--bg-elevated)]"
                >
                  — limpar seleção —
                </button>
              )}
              {loading && (
                <div className="px-3 py-2 text-xs text-[var(--text-muted)] animate-pulse">Carregando…</div>
              )}
              {fetchErr && (
                <div className="px-3 py-2 text-xs text-red-400">{fetchErr}</div>
              )}
              {!loading && !fetchErr && items.length === 0 && (
                <div className="px-3 py-2 text-xs text-[var(--text-faint)]">
                  {emptyLabel ?? "Nenhum resultado"}
                </div>
              )}
              {!loading && items.map((item) => {
                const picked = multi ? selectedIDs.has(item.id) : item.id === props.value;
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => (multi ? toggleMulti(item) : pickSingle(item))}
                    className={`w-full text-left px-3 py-1.5 text-sm transition-colors flex items-start gap-2 ${
                      picked
                        ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                        : "text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
                    }`}
                  >
                    {multi && (
                      <span
                        className={`mt-0.5 inline-flex items-center justify-center w-4 h-4 rounded-[var(--radius-sm)] border shrink-0 ${
                          picked
                            ? "bg-[var(--accent)] border-[var(--accent)] text-white"
                            : "border-[var(--border-default)]"
                        }`}
                        aria-hidden
                      >
                        {picked && (
                          <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                          </svg>
                        )}
                      </span>
                    )}
                    <span className="flex-1 min-w-0">
                      <span className="flex items-center gap-1.5">
                        {item.color && (
                          <span
                            className="inline-block w-2 h-2 rounded-full shrink-0"
                            style={{ backgroundColor: item.color }}
                          />
                        )}
                        <span className="block truncate">{item.label}</span>
                      </span>
                      {item.secondary && (
                        <span className="block text-[11px] text-[var(--text-faint)]">{item.secondary}</span>
                      )}
                    </span>
                  </button>
                );
              })}
            </div>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
}
