"use client";

import { useState, type ReactNode } from "react";
import Drawer from "@/components/ui/Drawer";
import DrawerSection from "@/components/ui/DrawerSection";
import Button from "@/components/ui/Button";
import PillButton from "@/components/ui/PillButton";
import { useLocale } from "@/contexts/LocaleContext";

interface SortConfig {
  field: string;
  direction: "asc" | "desc";
}

interface InventoryFilterDrawerProps<F> {
  open: boolean;
  onClose: () => void;
  filters: F;
  onFiltersChange: (f: F) => void;
  emptyFilters: F;
  sort: SortConfig;
  onSortChange: (s: SortConfig) => void;
  search: string;
  onSearchChange: (s: string) => void;
  sortFields: { field: string; label: string }[];
  defaultSortField: string;
  children: ReactNode;
}

export default function InventoryFilterDrawer<F>({
  open,
  onClose,
  filters,
  onFiltersChange,
  emptyFilters,
  sort,
  onSortChange,
  search,
  onSearchChange,
  sortFields,
  defaultSortField,
  children,
}: InventoryFilterDrawerProps<F>) {
  const { t } = useLocale();
  const activeCount = Object.values(filters as Record<string, string>).filter(Boolean).length;
  const [sortOpen, setSortOpen] = useState(false);

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("filters.title")}
      headerAction={
        <button
          onClick={() => { onFiltersChange(emptyFilters); onSearchChange(""); }}
          disabled={activeCount === 0 && !search}
          className="p-2 rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-30 disabled:pointer-events-none"
          title={t("filters.clearAll")}
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
          </svg>
        </button>
      }
      footer={
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" className="flex-1" onClick={onClose}>
            {t("common.close") || "Close"}
          </Button>
          <Button size="sm" className="flex-1" onClick={onClose}>
            {t("filters.apply")}
          </Button>
        </div>
      }
    >
      <div className="space-y-0">
        <div className="relative pb-3">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 -mt-1.5 w-4 h-4 text-[var(--text-faint)] pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            placeholder={t("common.search")}
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-2.5 md:py-2 text-base md:text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
          />
        </div>

        {children}

        <DrawerSection
          title={t("filters.sortBy") || "Sort by"}
          open={sortOpen}
          onToggle={() => setSortOpen((o) => !o)}
          active={sort.field !== defaultSortField}
        >
          <div className="flex flex-wrap gap-1.5">
            {sortFields.map((sf) => (
              <PillButton
                key={sf.field}
                active={sort.field === sf.field}
                onClick={() => onSortChange({
                  field: sf.field,
                  direction: sort.field === sf.field && sort.direction === "asc" ? "desc" : "asc",
                })}
              >
                {sf.label}
                {sort.field === sf.field && (
                  <svg className={`w-3 h-3 ml-0.5 ${sort.direction === "desc" ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 15l7-7 7 7" />
                  </svg>
                )}
              </PillButton>
            ))}
          </div>
        </DrawerSection>
      </div>
    </Drawer>
  );
}
