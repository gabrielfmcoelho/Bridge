"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DrawerSection from "@/components/ui/DrawerSection";
import Select from "@/components/ui/Select";
import PillButton from "@/components/ui/PillButton";
import InventoryFilterDrawer from "@/components/inventory/InventoryFilterDrawer";
import { tagsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";

export type ServiceFilters = {
  tag: string;
  developed_by: string;
  is_external_dependency: string;
  orchestrator_managed: string;
};

export const emptyFilters: ServiceFilters = { tag: "", developed_by: "", is_external_dependency: "", orchestrator_managed: "" };

interface SortConfig { field: string; direction: "asc" | "desc" }

interface FilterDrawerProps {
  open: boolean;
  onClose: () => void;
  filters: ServiceFilters;
  onFiltersChange: (f: ServiceFilters) => void;
  sort: SortConfig;
  onSortChange: (s: SortConfig) => void;
  search: string;
  onSearchChange: (s: string) => void;
}

export default function ServiceFilterDrawer({
  open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange,
}: FilterDrawerProps) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<string | null>(null);

  const { data: allTags = [] } = useQuery({ queryKey: ["tags", "service"], queryFn: () => tagsAPI.list("service") });
  const tags = Array.isArray(allTags) ? allTags : [];

  const set = (key: keyof ServiceFilters, value: string) => onFiltersChange({ ...filters, [key]: value });
  const toggle = (key: string) => setOpenSection((prev) => (prev === key ? null : key));

  return (
    <InventoryFilterDrawer
      open={open}
      onClose={onClose}
      filters={filters}
      onFiltersChange={onFiltersChange}
      emptyFilters={emptyFilters}
      sort={sort}
      onSortChange={onSortChange}
      search={search}
      onSearchChange={onSearchChange}
      sortFields={[
        { field: "nickname", label: t("service.nickname") },
        { field: "technology_stack", label: t("service.technologyStack") },
      ]}
      defaultSortField="nickname"
    >
      <DrawerSection title={t("common.tags")} open={openSection === "tags"} onToggle={() => toggle("tags")} active={!!filters.tag}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.tag} onClick={() => set("tag", "")}>{t("common.all") || "All"}</PillButton>
          {tags.map((tag) => (
            <PillButton key={tag} active={tag === filters.tag} onClick={() => set("tag", tag === filters.tag ? "" : tag)}>{tag}</PillButton>
          ))}
        </div>
      </DrawerSection>

      <DrawerSection title={t("service.developedBy")} open={openSection === "developed_by"} onToggle={() => toggle("developed_by")} active={!!filters.developed_by}>
        <Select value={filters.developed_by} onChange={(e) => set("developed_by", e.target.value)} options={[{ value: "", label: "--" }, { value: "internal", label: t("service.internal") }, { value: "external", label: t("service.external") }]} />
      </DrawerSection>

      <DrawerSection title={t("service.isExternalDependency")} open={openSection === "external"} onToggle={() => toggle("external")} active={!!filters.is_external_dependency}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.is_external_dependency} onClick={() => set("is_external_dependency", "")}>{t("common.all") || "All"}</PillButton>
          <PillButton active={filters.is_external_dependency === "yes"} onClick={() => set("is_external_dependency", filters.is_external_dependency === "yes" ? "" : "yes")}>{t("common.yes") || "Yes"}</PillButton>
          <PillButton active={filters.is_external_dependency === "no"} onClick={() => set("is_external_dependency", filters.is_external_dependency === "no" ? "" : "no")}>{t("common.no") || "No"}</PillButton>
        </div>
      </DrawerSection>

      <DrawerSection title={t("service.orchestratorManaged")} open={openSection === "orchestrated"} onToggle={() => toggle("orchestrated")} active={!!filters.orchestrator_managed}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.orchestrator_managed} onClick={() => set("orchestrator_managed", "")}>{t("common.all") || "All"}</PillButton>
          <PillButton active={filters.orchestrator_managed === "yes"} onClick={() => set("orchestrator_managed", filters.orchestrator_managed === "yes" ? "" : "yes")}>{t("common.yes") || "Yes"}</PillButton>
          <PillButton active={filters.orchestrator_managed === "no"} onClick={() => set("orchestrator_managed", filters.orchestrator_managed === "no" ? "" : "no")}>{t("common.no") || "No"}</PillButton>
        </div>
      </DrawerSection>
    </InventoryFilterDrawer>
  );
}
