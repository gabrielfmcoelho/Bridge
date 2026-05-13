import Button from "@/components/ui/Button";
import type { Host } from "@/lib/types";
import type { BatchProgress } from "./useBatchRunner";

export interface ScopeOption {
  key: string;
  label: string;
  count: number;
}

export default function BatchOperationShell({
  description,
  scopeLabel,
  scope,
  onScopeChange,
  scopeOptions,
  concurrency,
  onConcurrencyChange,
  concurrencyLabel,
  targetHosts,
  progress,
  running,
  doneCount,
  successCount,
  failedCount,
  startLabel,
  rerunLabel,
  stopLabel,
  cancelLabel,
  progressLabel,
  runningLabel,
  emptyHint,
  onStart,
  onStop,
  onClose,
}: {
  description: string;
  scopeLabel: string;
  scope: string;
  onScopeChange: (scope: string) => void;
  scopeOptions: ScopeOption[];
  concurrency: number;
  onConcurrencyChange: (value: number) => void;
  concurrencyLabel: string;
  targetHosts: Host[];
  progress: BatchProgress;
  running: boolean;
  doneCount: number;
  successCount: number;
  failedCount: number;
  startLabel: string;
  rerunLabel: string;
  stopLabel: string;
  cancelLabel: string;
  progressLabel: string;
  runningLabel: string;
  emptyHint?: string;
  onStart: () => void;
  onStop: () => void;
  onClose: () => void;
}) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-[var(--text-secondary)]">{description} ({targetHosts.length})</p>

      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">{scopeLabel}:</span>
        <div className="inline-flex rounded-[var(--radius-md)] border border-[var(--border-subtle)] overflow-hidden flex-wrap">
          {scopeOptions.map((opt, i) => (
            <button
              key={opt.key}
              type="button"
              disabled={running || (opt.count === 0 && opt.key !== scope)}
              onClick={() => onScopeChange(opt.key)}
              className={`px-3 py-1.5 text-xs transition-colors ${i > 0 ? "border-l border-[var(--border-subtle)]" : ""} ${scope === opt.key ? "bg-[var(--accent-muted)] text-[var(--accent)]" : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"} disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:text-[var(--text-muted)]`}
            >
              {opt.label} ({opt.count})
            </button>
          ))}
        </div>
      </div>

      <div className="flex items-center gap-3">
        <label className="text-xs text-[var(--text-muted)] whitespace-nowrap">{concurrencyLabel}</label>
        <input
          type="range"
          min={1}
          max={20}
          step={1}
          value={concurrency}
          disabled={running}
          onChange={(e) => onConcurrencyChange(Number(e.target.value))}
          className="flex-1 accent-[var(--accent)] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
        />
        <span className="text-xs font-medium text-[var(--text-primary)] w-6 text-right" style={{ fontFamily: "var(--font-mono)" }}>
          {concurrency}
        </span>
      </div>

      {targetHosts.length === 0 && emptyHint && (
        <p className="text-xs text-amber-400 bg-amber-500/10 border border-amber-500/25 rounded-[var(--radius-md)] px-3 py-2">
          {emptyHint}
        </p>
      )}

      {running && (
        <div>
          <div className="flex items-center justify-between text-xs text-[var(--text-muted)] mb-1.5">
            <span>{progressLabel}</span>
            <span style={{ fontFamily: "var(--font-mono)" }}>{doneCount}/{targetHosts.length}</span>
          </div>
          <div className="h-2 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
            <div
              className="h-full rounded-full bg-[var(--accent)] transition-all duration-300"
              style={{ width: `${targetHosts.length ? (doneCount / targetHosts.length) * 100 : 0}%` }}
            />
          </div>
          {(successCount > 0 || failedCount > 0) && (
            <div className="flex gap-3 mt-1.5 text-xs">
              <span className="text-emerald-400">{successCount} OK</span>
              {failedCount > 0 && <span className="text-red-400">{failedCount} failed</span>}
            </div>
          )}
        </div>
      )}

      {Object.keys(progress).length > 0 && (
        <div className="max-h-64 overflow-y-auto space-y-1 border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-2">
          {targetHosts.map(host => {
            const s = progress[host.oficial_slug];
            if (!s) return null;
            return (
              <div key={host.oficial_slug} className="flex items-center gap-2 py-1.5 px-2 rounded text-xs">
                {s.status === "pending" && <span className="w-2 h-2 rounded-full bg-[var(--text-faint)]" />}
                {s.status === "running" && <span className="w-2 h-2 rounded-full bg-[var(--accent)] animate-pulse" />}
                {s.status === "success" && <span className="w-2 h-2 rounded-full bg-emerald-400" />}
                {s.status === "failed" && <span className="w-2 h-2 rounded-full bg-red-400" />}
                <span className="text-[var(--text-primary)] font-medium" style={{ fontFamily: "var(--font-mono)" }}>{host.nickname}</span>
                {s.status === "running" && <span className="text-[var(--accent)] ml-auto">{runningLabel}{s.attempt && s.attempt > 1 ? ` (${s.attempt})` : ""}</span>}
                {s.status === "success" && <span className="text-emerald-400 ml-auto">OK</span>}
                {s.error && <span className="text-red-400 ml-auto truncate max-w-[200px]" title={s.error}>{s.error}</span>}
              </div>
            );
          })}
        </div>
      )}

      <div className="flex gap-2 justify-end">
        {running ? (
          <Button variant="danger" size="sm" onClick={onStop}>{stopLabel}</Button>
        ) : (
          <>
            <Button variant="secondary" size="sm" onClick={onClose}>{cancelLabel}</Button>
            <Button size="sm" onClick={onStart} disabled={targetHosts.length === 0}>
              {doneCount > 0 ? rerunLabel : startLabel}
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
