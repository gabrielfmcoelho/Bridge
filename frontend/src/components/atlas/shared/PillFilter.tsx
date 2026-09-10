"use client";

import PillButton from "@/components/ui/PillButton";

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
        <span className="text-2xs uppercase tracking-[0.12em] text-[var(--text-faint)] font-semibold mr-1">
          {label}
        </span>
      )}
      <PillButton shape="pill" size="sm" active={isAll} onClick={() => onChange([])}>
        All
      </PillButton>
      {options.map(opt => {
        const active = value.includes(opt.value);
        return (
          <PillButton
            key={opt.value}
            shape="pill"
            size="sm"
            active={active}
            count={opt.count}
            lead={renderLead?.(opt.value)}
            onClick={() => toggle(opt.value)}
          >
            {opt.label}
          </PillButton>
        );
      })}
    </div>
  );
}
