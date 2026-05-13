"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { LineageIndexes } from "@/lib/lineage/indexes";
import { NODE_TYPE_LABELS, NODE_COLORS } from "@/lib/lineage/style";

interface Props {
  open: boolean;
  indexes: LineageIndexes;
  onClose: () => void;
  onPick: (id: string) => void;
}

export default function SearchOmnibar({ open, indexes, onClose, onPick }: Props) {
  const { t } = useLocale();
  const [q, setQ] = useState("");
  const [activeIdx, setActiveIdx] = useState(0);
  const [prevOpen, setPrevOpen] = useState(open);
  const inputRef = useRef<HTMLInputElement>(null);

  // Reset state when the dialog transitions to open (React's recommended
  // "adjust state during render based on a prop change" pattern).
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setQ("");
      setActiveIdx(0);
    }
  }

  useEffect(() => {
    if (open) {
      const id = setTimeout(() => inputRef.current?.focus(), 50);
      return () => clearTimeout(id);
    }
  }, [open]);

  const results = useMemo(() => {
    const query = q.trim().toLowerCase();
    if (!query) return indexes.raw.nodes.slice(0, 30);
    return indexes.raw.nodes
      .map(n => {
        const lbl = n.label.toLowerCase();
        const id = n.id.toLowerCase();
        let score = 0;
        if (lbl.startsWith(query)) score = 100;
        else if (lbl.includes(query)) score = 50;
        else if (id.includes(query)) score = 25;
        return { n, score };
      })
      .filter(r => r.score > 0)
      .sort((a, b) => b.score - a.score)
      .slice(0, 30)
      .map(r => r.n);
  }, [q, indexes.raw.nodes]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[60] flex items-start justify-center pt-[15vh] bg-black/60" onClick={onClose}>
      <div className="w-full max-w-xl bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-lg)] shadow-[var(--shadow-lg)] overflow-hidden" onClick={e => e.stopPropagation()}>
        <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--border-subtle)]">
          <svg className="w-4 h-4 text-[var(--text-muted)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            ref={inputRef}
            value={q}
            onChange={e => { setQ(e.target.value); setActiveIdx(0); }}
            onKeyDown={e => {
              if (e.key === "ArrowDown") { e.preventDefault(); setActiveIdx(i => Math.min(i + 1, results.length - 1)); }
              else if (e.key === "ArrowUp") { e.preventDefault(); setActiveIdx(i => Math.max(i - 1, 0)); }
              else if (e.key === "Enter") { if (results[activeIdx]) onPick(results[activeIdx].id); }
              else if (e.key === "Escape") onClose();
            }}
            placeholder={t("atlas.lineage.search.placeholder")}
            className="flex-1 bg-transparent text-sm text-[var(--text-primary)] focus:outline-none"
          />
          <kbd className="text-[9px] px-1 py-0.5 rounded bg-[var(--bg-elevated)] text-[var(--text-faint)]">esc</kbd>
        </div>
        <div className="max-h-[50vh] overflow-y-auto">
          {results.length === 0 ? (
            <div className="p-4 text-xs text-[var(--text-muted)]">{t("common.noResults")}</div>
          ) : (
            results.map((n, i) => {
              const c = NODE_COLORS[n.type] ?? NODE_COLORS.table;
              return (
                <button
                  key={n.id}
                  onClick={() => onPick(n.id)}
                  onMouseEnter={() => setActiveIdx(i)}
                  className={`w-full text-left px-3 py-2 flex items-center gap-2 text-xs ${i === activeIdx ? "bg-[var(--bg-elevated)]" : ""}`}
                >
                  <span className={`text-[9px] px-1.5 py-0.5 rounded ${c.bg} ${c.text} font-semibold uppercase tracking-wider w-20 text-center shrink-0`}>
                    {NODE_TYPE_LABELS[n.type] ?? n.type}
                  </span>
                  <span className="font-mono text-[var(--text-primary)] truncate flex-1">{n.label}</span>
                  {n.layer && <span className="text-[9px] text-[var(--text-muted)] shrink-0">{n.layer}</span>}
                  {n.namespace && <span className="text-[9px] text-[var(--text-muted)] shrink-0">{n.namespace}</span>}
                </button>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
