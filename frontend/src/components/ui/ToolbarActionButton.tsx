import Icon from "./Icon";

interface ToolbarActionButtonProps {
  icon: string;
  label?: string;
  onClick: () => void;
  title?: string;
  hideLabel?: "sm" | "md";
  disabled?: boolean;
}

export default function ToolbarActionButton({
  icon,
  label,
  onClick,
  title,
  hideLabel = "sm",
  disabled,
}: ToolbarActionButtonProps) {
  const hiddenClass = hideLabel === "md" ? "hidden md:inline" : "hidden sm:inline";

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] transition disabled:opacity-40 disabled:cursor-not-allowed"
      title={title || label}
      // The label is display:none below the breakpoint, so it is not in the
      // accessible name there — and a touch user gets no tooltip either.
      aria-label={label || title}
    >
      <Icon path={icon} />
      {label && <span className={hiddenClass}>{label}</span>}
    </button>
  );
}
