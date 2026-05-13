import type { ReactNode, RefObject } from "react";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard, SkeletonTable } from "@/components/ui/Skeleton";

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
}: InventoryContentProps<T>) {
  if (isLoading) {
    return viewMode === "cards" ? (
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
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

  if (viewMode === "table") {
    return <>{renderTable(items)}</>;
  }

  const displayItems = visibleCount != null ? items.slice(0, visibleCount) : items;

  return (
    <>
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
        {displayItems.map((item, i) => (
          <div key={item.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
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
            <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
            <span>
              {loadingMoreLabel ?? "Loading more..."} ({visibleCount}/{items.length})
            </span>
          </div>
          {onLoadMore && (
            <button
              type="button"
              onClick={onLoadMore}
              className="px-3 py-1 rounded-[var(--radius-md)] border border-[var(--border-default)] text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-primary)] transition-colors"
            >
              {loadMoreLabel ?? "Load more"}
            </button>
          )}
        </div>
      )}
    </>
  );
}
