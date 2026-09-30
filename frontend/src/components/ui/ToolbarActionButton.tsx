import { forwardRef, type ButtonHTMLAttributes } from "react";
import Icon from "./Icon";

interface ToolbarActionButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> {
  icon: string;
  label?: string;
  hideLabel?: "sm" | "md";
  /** A toggle that's on (a quick filter): accent look, aria-pressed. */
  active?: boolean;
}

// forwardRef + prop spread so it can also be a DropdownMenu trigger (Radix
// asChild hands it a ref and its own handlers).
const ToolbarActionButton = forwardRef<HTMLButtonElement, ToolbarActionButtonProps>(function ToolbarActionButton(
  { icon, label, title, hideLabel = "sm", active, className = "", ...props },
  ref,
) {
  const hiddenClass = hideLabel === "md" ? "hidden md:inline" : "hidden sm:inline";

  return (
    <button
      ref={ref}
      type="button"
      aria-pressed={active}
      className={`flex items-center gap-1.5 h-8 px-3 text-sm rounded-[var(--radius-md)] border transition disabled:opacity-40 disabled:cursor-not-allowed ${
        active
          ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
          : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
      } ${className}`}
      title={title || label}
      // The label is display:none below the breakpoint, so it is not in the
      // accessible name there — and a touch user gets no tooltip either.
      aria-label={label || title}
      {...props}
    >
      <Icon path={icon} />
      {label && <span className={hiddenClass}>{label}</span>}
    </button>
  );
});

export default ToolbarActionButton;
