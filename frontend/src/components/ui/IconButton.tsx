import { type ButtonHTMLAttributes } from "react";

const variants = {
  default: "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)] border-transparent",
  outline: "text-[var(--text-muted)] border-[var(--border-default)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)]",
  accent: "bg-[var(--accent)] text-white border-transparent hover:brightness-110",
  danger: "text-red-400 hover:bg-red-500/10 hover:text-red-300 border-transparent",
  active: "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20",
};

const sizes = {
  sm: "w-8 h-8",
  md: "w-9 h-9",
};

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof variants;
  size?: keyof typeof sizes;
}

export default function IconButton({
  variant = "default",
  size = "sm",
  className = "",
  children,
  ...props
}: IconButtonProps) {
  return (
    <button
      className={`flex items-center justify-center rounded-[var(--radius-md)] border transition-colors ${variants[variant]} ${sizes[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}
