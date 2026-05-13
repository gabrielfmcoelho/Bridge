"use client";

import { useMemo } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { AtlasIndexes } from "@/lib/atlas/types";
import { buildColumnLineagePath, type ColumnLineageStep } from "@/lib/atlas/columnLineagePath";

interface Props {
  indexes: AtlasIndexes;
  columnId: string;
  onBack: () => void;
  onSelectColumn: (id: string) => void;
}

export default function ColumnLineagePanel({ indexes, columnId, onBack, onSelectColumn }: Props) {
  const { t } = useLocale();
  const column = indexes.nodesById.get(columnId);
  const table = column?.parent ? indexes.nodesById.get(column.parent) : null;
  const tableRec = table ? indexes.tablesById.get(table.id) : null;

  const upstream = useMemo(() => buildColumnLineagePath(indexes, columnId, "upstream"), [indexes, columnId]);
  const downstream = useMemo(() => buildColumnLineagePath(indexes, columnId, "downstream"), [indexes, columnId]);

  if (!column) {
    return <div className="text-sm text-[var(--text-muted)]">Column not found.</div>;
  }

  const dtype = (column.data as Record<string, unknown> | undefined)?.data_type;
  const tests = (column.data as Record<string, unknown> | undefined)?.tests;
  const testCount = Array.isArray(tests) ? tests.length : 0;
  const inferred = Boolean((column.data as Record<string, unknown> | undefined)?.inferred_from_sql);

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-col gap-3 -mt-1">
        <button
          onClick={onBack}
          className="inline-flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors w-fit"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          {t("atlas.catalog.column.backToTable")}{table ? `: ${table.label}` : ""}
        </button>

        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)] font-semibold">
              {t("atlas.catalog.column.header")}
            </span>
            {tableRec && (
              <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--text-faint)]">
                {tableRec.namespace} · {tableRec.layer}
              </span>
            )}
          </div>
          <h2 className="font-mono text-lg text-[var(--text-primary)] break-all" style={{ fontFamily: "var(--font-display)" }}>
            {column.label}
          </h2>
          <div className="flex items-center gap-3 text-[11px] text-[var(--text-muted)]">
            {Boolean(dtype) && (
              <span>
                <span className="text-[var(--text-faint)] uppercase tracking-wider mr-1">{t("atlas.catalog.column.dtype")}:</span>
                <span className="font-mono text-[var(--text-secondary)]">{String(dtype)}</span>
              </span>
            )}
            {testCount > 0 && (
              <span>
                <span className="text-[var(--text-faint)] uppercase tracking-wider mr-1">{t("atlas.catalog.column.tests")}:</span>
                <span className="tabular-nums">{testCount}</span>
              </span>
            )}
            {inferred && (
              <span className="text-[10px] uppercase tracking-wider text-amber-400/80">
                {t("atlas.catalog.column.inferred")}
              </span>
            )}
          </div>
        </div>
      </header>

      <Trail
        title={t("atlas.catalog.column.upstreamTrail")}
        emptyText={t("atlas.catalog.column.noUpstream")}
        steps={upstream}
        direction="upstream"
        indexes={indexes}
        onSelectColumn={onSelectColumn}
      />

      <Trail
        title={t("atlas.catalog.column.downstreamTrail")}
        emptyText={t("atlas.catalog.column.noDownstream")}
        steps={downstream}
        direction="downstream"
        indexes={indexes}
        onSelectColumn={onSelectColumn}
      />
    </div>
  );
}

function Trail({
  title,
  emptyText,
  steps,
  direction,
  indexes,
  onSelectColumn,
}: {
  title: string;
  emptyText: string;
  steps: ColumnLineageStep[];
  direction: "upstream" | "downstream";
  indexes: AtlasIndexes;
  onSelectColumn: (id: string) => void;
}) {
  return (
    <section className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)] font-semibold">{title}</span>
        {steps.length > 0 && (
          <span className="text-[10px] tabular-nums text-[var(--text-muted)]">{steps.length}</span>
        )}
      </div>
      {steps.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)] italic px-1">{emptyText}</p>
      ) : (
        <ol className="relative flex flex-col gap-2 pl-6">
          {/* Timeline rail */}
          <span
            className={`absolute left-[10px] top-2 bottom-2 w-px ${
              direction === "upstream" ? "bg-gradient-to-b from-[var(--accent)]/40 to-transparent" : "bg-gradient-to-b from-transparent to-[var(--accent)]/40"
            }`}
          />
          {steps.map(step => (
            <TrailStep
              key={`${step.column.id}-${step.depth}`}
              step={step}
              indexes={indexes}
              onSelectColumn={onSelectColumn}
            />
          ))}
        </ol>
      )}
    </section>
  );
}

function TrailStep({
  step,
  indexes,
  onSelectColumn,
}: {
  step: ColumnLineageStep;
  indexes: AtlasIndexes;
  onSelectColumn: (id: string) => void;
}) {
  const { t } = useLocale();
  const dotColor = step.confidence === "exact" ? "bg-emerald-400"
                  : step.confidence === "inferred" ? "bg-amber-400"
                  : step.confidence === "dynamic" ? "bg-rose-400"
                  : "bg-[var(--border-strong)]";

  const tableRec = step.table ? indexes.tablesById.get(step.table.id) : null;

  return (
    <li className="relative">
      {/* Timeline dot */}
      <span
        className={`absolute left-[-22px] top-2.5 w-3 h-3 rounded-full border-2 border-[var(--bg-surface)] ${dotColor}`}
        title={step.confidence ?? ""}
      />
      <button
        onClick={() => onSelectColumn(step.column.id)}
        className="group w-full text-left rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] hover:bg-[var(--bg-elevated)] hover:border-[var(--border-default)] transition-all p-2.5 flex flex-col gap-1.5"
      >
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-mono text-[12px] text-[var(--text-primary)]">{step.column.label}</span>
          {step.via && (
            <span className="text-[9px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-[var(--accent-muted)] text-[var(--accent)] font-semibold">
              {t("atlas.catalog.column.via")} {step.via}
            </span>
          )}
          <span className="ml-auto text-[10px] text-[var(--text-faint)] tabular-nums">
            {t("atlas.catalog.column.depth")} {step.depth}
          </span>
        </div>
        {(step.table || step.producer) && (
          <div className="flex items-center gap-2 text-[10px] font-mono text-[var(--text-muted)] truncate">
            {step.table && (
              <span className="flex items-center gap-1">
                {tableRec && (
                  <span className={`w-1.5 h-1.5 rounded-full ${
                    tableRec.layer === "source" ? "bg-blue-400" :
                    tableRec.layer === "bronze" ? "bg-amber-500" :
                    tableRec.layer === "silver" ? "bg-slate-300" :
                    tableRec.layer === "gold" ? "bg-yellow-400" : "bg-gray-400"
                  }`} />
                )}
                <span title={step.table.id}>{step.table.label}</span>
              </span>
            )}
            {step.producer && (
              <>
                <span className="text-[var(--text-faint)]">·</span>
                <span title={step.producer.id}>
                  <span className="text-[var(--text-faint)] mr-1">{step.producer.type.replace("_", " ")}</span>
                  {step.producer.label}
                </span>
              </>
            )}
          </div>
        )}
      </button>
    </li>
  );
}
