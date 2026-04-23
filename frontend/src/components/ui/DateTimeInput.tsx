"use client";

import type { InputHTMLAttributes } from "react";

interface Props extends Omit<InputHTMLAttributes<HTMLInputElement>, "type" | "onChange" | "value"> {
  label?: string;
  error?: string;
  variant: "date" | "datetime" | "time";
  value: string;
  onChange: (value: string) => void;
}

// Thin wrapper over the native date/time inputs. GLPI expects:
//   date     → YYYY-MM-DD
//   datetime → YYYY-MM-DD HH:MM:SS
//   time     → HH:MM:SS
// The browser emits YYYY-MM-DDTHH:MM (for datetime-local) and HH:MM (for time);
// we normalize both on change so the stored value is ready for the GLPI payload.
export default function DateTimeInput({
  label,
  error,
  variant,
  value,
  onChange,
  className,
  disabled,
  ...rest
}: Props) {
  const inputType = variant === "datetime" ? "datetime-local" : variant;

  const fromGLPI = (v: string): string => {
    if (!v) return "";
    if (variant === "datetime") return v.replace(" ", "T").slice(0, 16);
    if (variant === "time") return v.slice(0, 5);
    return v;
  };

  const toGLPI = (v: string): string => {
    if (!v) return "";
    if (variant === "datetime") return v.replace("T", " ") + ":00";
    if (variant === "time") return v.length === 5 ? v + ":00" : v;
    return v;
  };

  return (
    <div className="space-y-1.5">
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <input
        type={inputType}
        value={fromGLPI(value)}
        onChange={(e) => onChange(toGLPI(e.target.value))}
        disabled={disabled}
        className={`w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border rounded-[var(--radius-md)] px-3 py-2 text-sm transition-all duration-200 disabled:opacity-40 focus:outline-none focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] ${
          error ? "border-red-500" : "border-[var(--border-default)]"
        } ${className ?? ""}`}
        {...rest}
      />
      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
}
