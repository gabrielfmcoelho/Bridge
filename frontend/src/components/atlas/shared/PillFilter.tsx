"use client";

interface PillFilterProps<T extends string> {
  label: string;
  options: Array<{ value: T; label: string; count?: number }>;
  /** Selected values (empty array = all selected — the "all" pill is active). */
  value: T[];
  onChange: (next: T[]) => void;
  /** Optional: render a leading visual (e.g. layer color dot) next to each option. */
  renderLead?: (value: T) => React.ReactNode;
  /** Hide the label header. */
  hideLabel?: boolean;
}

/**
 * Multi-select pill filter — clicking a pill toggles its inclusion. The "All"
 * pill is active when the selection is empty (= no filter applied).
 */
export default function PillFilter<T extends string>({
  label,
  options,
  value,
  onChange,
  renderLead,
  hideLabel,
}: PillFilterProps<T>) {
  const isAll = value.length === 0;

  function toggle(v: T) {
    const set = new Set(value);
    if (set.has(v)) set.delete(v);
    else set.add(v);
    onChange(Array.from(set));
  }

  return (
    <div className="flex items-center gap-1.5 flex-wrap">
      {!hideLabel && (
        <span className="text-[10px] uppercase tracking-[0.12em] text-[var(--text-faint)] font-semibold mr-1">
          {label}
        </span>
      )}
      <button
        type="button"
        onClick={() => onChange([])}
        className={`text-[11px] px-2.5 py-1 rounded-full border font-medium transition-all ${
          isAll
            ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/30"
            : "bg-transparent text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] hover:border-[var(--border-strong)]"
        }`}
      >
        All
      </button>
      {options.map(opt => {
        const active = value.includes(opt.value);
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => toggle(opt.value)}
            className={`inline-flex items-center gap-1.5 text-[11px] px-2.5 py-1 rounded-full border font-medium transition-all ${
              active
                ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/30"
                : "bg-transparent text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] hover:border-[var(--border-strong)]"
            }`}
          >
            {renderLead?.(opt.value)}
            <span>{opt.label}</span>
            {opt.count !== undefined && (
              <span className={`text-[9px] tabular-nums ${active ? "opacity-80" : "text-[var(--text-faint)]"}`}>
                {opt.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
