import Icon from "@/components/ui/Icon";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import { VIEW_ICONS } from "@/components/ui/ViewToggle";

interface ListToolbarProps {
  search: string;
  onSearchChange: (s: string) => void;
  onFilterClick: () => void;
  activeFilterCount: number;
  searchPlaceholder?: string;
  actions?: React.ReactNode;
  /** Optional element rendered absolute-right inside the search input. */
  searchAdornment?: React.ReactNode;
  /** Cards/table switch, first of the right-hand controls: it changes how this list reads. */
  viewMode?: "cards" | "table";
  onViewModeChange?: (mode: "cards" | "table") => void;
}

export default function ListToolbar({
  search,
  onSearchChange,
  onFilterClick,
  activeFilterCount,
  searchPlaceholder = "Search...",
  actions,
  searchAdornment,
  viewMode,
  onViewModeChange,
}: ListToolbarProps) {
  const { t } = useLocale();
  return (
    // Phones: search owns its row; every control (filters first) wraps to the row below.
    <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
        {/* Search input */}
        <div className="relative sm:flex-1 sm:max-w-sm">
          <Icon path={ICON_PATHS.search} className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-faint)] pointer-events-none" />
          <input
            placeholder={searchPlaceholder}
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className={`w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 ${searchAdornment ? "pr-9" : "pr-3"} h-8 text-base md:text-sm transition duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]`}
          />
          {searchAdornment && (
            <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center">
              {searchAdornment}
            </div>
          )}
        </div>

      <div className="flex items-center gap-1.5 sm:flex-1">
        {/* Filter button */}
        <button
          onClick={onFilterClick}
          className={`relative flex items-center gap-1.5 h-8 px-3 text-sm rounded-[var(--radius-md)] border transition ${
            activeFilterCount > 0
              ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
              : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
          }`}
        >
          <Icon path={ICON_PATHS.filter} />
          <span className="hidden sm:inline">{t("common.filters")}</span>
          {activeFilterCount > 0 && (
            <span className="absolute -top-1.5 -right-1.5 w-4 h-4 rounded-full bg-[var(--accent)] text-[var(--bg-base)] text-2xs font-bold flex items-center justify-center">
              {activeFilterCount}
            </span>
          )}
        </button>
        {/* Order: how the list reads (cards/table), then group, bulk actions, export. */}
        <div className="flex items-center gap-1.5 sm:ml-auto">
        {viewMode && onViewModeChange && (
          <div className="hidden sm:flex">
            <ToolbarSelect<"cards" | "table">
              name={t("inventory.view.title")}
              icon={VIEW_ICONS.cards}
              value={viewMode}
              onChange={onViewModeChange}
              options={[
                { value: "cards", label: t("common.cards"), icon: VIEW_ICONS.cards },
                { value: "table", label: t("common.table"), icon: VIEW_ICONS.table },
              ]}
            />
          </div>
        )}
        {actions}
        </div>
      </div>
    </div>
  );
}
