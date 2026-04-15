export interface Tab {
  key: string;
  label: string;
  icon?: string;
  badge?: number;
}

export default function TabBar({
  tabs,
  activeTab,
  onChange,
  className = "",
}: {
  tabs: Tab[];
  activeTab: string;
  onChange: (key: string) => void;
  className?: string;
}) {
  return (
    <div
      className={`flex gap-1 p-1 rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] w-full md:w-fit overflow-x-auto ${className}`}
    >
      {tabs.map((tab) => (
        <button
          key={tab.key}
          onClick={() => onChange(tab.key)}
          className={`flex-1 md:flex-none flex items-center justify-center gap-1.5 px-3 md:px-4 py-1.5 text-xs md:text-sm font-medium rounded-[var(--radius-sm)] transition-all duration-150 whitespace-nowrap ${
            activeTab === tab.key
              ? "bg-[var(--accent-muted)] text-[var(--accent)]"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
          }`}
        >
          {tab.icon && (
            <svg className="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d={tab.icon} />
            </svg>
          )}
          <span className="hidden sm:inline">{tab.label}</span>
          {tab.badge !== undefined && tab.badge > 0 && (
            <span className="w-4 h-4 rounded-full bg-amber-500/20 text-amber-400 text-[10px] font-bold flex items-center justify-center shrink-0">
              {tab.badge}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}
