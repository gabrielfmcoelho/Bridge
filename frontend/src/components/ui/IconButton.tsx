import { forwardRef, type ButtonHTMLAttributes } from "react";

const variants = {
  default: "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)] border-transparent",
  outline: "text-[var(--text-muted)] border-[var(--border-default)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)]",
  accent: "bg-[var(--accent)] text-white border-transparent hover:brightness-110",
  danger: "text-[var(--danger)] hover:bg-[var(--danger)]/10 hover:brightness-110 border-transparent",
  active: "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20",
};

const sizes = {
  sm: "w-8 h-8",
  md: "w-9 h-9",
};

interface IconButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label" | "title"> {
  /** Required: an icon alone has no name. Becomes the accessible name and the tooltip. */
  label: string;
  variant?: keyof typeof variants;
  size?: keyof typeof sizes;
}

// forwardRef so it can also trigger a DropdownMenu (the "…" more menu).
const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { label, variant = "default", size = "sm", className = "", children, type = "button", ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      title={label}
      className={`flex items-center justify-center rounded-[var(--radius-md)] border transition duration-150 enabled:active:scale-[0.95] ${variants[variant]} ${sizes[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
});

export default IconButton;
