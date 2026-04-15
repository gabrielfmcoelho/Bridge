interface StepIndicatorProps {
  steps: string[];
  current: number;
}

export default function StepIndicator({ steps, current }: StepIndicatorProps) {
  // Show at most 2 steps: previous+current or current+next
  const currentIdx = current - 1;
  let startIdx: number;
  if (currentIdx === 0) {
    startIdx = 0;
  } else if (currentIdx >= steps.length - 1) {
    startIdx = Math.max(0, steps.length - 2);
  } else {
    startIdx = currentIdx - 1;
  }
  const visible = steps.slice(startIdx, startIdx + 2);
  const visibleStart = startIdx;

  return (
    <div className="flex items-center gap-2">
      {visible.map((label, vi) => {
        const stepNum = visibleStart + vi + 1;
        const isLast = vi === visible.length - 1;
        return (
          <div key={stepNum} className="flex items-center gap-2 flex-1 min-w-0">
            <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-semibold shrink-0 transition-colors ${
              current === stepNum ? "bg-[var(--accent)] text-white" : current > stepNum ? "bg-emerald-500/20 text-emerald-400" : "bg-[var(--bg-elevated)] text-[var(--text-faint)]"
            }`}>
              {current > stepNum ? "✓" : stepNum}
            </div>
            <span className={`text-xs font-medium leading-tight min-w-0 break-words ${current === stepNum ? "text-[var(--text-primary)]" : "text-[var(--text-faint)]"}`}>
              {label}
            </span>
            {!isLast && <div className={`flex-1 h-px min-w-4 ${current > stepNum ? "bg-emerald-500/40" : "bg-[var(--border-subtle)]"}`} />}
          </div>
        );
      })}
      {/* Step counter */}
      <span className="text-[10px] text-[var(--text-faint)] shrink-0 ml-auto" style={{ fontFamily: "var(--font-mono)" }}>
        {current}/{steps.length}
      </span>
    </div>
  );
}
