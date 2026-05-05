"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import DrawerSection from "@/components/ui/DrawerSection";
import Select from "@/components/ui/Select";
import PillButton from "@/components/ui/PillButton";
import InventoryFilterDrawer from "@/components/inventory/InventoryFilterDrawer";
import { enumsAPI, contactsAPI, tagsAPI } from "@/lib/api";
import { contactsToOptions } from "@/lib/utils";
import { useLocale } from "@/contexts/LocaleContext";
import type { HostFilters, HostSortConfig, SortField } from "@/lib/types";

const emptyFilters: HostFilters = {
  situacao: "", tag: "", entidade_responsavel: "",
  responsavel_interno: "", key_test_status: "", password_test_status: "",
  scan_result: "", has_scan: "", alert_level: "",
};

interface FilterDrawerProps {
  open: boolean;
  onClose: () => void;
  filters: HostFilters;
  onFiltersChange: (f: HostFilters) => void;
  sort: HostSortConfig;
  onSortChange: (s: HostSortConfig) => void;
  search: string;
  onSearchChange: (s: string) => void;
}

export default function FilterDrawer({
  open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange,
}: FilterDrawerProps) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<string | null>(null);

  const sortFields: { field: string; label: string }[] = [
    { field: "nickname", label: t("sort.name") },
    { field: "situacao", label: t("sort.situacao") },
    { field: "containers_count", label: t("sort.containers") },
    { field: "resource_cpu", label: t("sort.cpuPercent") },
    { field: "resource_ram", label: t("sort.ramPercent") },
    { field: "resource_disk", label: t("sort.diskPercent") },
  ];

  const { data: situacoes = [] } = useQuery({ queryKey: ["enums", "situacao"], queryFn: () => enumsAPI.list("situacao") });
  const { data: entidades = [] } = useQuery({ queryKey: ["enums", "entidade_responsavel"], queryFn: () => enumsAPI.list("entidade_responsavel") });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const { data: allTags = [] } = useQuery({ queryKey: ["tags", "host"], queryFn: () => tagsAPI.list("host") });
  const tags = Array.isArray(allTags) ? allTags : [];

  const set = (key: keyof HostFilters, value: string) => onFiltersChange({ ...filters, [key]: value });
  const toggle = (key: string) => setOpenSection((prev) => (prev === key ? null : key));

  const testStatusOptions = [
    { value: "", label: "--" },
    { value: "success", label: t("filters.success") },
    { value: "failed", label: t("filters.failed") },
    { value: "untested", label: t("filters.untested") },
  ];
  const scanOptions = [
    { value: "", label: "--" },
    { value: "with", label: t("filters.withScan") },
    { value: "without", label: t("filters.withoutScan") },
  ];
  const alertOptions = [
    { value: "", label: "--" },
    { value: "critical", label: t("alert.critical") },
    { value: "warning", label: t("alert.warning") },
    { value: "info", label: t("alert.info") },
    { value: "none", label: t("filters.noAlerts") },
  ];

  // Adapt HostSortConfig to generic SortConfig for InventoryFilterDrawer
  const handleSortChange = (s: { field: string; direction: "asc" | "desc" }) =>
    onSortChange({ field: s.field as SortField, direction: s.direction });

  return (
    <InventoryFilterDrawer
      open={open}
      onClose={onClose}
      filters={filters}
      onFiltersChange={onFiltersChange}
      emptyFilters={emptyFilters}
      sort={sort}
      onSortChange={handleSortChange}
      search={search}
      onSearchChange={onSearchChange}
      sortFields={sortFields}
      defaultSortField="nickname"
    >
      <DrawerSection title={t("host.situacao")} open={openSection === "situacao"} onToggle={() => toggle("situacao")} active={!!filters.situacao}>
        <Select value={filters.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((s) => ({ value: s.value, label: s.value }))} />
      </DrawerSection>

      <DrawerSection title={t("common.tags")} open={openSection === "tags"} onToggle={() => toggle("tags")} active={!!filters.tag}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.tag} onClick={() => set("tag", "")}>{t("common.all")}</PillButton>
          {tags.map((tag) => (
            <PillButton key={tag} active={tag === filters.tag} onClick={() => set("tag", tag === filters.tag ? "" : tag)}>{tag}</PillButton>
          ))}
        </div>
      </DrawerSection>

      <DrawerSection title={t("host.entidadeResponsavel")} open={openSection === "responsaveis"} onToggle={() => toggle("responsaveis")} active={!!filters.entidade_responsavel || !!filters.responsavel_interno}>
        <div className="space-y-3">
          <FieldLabel label={t("host.entidadeResponsavel")}>
            <Select value={filters.entidade_responsavel} onChange={(e) => set("entidade_responsavel", e.target.value)} options={entidades.map((e) => ({ value: e.value, label: e.value }))} />
          </FieldLabel>
          <FieldLabel label={t("host.responsavelInterno")}>
            <Select value={filters.responsavel_interno} onChange={(e) => set("responsavel_interno", e.target.value)} options={contactsToOptions(contacts)} />
          </FieldLabel>
        </div>
      </DrawerSection>

      <DrawerSection title={t("filters.tests")} open={openSection === "tests"} onToggle={() => toggle("tests")} active={!!filters.scan_result || !!filters.key_test_status || !!filters.password_test_status}>
        <div className="space-y-3">
          <FieldLabel label={t("filters.scanResult")}>
            <Select value={filters.scan_result} onChange={(e) => set("scan_result", e.target.value)} options={testStatusOptions} />
          </FieldLabel>
          <div className="grid grid-cols-2 gap-3">
            <FieldLabel label={t("filters.keyTest")}>
              <Select value={filters.key_test_status} onChange={(e) => set("key_test_status", e.target.value)} options={testStatusOptions} />
            </FieldLabel>
            <FieldLabel label={t("filters.pwdTest")}>
              <Select value={filters.password_test_status} onChange={(e) => set("password_test_status", e.target.value)} options={testStatusOptions} />
            </FieldLabel>
          </div>
        </div>
      </DrawerSection>

      <DrawerSection title={t("filters.scanAndAlerts")} open={openSection === "scan"} onToggle={() => toggle("scan")} active={!!filters.has_scan || !!filters.alert_level}>
        <div className="grid grid-cols-2 gap-3">
          <FieldLabel label={t("filters.scan")}>
            <Select value={filters.has_scan} onChange={(e) => set("has_scan", e.target.value)} options={scanOptions} />
          </FieldLabel>
          <FieldLabel label={t("filters.alerts")}>
            <Select value={filters.alert_level} onChange={(e) => set("alert_level", e.target.value)} options={alertOptions} />
          </FieldLabel>
        </div>
      </DrawerSection>
    </InventoryFilterDrawer>
  );
}

function FieldLabel({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1.5">{label}</label>
      {children}
    </div>
  );
}

export { emptyFilters };
