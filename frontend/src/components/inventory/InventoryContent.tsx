"use client";

import type { ReactNode, RefObject } from "react";
import EmptyState from "@/components/ui/EmptyState";
import Spinner from "@/components/ui/Spinner";
import { SkeletonCard, SkeletonTable } from "@/components/ui/Skeleton";
import { useLocale } from "@/contexts/LocaleContext";
import SectionHeading from "@/components/ui/SectionHeading";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { ItemGroup } from "@/lib/grouping";

// Full literals so Tailwind generates them.
const GRID = {
  4: "grid-cols-1 md:grid-cols-2 xl:grid-cols-4",
  3: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
} as const;

interface InventoryContentProps<T extends { id: number }> {
  isLoading: boolean;
  items: T[];
  viewMode: "cards" | "table";
  emptyIcon: "server" | "globe" | "folder" | "box" | "search" | "key";
  emptyTitle: string;
  emptyDescription: string;
  emptyAction?: ReactNode;
  renderCard: (item: T, index: number) => ReactNode;
  renderTable: (items: T[]) => ReactNode;
  skeletonCount?: number;
  loadMoreRef?: RefObject<HTMLDivElement | null>;
  visibleCount?: number;
  // Manual fallback for loading the next batch when the IntersectionObserver
  // doesn't fire (offscreen sentinel, viewport snap, prefers-reduced-motion).
  // Receives the same callback that the observer would invoke.
  onLoadMore?: () => void;
  loadingMoreLabel?: string;
  loadMoreLabel?: string;
  // When set, items render as one section per group (cards grid or table),
  // all of them — grouping wants the whole list, so no load-more window.
  groups?: ItemGroup<T>[];
  /** Card columns at the widest size: 4 full-width (default), 3 beside a side column. */
  columns?: 3 | 4;
}

export default function InventoryContent<T extends { id: number }>({
  isLoading,
  items,
  viewMode,
  emptyIcon,
  emptyTitle,
  emptyDescription,
  emptyAction,
  renderCard,
  renderTable,
  skeletonCount = 6,
  loadMoreRef,
  visibleCount,
  onLoadMore,
  loadingMoreLabel,
  loadMoreLabel,
  groups,
  columns = 4,
}: InventoryContentProps<T>) {
  const { t } = useLocale();
  if (isLoading) {
    return viewMode === "cards" ? (
      <div className={`grid ${GRID[columns]} gap-4 max-md:pb-24`}>
        {Array.from({ length: skeletonCount }).map((_, i) => <SkeletonCard key={i} />)}
      </div>
    ) : (
      <SkeletonTable rows={5} />
    );
  }

  if (items.length === 0) {
    return (
      <EmptyState
        icon={emptyIcon}
        title={emptyTitle}
        description={emptyDescription}
        action={emptyAction}
      />
    );
  }

  if (groups) {
    return (
      <div className="space-y-8 max-md:pb-24">
        {groups.map((g) => (
          // Native disclosure: collapsible with no state, keyboard and screen
          // reader support included.
          <details key={g.id ?? "unlinked"} open className="group">
            <summary className="list-none cursor-pointer [&::-webkit-details-marker]:hidden">
              <SectionHeading variant="rule" count={g.items.length}>
                <Icon path={ICON_PATHS.chevronRight} className="w-3.5 h-3.5 text-[var(--text-muted)] transition-transform duration-150 group-open:rotate-90" />
                {g.label}
              </SectionHeading>
            </summary>
            {viewMode === "table" ? (
              renderTable(g.items)
            ) : (
              <div className={`grid ${GRID[columns]} gap-4`}>
                {g.items.map((item, i) => (
                  <div key={item.id} className="stagger-in h-full" style={{ "--i": Math.min(i, 8) } as React.CSSProperties}>
                    {renderCard(item, i)}
                  </div>
                ))}
              </div>
            )}
          </details>
        ))}
      </div>
    );
  }

  if (viewMode === "table") {
    return <>{renderTable(items)}</>;
  }

  const displayItems = visibleCount != null ? items.slice(0, visibleCount) : items;

  return (
    <>
      <div className={`grid ${GRID[columns]} gap-4 max-md:pb-24`}>
        {displayItems.map((item, i) => (
          <div key={item.id} className="stagger-in h-full" style={{ "--i": i } as React.CSSProperties}>
            {renderCard(item, i)}
          </div>
        ))}
      </div>
      {loadMoreRef && visibleCount != null && visibleCount < items.length && (
        // Footer that doubles as the IntersectionObserver sentinel. When it
        // scrolls into view the parent's observer bumps visibleCount; the
        // explicit button below it is the manual fallback for cases where
        // the observer never fires (the spinner stays visible but never
        // resolves into more cards).
        <div
          ref={loadMoreRef}
          className="mt-6 flex flex-col items-center gap-3 py-4 text-sm text-[var(--text-muted)]"
        >
          <div className="inline-flex items-center gap-2">
            <Spinner />
            <span>
              {loadingMoreLabel ?? t("common.loadingMore")} ({visibleCount}/{items.length})
            </span>
          </div>
          {onLoadMore && (
            <button
              type="button"
              onClick={onLoadMore}
              className="px-3 py-1 rounded-[var(--radius-md)] border border-[var(--border-default)] text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-primary)] transition-colors"
            >
              {loadMoreLabel ?? t("common.loadMore")}
            </button>
          )}
        </div>
      )}
    </>
  );
}
