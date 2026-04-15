interface CheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  className?: string;
}

export default function Checkbox({ label, checked, onChange, disabled, className = "" }: CheckboxProps) {
  return (
    <label className={`flex items-center gap-2 text-sm text-[var(--text-secondary)] cursor-pointer ${disabled ? "opacity-40 cursor-not-allowed" : ""} ${className}`}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        disabled={disabled}
        className="w-4 h-4 rounded accent-[var(--accent)]"
      />
      {label}
    </label>
  );
}
