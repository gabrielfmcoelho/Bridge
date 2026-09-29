"use client";

import { useId, useMemo, useRef, useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import FormField, { INPUT_CLASS } from "@/components/ui/FormField";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export interface RelationOption {
  id: number;
  label: string;
  /** Context that tells same-named items apart (the host a service runs on…). */
  secondary?: string;
  mono?: boolean;
}

const MAX_RESULTS = 8;

/**
 * Links to other assets: the chosen ones as removable chips, then a search
 * box (combobox) over the options. Built for hundreds of options — the old
 * checkbox cloud listed all 535 services at once. Arrow keys move, Enter picks,
 * Escape closes; Enter never submits the surrounding form.
 */
export default function RelationPicker({ label, options, selected, onChange, placeholder }: {
  label: string;
  options: RelationOption[];
  selected: number[];
  onChange: (ids: number[]) => void;
  placeholder?: string;
}) {
  const { t } = useLocale();
  const id = useId();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const byId = useMemo(() => new Map(options.map((o) => [o.id, o])), [options]);
  const chosen = selected.map((sid) => byId.get(sid) ?? { id: sid, label: `#${sid}` });
  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    const pool = options.filter((o) => !selected.includes(o.id));
    const hits = q ? pool.filter((o) => o.label.toLowerCase().includes(q) || o.secondary?.toLowerCase().includes(q)) : pool;
    return hits.slice(0, MAX_RESULTS);
  }, [options, selected, query]);

  const pick = (o: RelationOption) => {
    onChange([...selected, o.id]);
    setQuery("");
    setActive(0);
    inputRef.current?.focus();
  };
  const listId = `${id}-list`;
  const showList = open && results.length > 0;

  return (
    <FormField label={label} htmlFor={id} hint={selected.length ? t("form.relationsCount", { count: String(selected.length) }) : undefined}>
      {chosen.length > 0 && (
        <ul className="flex flex-wrap gap-1.5 mb-2">
          {chosen.map((o) => (
            <li key={o.id} className="inline-flex max-w-full items-center gap-1 rounded-full border border-[var(--border-default)] bg-[var(--bg-overlay)] pl-2.5 pr-1 py-0.5 text-xs">
              <span className={`truncate text-[var(--text-secondary)] font-medium ${o.mono ? "font-mono" : ""}`} title={o.label}>{o.label}</span>
              {o.secondary && <span className="truncate text-[var(--text-muted)] max-w-[10rem]" title={o.secondary}>{o.secondary}</span>}
              <button
                type="button"
                onClick={() => onChange(selected.filter((s) => s !== o.id))}
                aria-label={t("common.removeItem", { label: o.label })}
                className="rounded-full p-0.5 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
              >
                <Icon path={ICON_PATHS.close} className="w-3 h-3" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="relative">
        <Icon path={ICON_PATHS.search} className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-faint)] pointer-events-none" />
        <input
          ref={inputRef}
          id={id}
          role="combobox"
          aria-expanded={showList}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={showList ? `${id}-opt-${results[active]?.id}` : undefined}
          value={query}
          placeholder={placeholder ?? t("form.searchToLink")}
          onChange={(e) => { setQuery(e.target.value); setOpen(true); setActive(0); }}
          onFocus={() => setOpen(true)}
          onBlur={() => setTimeout(() => setOpen(false), 120)}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); setOpen(true); setActive((a) => Math.min(a + 1, results.length - 1)); }
            else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
            else if (e.key === "Enter") { e.preventDefault(); if (showList && results[active]) pick(results[active]); }
            else if (e.key === "Escape" && open) { e.stopPropagation(); setOpen(false); }
          }}
          className={`${INPUT_CLASS} pl-9`}
        />
        {showList && (
          <ul
            id={listId}
            role="listbox"
            className="absolute z-20 mt-1 w-full max-h-72 overflow-y-auto rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-surface)] py-1 shadow-lg"
          >
            {results.map((o, i) => (
              <li
                key={o.id}
                id={`${id}-opt-${o.id}`}
                role="option"
                aria-selected={i === active}
                onMouseDown={(e) => e.preventDefault()}
                onMouseEnter={() => setActive(i)}
                onClick={() => pick(o)}
                className={`flex items-baseline gap-2 px-3 py-1.5 text-sm cursor-pointer ${i === active ? "bg-[var(--bg-elevated)]" : ""}`}
              >
                <span className={`truncate text-[var(--text-primary)] ${o.mono ? "font-mono" : ""}`}>{o.label}</span>
                {o.secondary && <span className="truncate text-xs text-[var(--text-muted)]">{o.secondary}</span>}
              </li>
            ))}
          </ul>
        )}
      </div>
    </FormField>
  );
}
