import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface EmptyStateProps {
  icon?: "server" | "globe" | "folder" | "box" | "search" | "key" | "topology";
  title: string;
  description?: string;
  /** The one call to action (ADS: never more than one primary). */
  action?: React.ReactNode;
  /** Optional low-emphasis follow-up (a link to docs, "clear filters"). */
  secondaryAction?: React.ReactNode;
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

// ADS empty state: centred, 464px (304px compact), header + description, one CTA.
export default function EmptyState({ icon = "search", title, description, action, secondaryAction, compact }: EmptyStateProps) {
  return (
    <div className={`mx-auto flex flex-col items-center justify-center animate-fade-in px-6 text-center ${compact ? "max-w-[19rem] py-12" : "max-w-[29rem] min-h-[50vh]"}`}>
      <div className="w-16 h-16 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)] flex items-center justify-center mb-4">
        <Icon path={icons[icon]} className="w-7 h-7 text-[var(--text-muted)]" strokeWidth={1.5} />
      </div>
      <h3 className="text-heading-sm font-semibold font-display text-[var(--text-primary)] mb-1">{title}</h3>
      {description && <p className="text-sm text-[var(--text-secondary)] mb-4">{description}</p>}
      {(action || secondaryAction) && (
        <div className="flex flex-col items-center gap-2">
          {action}
          {secondaryAction}
        </div>
      )}
    </div>
  );
}
