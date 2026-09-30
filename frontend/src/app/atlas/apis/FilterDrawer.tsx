"use client";

import { useState } from "react";
import DrawerSection from "@/components/ui/DrawerSection";
import PillButton from "@/components/ui/PillButton";
import InventoryFilterDrawer from "@/components/inventory/InventoryFilterDrawer";
import { useLocale } from "@/contexts/LocaleContext";
import { emptyApiFilters, type ApiFilters } from "./_components/apiInsights";

interface SortConfig { field: string; direction: "asc" | "desc" }

export default function ApiFilterDrawer({ open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange, specVersions }: {
  open: boolean;
  onClose: () => void;
  filters: ApiFilters;
  onFiltersChange: (f: ApiFilters) => void;
  sort: SortConfig;
  onSortChange: (s: SortConfig) => void;
  search: string;
  onSearchChange: (s: string) => void;
  /** The spec versions present in the list. */
  specVersions: string[];
}) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<string | null>(null);
  const set = (key: keyof ApiFilters, value: string) => onFiltersChange({ ...filters, [key]: filters[key] === value ? "" : value });
  const toggle = (key: string) => setOpenSection((prev) => (prev === key ? null : key));
  const pills = (key: keyof ApiFilters, options: { value: string; label: string }[]) => (
    <div className="flex flex-wrap gap-1.5">
      <PillButton active={!filters[key]} onClick={() => onFiltersChange({ ...filters, [key]: "" })}>{t("common.all")}</PillButton>
      {options.map((o) => <PillButton key={o.value} active={filters[key] === o.value} onClick={() => set(key, o.value)}>{o.label}</PillButton>)}
    </div>
  );

  return (
    <InventoryFilterDrawer
      open={open}
      onClose={onClose}
      filters={filters}
      onFiltersChange={onFiltersChange}
      emptyFilters={emptyApiFilters}
      sort={sort}
      onSortChange={onSortChange}
      search={search}
      onSearchChange={onSearchChange}
      sortFields={[
        { field: "name", label: t("atlas.apis.name") },
        { field: "operation_count", label: t("atlas.apis.tabEndpoints") },
        { field: "updated_at", label: t("atlas.apis.updated") },
      ]}
      defaultSortField="name"
    >
      <DrawerSection title={t("atlas.apis.links")} open={openSection === "links"} onToggle={() => toggle("links")} active={!!filters.links}>
        {pills("links", [{ value: "linked", label: t("atlas.apis.linked") }, { value: "none", label: t("atlas.apis.avulso") }])}
      </DrawerSection>
      <DrawerSection title={t("atlas.apis.sourceLabel")} open={openSection === "source"} onToggle={() => toggle("source")} active={!!filters.source}>
        {pills("source", ["upload", "url"].map((v) => ({ value: v, label: t(`atlas.apis.sourceType.${v}`) })))}
      </DrawerSection>
      <DrawerSection title={t("atlas.apis.specVersion")} open={openSection === "spec"} onToggle={() => toggle("spec")} active={!!filters.spec}>
        {pills("spec", specVersions.map((v) => ({ value: v, label: v })))}
      </DrawerSection>
      <DrawerSection title={t("atlas.apis.health")} open={openSection === "health"} onToggle={() => toggle("health")} active={!!filters.base || !!filters.stale}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={filters.base === "none"} onClick={() => set("base", "none")}>{t("atlas.apis.kpi.noBaseUrl")}</PillButton>
          <PillButton active={filters.stale === "yes"} onClick={() => set("stale", "yes")}>{t("atlas.apis.kpi.specStale")}</PillButton>
        </div>
      </DrawerSection>
    </InventoryFilterDrawer>
  );
}
