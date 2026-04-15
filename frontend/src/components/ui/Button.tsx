import { type ButtonHTMLAttributes } from "react";

const variants = {
  primary:
    "bg-[var(--accent)] hover:brightness-110 text-white border-[var(--accent)]/50 shadow-[0_0_12px_var(--accent-glow)]",
  secondary:
    "bg-[var(--bg-elevated)] hover:bg-[var(--bg-overlay)] text-[var(--text-secondary)] border-[var(--border-default)] hover:text-[var(--text-primary)] hover:border-[var(--border-strong)]",
  danger:
    "bg-red-500/10 hover:bg-red-500/20 text-red-400 border-red-500/30 hover:border-red-500/50",
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

function Spinner({ size }: { size: string }) {
  const sizeClass = size === "sm" ? "w-3 h-3" : size === "lg" ? "w-5 h-5" : "w-4 h-4";
  return (
    <svg className={`${sizeClass} animate-spin`} viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" strokeOpacity="0.25" />
      <path d="M12 2a10 10 0 019.17 6" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
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
      className={`inline-flex items-center justify-center rounded-[var(--radius-md)] border font-medium transition-all duration-200 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:transform-none ${variants[variant]} ${sizes[size]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading && <Spinner size={size} />}
      {children}
    </button>
  );
}
