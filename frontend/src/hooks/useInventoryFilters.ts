import { useState, useCallback, useMemo } from "react";
import { useLocalStorage } from "./useLocalStorage";

interface SortConfig {
  field: string;
  direction: "asc" | "desc";
}

interface UseInventoryFiltersOptions<F> {
  storageKey: string;
  emptyFilters: F;
  defaultSort: SortConfig;
}

export function useInventoryFilters<F extends Record<string, string>>({
  storageKey,
  emptyFilters,
  defaultSort,
}: UseInventoryFiltersOptions<F>) {
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<F>(emptyFilters);
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [sort, setSort] = useLocalStorage<SortConfig>(`${storageKey}_sort`, defaultSort);

  const activeFilterCount = useMemo(
    () => Object.values(filters).filter(Boolean).length,
    [filters],
  );

  const resetAll = useCallback(() => {
    setSearch("");
    setFilters(emptyFilters);
    setSort(defaultSort);
  }, [emptyFilters, defaultSort, setSort]);

  return {
    search,
    setSearch,
    filters,
    setFilters,
    sort,
    setSort,
    viewMode,
    setViewMode,
    activeFilterCount,
    resetAll,
  };
}
