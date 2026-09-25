import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

/**
 * Descriptive metadata an admin can filter by (ADS tag): a neutral pill, never
 * a status (that is Lozenge) and never a count (Badge). Removable when it sits
 * in an input or an active filter.
 */
export default function Tag({ children, onRemove, removeLabel }: { children: string; onRemove?: () => void; removeLabel?: string }) {
  return (
    <span className="inline-flex max-w-[12rem] items-center gap-1 rounded-full border border-[var(--border-default)] bg-[var(--bg-overlay)] px-2 py-0.5 text-xs font-medium text-[var(--text-secondary)]">
      <span className="truncate" title={children}>{children}</span>
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={removeLabel ?? children}
          className="-mr-1 rounded-full p-0.5 text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
        >
          <Icon path={ICON_PATHS.close} className="w-3 h-3" />
        </button>
      )}
    </span>
  );
}
