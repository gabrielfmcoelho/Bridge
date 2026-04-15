export default function PillButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`inline-flex items-center px-3 py-1.5 text-xs rounded-[var(--radius-md)] border transition-all duration-150 font-medium whitespace-nowrap ${
        active
          ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
          : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:bg-[var(--bg-overlay)] hover:text-[var(--text-secondary)]"
      }`}
    >
      {children}
    </button>
  );
}
