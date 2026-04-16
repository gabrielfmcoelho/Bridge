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
        <div ref={loadMoreRef} className="h-1" />
      )}
    </>
  );
}
