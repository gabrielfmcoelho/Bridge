"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import DrawerSection from "@/components/ui/DrawerSection";
import Select from "@/components/ui/Select";
import Button from "@/components/ui/Button";
import PillButton from "@/components/ui/PillButton";
import { enumsAPI, contactsAPI, tagsAPI } from "@/lib/api";
import { contactsToOptions } from "@/lib/utils";
import { useLocale } from "@/contexts/LocaleContext";
import type { HostFilters, HostSortConfig, SortField } from "@/lib/types";

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

const emptyFilters: HostFilters = {
  situacao: "", tag: "", entidade_responsavel: "",
  responsavel_interno: "", key_test_status: "", password_test_status: "",
  has_scan: "", alert_level: "",
};

/* sortFields moved inside component for t() access */

type SectionKey = "situacao" | "tags" | "responsaveis" | "tests" | "scan" | "sort";

export default function FilterDrawer({
  open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange,
}: FilterDrawerProps) {
  const { t } = useLocale();

  const sortFields: { field: SortField; label: string }[] = [
    { field: "nickname", label: t("sort.name") },
    { field: "situacao", label: t("sort.situacao") },
    { field: "containers_count", label: t("sort.containers") },
    { field: "resource_cpu", label: t("sort.cpuPercent") },
    { field: "resource_ram", label: t("sort.ramPercent") },
    { field: "resource_disk", label: t("sort.diskPercent") },
  ];

  const [openSection, setOpenSection] = useState<SectionKey | null>(null);

  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const { data: entidades = [] } = useQuery({
    queryKey: ["enums", "entidade_responsavel"],
    queryFn: () => enumsAPI.list("entidade_responsavel"),
  });
  const { data: rawContacts } = useQuery({
    queryKey: ["contacts"],
    queryFn: contactsAPI.list,
  });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const { data: allTags = [] } = useQuery({
    queryKey: ["tags", "host"],
    queryFn: () => tagsAPI.list("host"),
  });
  const tags = Array.isArray(allTags) ? allTags : [];

  const set = (key: keyof HostFilters, value: string) =>
    onFiltersChange({ ...filters, [key]: value });

  const activeCount = Object.values(filters).filter(Boolean).length;

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

  const toggle = (key: SectionKey) =>
    setOpenSection((prev) => (prev === key ? null : key));

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("filters.title")}
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
            {t("common.close") || "Fechar"}
          </Button>
          <Button size="sm" className="flex-1" onClick={onClose}>
            {t("filters.apply")}
          </Button>
        </div>
      }
    >
      <div className="space-y-0">
        {/* Search — always visible */}
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

        {/* Flat collapsible filter sections */}
        <DrawerSection title={t("host.situacao")} open={openSection === "situacao"} onToggle={() => toggle("situacao")} active={!!filters.situacao}>
          <Select
            value={filters.situacao}
            onChange={(e) => set("situacao", e.target.value)}
            options={situacoes.map((s) => ({ value: s.value, label: s.value }))}
          />
        </DrawerSection>

        <DrawerSection title={t("common.tags")} open={openSection === "tags"} onToggle={() => toggle("tags")} active={!!filters.tag}>
          <div className="flex flex-wrap gap-1.5">
            <PillButton active={!filters.tag} onClick={() => set("tag", "")}>{t("common.all")}</PillButton>
            {tags.map((tag) => (
              <PillButton key={tag} active={tag === filters.tag} onClick={() => set("tag", tag === filters.tag ? "" : tag)}>
                {tag}
              </PillButton>
            ))}
          </div>
        </DrawerSection>

        <DrawerSection title={t("host.entidadeResponsavel")} open={openSection === "responsaveis"} onToggle={() => toggle("responsaveis")} active={!!filters.entidade_responsavel || !!filters.responsavel_interno}>
          <div className="space-y-3">
            <FieldLabel label={t("host.entidadeResponsavel")}>
              <Select
                value={filters.entidade_responsavel}
                onChange={(e) => set("entidade_responsavel", e.target.value)}
                options={entidades.map((e) => ({ value: e.value, label: e.value }))}
              />
            </FieldLabel>
            <FieldLabel label={t("host.responsavelInterno")}>
              <Select
                value={filters.responsavel_interno}
                onChange={(e) => set("responsavel_interno", e.target.value)}
                options={contactsToOptions(contacts)}
              />
            </FieldLabel>
          </div>
        </DrawerSection>

        <DrawerSection title={t("filters.tests")} open={openSection === "tests"} onToggle={() => toggle("tests")} active={!!filters.key_test_status || !!filters.password_test_status}>
          <div className="grid grid-cols-2 gap-3">
            <FieldLabel label={t("filters.keyTest")}>
              <Select value={filters.key_test_status} onChange={(e) => set("key_test_status", e.target.value)} options={testStatusOptions} />
            </FieldLabel>
            <FieldLabel label={t("filters.pwdTest")}>
              <Select value={filters.password_test_status} onChange={(e) => set("password_test_status", e.target.value)} options={testStatusOptions} />
            </FieldLabel>
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

        <DrawerSection title={t("filters.sortBy")} open={openSection === "sort"} onToggle={() => toggle("sort")} active={sort.field !== "nickname"}>
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

function FieldLabel({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1.5">
        {label}
      </label>
      {children}
    </div>
  );
}

export { emptyFilters };
