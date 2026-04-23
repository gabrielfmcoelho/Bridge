"use client";

interface ToggleProps {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  ariaLabel?: string;
  className?: string;
}

export default function Toggle({ checked, onChange, disabled, ariaLabel, className = "" }: ToggleProps) {
  return (
    <label
      className={`relative inline-flex items-center ${disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer"} ${className}`}
      onClick={(e) => e.stopPropagation()}
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        aria-label={ariaLabel}
        className="sr-only peer"
      />
      <div
        className="relative w-9 h-5 bg-[var(--bg-overlay)] rounded-full
                   peer-checked:bg-[var(--accent)] peer-focus:outline-none
                   transition-colors
                   after:content-[''] after:absolute after:top-[2px] after:left-[2px]
                   after:bg-white after:rounded-full after:h-4 after:w-4
                   after:transition-transform
                   peer-checked:after:translate-x-4"
      />
    </label>
  );
}
