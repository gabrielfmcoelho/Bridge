"use client";

import { useMemo } from "react";
import type { HostEntidade } from "@/lib/types";
import Select from "@/components/ui/Select";
import IconButton from "@/components/ui/IconButton";
import EmptyState from "@/components/ui/EmptyState";

interface EntidadeListProps {
  value: HostEntidade[];
  onChange: (v: HostEntidade[]) => void;
  options: { value: string }[];
  t: (k: string) => string;
}

export default function EntidadeList({ value, onChange, options, t }: EntidadeListProps) {
  const linked = useMemo(() => new Set(value.map((v) => v.entidade)), [value]);
  const available = options.filter((o) => !linked.has(o.value));

  const setMain = (idx: number) => {
    onChange(value.map((item, i) => ({ ...item, is_main: i === idx })));
  };

  const remove = (idx: number) => {
    const next = value.filter((_, i) => i !== idx);
    if (next.length > 0 && !next.some((e) => e.is_main)) {
      next[0] = { ...next[0], is_main: true };
    }
    onChange(next);
  };

  const add = (entidade: string) => {
    if (!entidade) return;
    const isMain = value.length === 0;
    onChange([...value, { entidade, is_main: isMain }]);
  };

  if (options.length === 0) {
    return (
      <EmptyState
        icon="folder"
        title={t("entidade.noOptions")}
        compact
      />
    );
  }

  return (
    <div className="space-y-2">
      {value.map((item, idx) => (
        <div
          key={item.entidade}
          className="flex items-center gap-3 px-3 py-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)]"
        >
          <span className="text-sm text-[var(--text-primary)] flex-1 min-w-0 truncate">{item.entidade}</span>
          <button
            type="button"
            onClick={() => setMain(idx)}
            className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
              item.is_main
                ? "bg-[var(--accent)] text-white"
                : "bg-[var(--bg-surface)] text-[var(--text-muted)] hover:text-[var(--text-secondary)] border border-[var(--border-default)]"
            }`}
          >
            {item.is_main && (
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
            )}
            {t("responsavel.main")}
          </button>
          <IconButton
            variant="danger"
            size="sm"
            type="button"
            onClick={() => remove(idx)}
            title={t("common.remove")}
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </IconButton>
        </div>
      ))}

      {available.length > 0 ? (
        <Select
          value=""
          options={[
            { value: "", label: t("entidade.select") },
            ...available.map((o) => ({ value: o.value, label: o.value })),
          ]}
          onChange={(e) => add(e.target.value)}
        />
      ) : (
        value.length > 0 && (
          <p className="text-xs text-[var(--text-faint)]">{t("entidade.allLinked")}</p>
        )
      )}
    </div>
  );
}
