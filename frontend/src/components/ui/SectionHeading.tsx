export default function SectionHeading({
  children,
  className = "",
  actions,
}: {
  children: React.ReactNode;
  className?: string;
  actions?: React.ReactNode;
}) {
  return (
    <div className={`flex items-center justify-between mb-3 ${className}`}>
      <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider flex items-center gap-1.5">
        {children}
      </h2>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}
