"use client";

import { useState, type ReactNode } from "react";
import Drawer from "@/components/ui/Drawer";
import DrawerSection from "@/components/ui/DrawerSection";
import Button from "@/components/ui/Button";
import PillButton from "@/components/ui/PillButton";
import { useLocale } from "@/contexts/LocaleContext";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

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
          className="p-2 rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--danger)] hover:bg-[var(--danger)]/10 transition-colors disabled:opacity-30 disabled:pointer-events-none"
          title={t("filters.clearAll")}
        >
          <Icon path={ICON_PATHS.trash} />
        </button>
      }
      footer={
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" className="flex-1" onClick={onClose}>
            {t("common.close")}
          </Button>
          <Button size="sm" className="flex-1" onClick={onClose}>
            {t("filters.apply")}
          </Button>
        </div>
      }
    >
      <div className="space-y-0">
        <div className="relative pb-3">
          <Icon path={ICON_PATHS.search} className="absolute left-3 top-1/2 -translate-y-1/2 -mt-1.5 w-4 h-4 text-[var(--text-faint)] pointer-events-none" />
          <input
            placeholder={t("common.search")}
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-2.5 md:py-2 text-base md:text-sm transition duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
          />
        </div>

        {children}

        <DrawerSection
          title={t("filters.sortBy")}
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
                  <Icon path={ICON_PATHS.chevronUp} className={`w-3 h-3 ml-0.5 ${sort.direction === "desc" ? "rotate-180" : ""}`} />
                )}
              </PillButton>
            ))}
          </div>
        </DrawerSection>
      </div>
    </Drawer>
  );
}
