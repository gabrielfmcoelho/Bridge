"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DrawerSection from "@/components/ui/DrawerSection";
import PillButton from "@/components/ui/PillButton";
import InventoryFilterDrawer from "@/components/inventory/InventoryFilterDrawer";
import { tagsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";

export type ProjectFilters = {
  tag: string;
  situacao: string;
};

export const emptyFilters: ProjectFilters = { tag: "", situacao: "" };

interface SortConfig { field: string; direction: "asc" | "desc" }

interface FilterDrawerProps {
  open: boolean;
  onClose: () => void;
  filters: ProjectFilters;
  onFiltersChange: (f: ProjectFilters) => void;
  sort: SortConfig;
  onSortChange: (s: SortConfig) => void;
  search: string;
  onSearchChange: (s: string) => void;
}

export default function ProjectFilterDrawer({
  open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange,
}: FilterDrawerProps) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<string | null>(null);

  const { data: allTags = [] } = useQuery({ queryKey: ["tags", "project"], queryFn: () => tagsAPI.list("project") });
  const tags = Array.isArray(allTags) ? allTags : [];

  const set = (key: keyof ProjectFilters, value: string) => onFiltersChange({ ...filters, [key]: value });
  const toggle = (key: string) => setOpenSection((prev) => (prev === key ? null : key));

  const situacaoOptions = ["active", "maintenance", "deprecated", "inactive"];

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
        { field: "name", label: t("project.name") || "Name" },
        { field: "situacao", label: t("host.situacao") || "Situacao" },
        { field: "setor", label: t("project.setorResponsavel") || "Setor" },
      ]}
      defaultSortField="name"
    >
      <DrawerSection title={t("common.tags")} open={openSection === "tags"} onToggle={() => toggle("tags")} active={!!filters.tag}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.tag} onClick={() => set("tag", "")}>{t("common.all")}</PillButton>
          {tags.map((tag) => (
            <PillButton key={tag} active={tag === filters.tag} onClick={() => set("tag", tag === filters.tag ? "" : tag)}>{tag}</PillButton>
          ))}
        </div>
      </DrawerSection>

      <DrawerSection title={t("host.situacao") || "Status"} open={openSection === "situacao"} onToggle={() => toggle("situacao")} active={!!filters.situacao}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.situacao} onClick={() => set("situacao", "")}>{t("common.all")}</PillButton>
          {situacaoOptions.map((s) => (
            <PillButton key={s} active={s === filters.situacao} onClick={() => set("situacao", s === filters.situacao ? "" : s)}>{s}</PillButton>
          ))}
        </div>
      </DrawerSection>
    </InventoryFilterDrawer>
  );
}
