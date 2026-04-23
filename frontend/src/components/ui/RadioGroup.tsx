"use client";

interface RadioOption {
  value: string;
  label: string;
}

interface Props {
  label?: string;
  error?: string;
  options: RadioOption[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  name?: string;
}

export default function RadioGroup({
  label,
  error,
  options,
  value,
  onChange,
  disabled,
  name,
}: Props) {
  return (
    <div className="space-y-1.5">
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <div className="flex flex-col gap-1.5" role="radiogroup">
        {options.map((opt) => {
          const checked = opt.value === value;
          return (
            <label
              key={opt.value}
              className={`inline-flex items-center gap-2 text-sm cursor-pointer rounded-[var(--radius-sm)] px-2 py-1 transition-colors ${
                disabled ? "opacity-60 cursor-not-allowed" : "hover:bg-[var(--bg-elevated)]"
              }`}
            >
              <input
                type="radio"
                name={name}
                value={opt.value}
                checked={checked}
                disabled={disabled}
                onChange={() => onChange(opt.value)}
                className="shrink-0 accent-[var(--accent)]"
              />
              <span className="text-[var(--text-primary)]">{opt.label}</span>
            </label>
          );
        })}
      </div>
      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
}
