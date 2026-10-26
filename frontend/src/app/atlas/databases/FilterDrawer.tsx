"use client";

import { useState } from "react";
import DrawerSection from "@/components/ui/DrawerSection";
import PillButton from "@/components/ui/PillButton";
import InventoryFilterDrawer from "@/components/inventory/InventoryFilterDrawer";
import { useLocale } from "@/contexts/LocaleContext";
import { emptyDatabaseFilters, ENGINES, ENVIRONMENTS, SENSITIVITIES, type DatabaseFilters } from "./_components/databaseInsights";

interface SortConfig { field: string; direction: "asc" | "desc" }

export default function DatabaseFilterDrawer({ open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange }: {
  open: boolean;
  onClose: () => void;
  filters: DatabaseFilters;
  onFiltersChange: (f: DatabaseFilters) => void;
  sort: SortConfig;
  onSortChange: (s: SortConfig) => void;
  search: string;
  onSearchChange: (s: string) => void;
}) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<string | null>(null);
  const set = (key: keyof DatabaseFilters, value: string) => onFiltersChange({ ...filters, [key]: filters[key] === value ? "" : value });
  const toggle = (key: string) => setOpenSection((prev) => (prev === key ? null : key));
  const pills = (key: keyof DatabaseFilters, options: { value: string; label: string }[]) => (
    <div className="flex flex-wrap gap-1.5">
      <PillButton active={!filters[key]} onClick={() => onFiltersChange({ ...filters, [key]: "" })}>{t("common.all")}</PillButton>
      {options.map((o) => <PillButton key={o.value} active={filters[key] === o.value} onClick={() => set(key, o.value)}>{o.label}</PillButton>)}
    </div>
  );
  const section = (key: keyof DatabaseFilters, title: string, values: readonly string[], prefix: string) => (
    <DrawerSection title={title} open={openSection === key} onToggle={() => toggle(key)} active={!!filters[key]}>
      {pills(key, values.map((v) => ({ value: v, label: t(`${prefix}.${v}`) })))}
    </DrawerSection>
  );

  return (
    <InventoryFilterDrawer
      open={open}
      onClose={onClose}
      filters={filters}
      onFiltersChange={onFiltersChange}
      emptyFilters={emptyDatabaseFilters}
      sort={sort}
      onSortChange={onSortChange}
      search={search}
      onSearchChange={onSearchChange}
      sortFields={[
        { field: "name", label: t("atlas.databases.name") },
        { field: "engine", label: t("atlas.databases.engineLabel") },
        { field: "updated_at", label: t("atlas.apis.updated") },
      ]}
      defaultSortField="name"
    >
      {section("engine", t("atlas.databases.engineLabel"), ENGINES, "atlas.databases.engine")}
      {section("environment", t("atlas.databases.environmentLabel"), ENVIRONMENTS, "atlas.databases.environment")}
      {section("sensitivity", t("atlas.databases.sensitivityLabel"), SENSITIVITIES, "atlas.databases.sensitivity")}
      <DrawerSection title={t("atlas.databases.where")} open={openSection === "where"} onToggle={() => toggle("where")} active={!!filters.where}>
        {pills("where", [{ value: "instance", label: t("atlas.databases.onInstance") }, { value: "external", label: t("atlas.databases.external") }])}
      </DrawerSection>
      <DrawerSection title={t("atlas.databases.health")} open={openSection === "health"} onToggle={() => toggle("health")}
        active={!!filters.backup || !!filters.consumers || !!filters.lgpd}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={filters.backup === "missing"} onClick={() => set("backup", "missing")}>{t("atlas.databases.kpi.prodNoBackup")}</PillButton>
          <PillButton active={filters.consumers === "none"} onClick={() => set("consumers", "none")}>{t("atlas.databases.kpi.noConsumers")}</PillButton>
          <PillButton active={filters.lgpd === "personal"} onClick={() => set("lgpd", "personal")}>{t("atlas.databases.kpi.personalData")}</PillButton>
        </div>
      </DrawerSection>
    </InventoryFilterDrawer>
  );
}
