import { type ButtonHTMLAttributes } from "react";
import Spinner from "./Spinner";

const variants = {
  primary:
    "bg-[var(--accent)] hover:brightness-110 text-white border-[var(--accent)]/50",
  secondary:
    "bg-[var(--bg-elevated)] hover:bg-[var(--bg-overlay)] text-[var(--text-secondary)] border-[var(--border-default)] hover:text-[var(--text-primary)] hover:border-[var(--border-strong)]",
  // --danger is theme-tuned in globals.css, so no light/dark branch here.
  danger:
    "bg-[var(--danger)]/10 hover:bg-[var(--danger)]/20 text-[var(--danger)] border-[var(--danger)]/30 hover:border-[var(--danger)]/50",
  ghost:
    "bg-transparent hover:bg-[var(--bg-elevated)] text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]",
};

const sizes = {
  sm: "h-[30px] px-3 text-xs gap-1.5",
  md: "px-4 py-2 text-sm gap-2",
  lg: "px-6 py-2.5 text-base gap-2",
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof variants;
  size?: keyof typeof sizes;
  loading?: boolean;
}


export default function Button({
  variant = "primary",
  size = "md",
  loading = false,
  className = "",
  children,
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      className={`inline-flex items-center justify-center rounded-[var(--radius-md)] border font-medium transition duration-150 enabled:active:scale-[0.98] disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:transform-none ${variants[variant]} ${sizes[size]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading && <Spinner size={size === "lg" ? "md" : size === "sm" ? "xs" : "sm"} />}
      {children}
    </button>
  );
}
