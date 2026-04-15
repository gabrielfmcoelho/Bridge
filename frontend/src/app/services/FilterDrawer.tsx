"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import DrawerSection from "@/components/ui/DrawerSection";
import Select from "@/components/ui/Select";
import Button from "@/components/ui/Button";
import PillButton from "@/components/ui/PillButton";
import { tagsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";

export type ServiceFilters = {
  tag: string;
  developed_by: string;
  is_external_dependency: string;
  orchestrator_managed: string;
};

export const emptyFilters: ServiceFilters = {
  tag: "",
  developed_by: "",
  is_external_dependency: "",
  orchestrator_managed: "",
};

type SectionKey = "tags" | "developed_by" | "external" | "orchestrated" | "sort";

interface FilterDrawerProps {
  open: boolean;
  onClose: () => void;
  filters: ServiceFilters;
  onFiltersChange: (f: ServiceFilters) => void;
  sort: string;
  onSortChange: (s: string) => void;
  search: string;
  onSearchChange: (s: string) => void;
}

export default function ServiceFilterDrawer({
  open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange,
}: FilterDrawerProps) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<SectionKey | null>(null);

  const { data: allTags = [] } = useQuery({
    queryKey: ["tags", "service"],
    queryFn: () => tagsAPI.list("service"),
  });
  const tags = Array.isArray(allTags) ? allTags : [];

  const set = (key: keyof ServiceFilters, value: string) =>
    onFiltersChange({ ...filters, [key]: value });

  const activeCount = Object.values(filters).filter(Boolean).length;
  const toggle = (key: SectionKey) => setOpenSection((prev) => (prev === key ? null : key));

  const sortFields = [
    { field: "nickname", label: t("service.nickname") },
    { field: "technology_stack", label: t("service.technologyStack") },
  ];

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("filters.title") || t("common.filter")}
      headerAction={
        <button
          onClick={() => { onFiltersChange(emptyFilters); onSearchChange(""); }}
          disabled={activeCount === 0 && !search}
          className="p-1.5 rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-30 disabled:pointer-events-none"
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
        {/* Search */}
        <div className="relative pb-3">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 -mt-1.5 w-4 h-4 text-[var(--text-faint)] pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            placeholder={t("common.search")}
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-2 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
          />
        </div>

        <DrawerSection title={t("common.tags")} open={openSection === "tags"} onToggle={() => toggle("tags")} active={!!filters.tag}>
          <div className="flex flex-wrap gap-1.5">
            <PillButton active={!filters.tag} onClick={() => set("tag", "")}>{t("common.all") || "All"}</PillButton>
            {tags.map((tag) => (
              <PillButton key={tag} active={tag === filters.tag} onClick={() => set("tag", tag === filters.tag ? "" : tag)}>
                {tag}
              </PillButton>
            ))}
          </div>
        </DrawerSection>

        <DrawerSection title={t("service.developedBy")} open={openSection === "developed_by"} onToggle={() => toggle("developed_by")} active={!!filters.developed_by}>
          <Select
            value={filters.developed_by}
            onChange={(e) => set("developed_by", e.target.value)}
            options={[
              { value: "", label: "--" },
              { value: "internal", label: t("service.internal") },
              { value: "external", label: t("service.external") },
            ]}
          />
        </DrawerSection>

        <DrawerSection title={t("service.isExternalDependency")} open={openSection === "external"} onToggle={() => toggle("external")} active={!!filters.is_external_dependency}>
          <div className="flex flex-wrap gap-1.5">
            <PillButton active={!filters.is_external_dependency} onClick={() => set("is_external_dependency", "")}>{t("common.all") || "All"}</PillButton>
            <PillButton active={filters.is_external_dependency === "yes"} onClick={() => set("is_external_dependency", filters.is_external_dependency === "yes" ? "" : "yes")}>
              {t("common.yes") || "Sim"}
            </PillButton>
            <PillButton active={filters.is_external_dependency === "no"} onClick={() => set("is_external_dependency", filters.is_external_dependency === "no" ? "" : "no")}>
              {t("common.no") || "Nao"}
            </PillButton>
          </div>
        </DrawerSection>

        <DrawerSection title={t("service.orchestratorManaged")} open={openSection === "orchestrated"} onToggle={() => toggle("orchestrated")} active={!!filters.orchestrator_managed}>
          <div className="flex flex-wrap gap-1.5">
            <PillButton active={!filters.orchestrator_managed} onClick={() => set("orchestrator_managed", "")}>{t("common.all") || "All"}</PillButton>
            <PillButton active={filters.orchestrator_managed === "yes"} onClick={() => set("orchestrator_managed", filters.orchestrator_managed === "yes" ? "" : "yes")}>
              {t("common.yes") || "Sim"}
            </PillButton>
            <PillButton active={filters.orchestrator_managed === "no"} onClick={() => set("orchestrator_managed", filters.orchestrator_managed === "no" ? "" : "no")}>
              {t("common.no") || "Nao"}
            </PillButton>
          </div>
        </DrawerSection>

        <DrawerSection title={t("filters.sortBy") || "Sort by"} open={openSection === "sort"} onToggle={() => toggle("sort")} active={sort !== "nickname"}>
          <div className="flex flex-wrap gap-1.5">
            {sortFields.map((sf) => (
              <PillButton key={sf.field} active={sort === sf.field} onClick={() => onSortChange(sf.field)}>
                {sf.label}
              </PillButton>
            ))}
          </div>
        </DrawerSection>
      </div>
    </Drawer>
  );
}
