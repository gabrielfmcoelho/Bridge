import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface EmptyStateProps {
  icon?: "server" | "globe" | "folder" | "box" | "search" | "key" | "topology";
  title: string;
  description?: string;
  action?: React.ReactNode;
  compact?: boolean;
}

const icons: Record<string, string> = {
  server: ICON_PATHS.server,
  globe: ICON_PATHS.globe,
  folder: ICON_PATHS.folder,
  box: ICON_PATHS.cube,
  search: ICON_PATHS.search,
  key: ICON_PATHS.key,
  topology: ICON_PATHS.bolt,
};

export default function EmptyState({ icon = "search", title, description, action, compact }: EmptyStateProps) {
  return (
    <div className={`flex flex-col items-center justify-center animate-fade-in px-6 text-center ${compact ? "py-12" : "min-h-[50vh]"}`}>
      <div className="w-16 h-16 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)] flex items-center justify-center mb-4">
        <Icon path={icons[icon]} className="w-7 h-7 text-[var(--text-faint)]" strokeWidth={1.5} />
      </div>
      <h3 className="text-sm font-medium text-[var(--text-secondary)] mb-1">{title}</h3>
      {description && <p className="text-xs text-[var(--text-muted)] mb-4">{description}</p>}
      {action}
    </div>
  );
}
