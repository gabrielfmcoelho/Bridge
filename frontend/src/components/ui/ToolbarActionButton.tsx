interface ToolbarActionButtonProps {
  icon: string;
  label?: string;
  onClick: () => void;
  title?: string;
  hideLabel?: "sm" | "md";
}

export default function ToolbarActionButton({
  icon,
  label,
  onClick,
  title,
  hideLabel = "sm",
}: ToolbarActionButtonProps) {
  const hiddenClass = hideLabel === "md" ? "hidden md:inline" : "hidden sm:inline";

  return (
    <button
      onClick={onClick}
      className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] transition-all"
      title={title || label}
    >
      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d={icon} />
      </svg>
      {label && <span className={hiddenClass}>{label}</span>}
    </button>
  );
}
