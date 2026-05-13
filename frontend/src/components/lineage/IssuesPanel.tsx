"use client";

import { useMemo, useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { LineageIndexes } from "@/lib/lineage/indexes";
import type { LineageWarning } from "@/lib/lineage/types";

interface Props {
  indexes: LineageIndexes;
  filterKind: string | null;
  onNavigate: (view: "overview" | "graph" | "issues", extra?: Record<string, string | undefined>) => void;
}

const GAP_GROUPS: Array<{ key: "orphan_tasks" | "isolated_models" | "unused_sources" | "unused_macros"; labelKey: string }> = [
  { key: "orphan_tasks",    labelKey: "atlas.lineage.cards.orphanTasks" },
  { key: "isolated_models", labelKey: "atlas.lineage.cards.isolatedModels" },
  { key: "unused_sources",  labelKey: "atlas.lineage.cards.unusedSources" },
  { key: "unused_macros",   labelKey: "atlas.lineage.cards.unusedMacros" },
];

export default function IssuesPanel({ indexes, filterKind, onNavigate }: Props) {
  const { t } = useLocale();
  const [expanded, setExpanded] = useState<string | null>(filterKind);

  const warnings = useMemo(() => indexes.raw.warnings ?? [], [indexes.raw.warnings]);
  const errors = useMemo(
    () => warnings.filter(w => w.severity === "error" && (!filterKind || w.kind === filterKind)),
    [warnings, filterKind],
  );
  const otherWarnings = useMemo(
    () => warnings.filter(w => w.severity !== "error" && (!filterKind || w.kind === filterKind)),
    [warnings, filterKind],
  );

  const gaps = indexes.raw.coverage?.gaps ?? {};

  return (
    <div className="space-y-6">
      {/* Errors */}
      {errors.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-red-300 uppercase tracking-wider mb-2">
            {t("atlas.lineage.issues.errors")} ({errors.length})
          </h2>
          <div className="space-y-2">
            {errors.map((w, i) => <WarningRow key={i} w={w} onNavigate={onNavigate} indexes={indexes} severity="error" />)}
          </div>
        </section>
      )}

      {/* Other warnings */}
      {otherWarnings.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] uppercase tracking-wider mb-2">
            {t("atlas.lineage.issues.warnings")} ({otherWarnings.length})
          </h2>
          <div className="space-y-2">
            {otherWarnings.map((w, i) => <WarningRow key={i} w={w} onNavigate={onNavigate} indexes={indexes} severity="warning" />)}
          </div>
        </section>
      )}

      {/* Coverage gaps */}
      <section>
        <h2 className="text-sm font-semibold text-[var(--text-secondary)] uppercase tracking-wider mb-2">
          {t("atlas.lineage.issues.coverageGaps")}
        </h2>
        <div className="space-y-2">
          {GAP_GROUPS.map(g => {
            const ids = (gaps[g.key] ?? []) as string[];
            if (ids.length === 0) return null;
            const isOpen = expanded === g.key;
            return (
              <div key={g.key} className="bg-[var(--bg-surface)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] overflow-hidden">
                <button
                  onClick={() => setExpanded(isOpen ? null : g.key)}
                  className="w-full px-4 py-2 flex items-center justify-between text-left hover:bg-[var(--bg-elevated)] transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <svg className={`w-3 h-3 transition-transform ${isOpen ? "rotate-90" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                    </svg>
                    <span className="font-medium text-sm">{t(g.labelKey)}</span>
                    <span className="text-xs text-[var(--text-muted)]">({ids.length})</span>
                  </div>
                </button>
                {isOpen && (
                  <div className="px-4 pb-3 space-y-0.5 max-h-[400px] overflow-y-auto">
                    {ids.map(id => {
                      const n = indexes.nodesById.get(id);
                      return (
                        <button
                          key={id}
                          onClick={() => onNavigate("graph", { focus: id })}
                          className="w-full text-left text-xs font-mono text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] px-2 py-1 truncate"
                          title={id}
                        >
                          {n?.label ?? id}
                          {n?.file && <span className="text-[var(--text-faint)] ml-2">{n.file}</span>}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}

function WarningRow({ w, onNavigate, indexes, severity }: {
  w: LineageWarning;
  onNavigate: (view: "overview" | "graph" | "issues", extra?: Record<string, string | undefined>) => void;
  indexes: LineageIndexes;
  severity: "error" | "warning";
}) {
  const node = w.source_id ? indexes.nodesById.get(w.source_id) : null;
  const cls = severity === "error"
    ? "bg-red-500/10 border-red-500/30"
    : "bg-amber-500/10 border-amber-500/30";

  return (
    <div className={`p-3 rounded-[var(--radius-md)] border ${cls}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="text-[10px] font-mono uppercase tracking-wider text-[var(--text-muted)]">{w.kind}</span>
            {w.file && <span className="text-[10px] font-mono text-[var(--text-faint)]">{w.file}{w.line ? `:${w.line}` : ""}</span>}
          </div>
          <p className="text-xs text-[var(--text-primary)]">{w.message}</p>
        </div>
        {w.source_id && (
          <button
            onClick={() => onNavigate("graph", { focus: w.source_id })}
            className="shrink-0 text-[10px] px-2 py-1 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-surface)] transition-colors"
          >
            {node?.label ?? "open"} →
          </button>
        )}
      </div>
    </div>
  );
}
