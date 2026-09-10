import type { ReactNode } from "react";

// Label + control + hint + error, the one field anatomy. Input, Textarea,
// Select and NativeSelect render through it; use it directly around any
// control that has no `label` prop of its own.
export const INPUT_CLASS =
  "w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2.5 md:py-2 text-base md:text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)] disabled:opacity-40";
export const INPUT_ERROR_CLASS = "border-[var(--danger)] focus:border-[var(--danger)] focus:ring-[var(--danger)]/20";

interface FormFieldProps {
  label?: ReactNode;
  required?: boolean;
  hint?: ReactNode;
  error?: string;
  htmlFor?: string;
  className?: string;
  children: ReactNode;
}

export default function FormField({ label, required, hint, error, htmlFor, className = "", children }: FormFieldProps) {
  return (
    <div className={`space-y-1.5 ${className}`}>
      {label && (
        <label htmlFor={htmlFor} className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
          {required && (
            <span className="text-[var(--danger)] ml-0.5" aria-hidden>
              *
            </span>
          )}
        </label>
      )}
      {children}
      {hint && <p className="text-xs text-[var(--text-muted)]">{hint}</p>}
      {error && <p className="text-xs text-[var(--danger)]">{error}</p>}
    </div>
  );
}
