interface ListToolbarProps {
  search: string;
  onSearchChange: (s: string) => void;
  onFilterClick: () => void;
  activeFilterCount: number;
  searchPlaceholder?: string;
  actions?: React.ReactNode;
}

export default function ListToolbar({
  search,
  onSearchChange,
  onFilterClick,
  activeFilterCount,
  searchPlaceholder = "Search...",
  actions,
}: ListToolbarProps) {
  return (
    <div className="flex items-center gap-2 mb-5">
      {/* Search input */}
      <div className="relative flex-1 max-w-sm min-w-[200px]">
        <svg
          className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-faint)] pointer-events-none"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          placeholder={searchPlaceholder}
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-1.5 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
        />
      </div>

      {/* Filter button */}
      <button
        onClick={onFilterClick}
        className={`relative flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border transition-all ${
          activeFilterCount > 0
            ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
            : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
        }`}
      >
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
        </svg>
        <span className="hidden sm:inline">Filters</span>
        {activeFilterCount > 0 && (
          <span className="absolute -top-1.5 -right-1.5 w-4 h-4 rounded-full bg-[var(--accent)] text-[var(--bg-base)] text-[10px] font-bold flex items-center justify-center">
            {activeFilterCount}
          </span>
        )}
      </button>

      <div className="flex-1" />

      {/* Action slot */}
      {actions}
    </div>
  );
}
