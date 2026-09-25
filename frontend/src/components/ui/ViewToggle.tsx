import { ICON_PATHS } from "@/lib/icon-paths";
export const VIEW_ICONS = {
  cards: ICON_PATHS.viewCards,
  table: ICON_PATHS.viewTable,
  kanban: ICON_PATHS.viewKanban,
};

export default function ViewToggle({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: { key: string; label: string; icon: string }[];
}) {
  return (
    <div className="flex border border-[var(--border-default)] rounded-[var(--radius-md)] overflow-hidden h-8">
      {options.map((opt) => (
        <button
          key={opt.key}
          onClick={() => onChange(opt.key)}
          className={`px-2 py-1.5 transition-colors ${
            value === opt.key
              ? "bg-[var(--accent-muted)] text-[var(--accent)]"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
          }`}
          title={opt.label}
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d={opt.icon} />
          </svg>
        </button>
      ))}
    </div>
  );
}
