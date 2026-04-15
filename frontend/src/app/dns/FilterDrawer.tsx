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

export type DNSFilters = {
  situacao: string;
  tag: string;
  responsavel: string;
  has_https: string;
};

export const emptyFilters: DNSFilters = { situacao: "", tag: "", responsavel: "", has_https: "" };

type SectionKey = "situacao" | "tags" | "responsavel" | "https" | "sort";

interface FilterDrawerProps {
  open: boolean;
  onClose: () => void;
  filters: DNSFilters;
  onFiltersChange: (f: DNSFilters) => void;
  sort: string;
  onSortChange: (s: string) => void;
  search: string;
  onSearchChange: (s: string) => void;
}

export default function DnsFilterDrawer({
  open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange,
}: FilterDrawerProps) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<SectionKey | null>(null);

  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const { data: allTags = [] } = useQuery({ queryKey: ["tags", "dns"], queryFn: () => tagsAPI.list("dns") });
  const tags = Array.isArray(allTags) ? allTags : [];

  const set = (key: keyof DNSFilters, value: string) =>
    onFiltersChange({ ...filters, [key]: value });

  const activeCount = Object.values(filters).filter(Boolean).length;
  const toggle = (key: SectionKey) => setOpenSection((prev) => (prev === key ? null : key));

  const sortFields = [
    { field: "domain", label: t("dns.domain") || "Domain" },
    { field: "situacao", label: t("host.situacao") || "Situacao" },
    { field: "responsavel", label: t("dns.responsavel") || "Responsavel" },
  ];

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

        <DrawerSection title={t("host.situacao")} open={openSection === "situacao"} onToggle={() => toggle("situacao")} active={!!filters.situacao}>
          <Select value={filters.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((s) => ({ value: s.value, label: s.value }))} />
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

        <DrawerSection title={t("dns.responsavel")} open={openSection === "responsavel"} onToggle={() => toggle("responsavel")} active={!!filters.responsavel}>
          <Select value={filters.responsavel} onChange={(e) => set("responsavel", e.target.value)} options={[{ value: "", label: "--" }, ...contactsToOptions(contacts)]} />
        </DrawerSection>

        <DrawerSection title="HTTPS" open={openSection === "https"} onToggle={() => toggle("https")} active={!!filters.has_https}>
          <div className="flex flex-wrap gap-1.5">
            <PillButton active={!filters.has_https} onClick={() => set("has_https", "")}>{t("common.all")}</PillButton>
            <PillButton active={filters.has_https === "yes"} onClick={() => set("has_https", filters.has_https === "yes" ? "" : "yes")}>
              {t("common.yes") || "Yes"}
            </PillButton>
            <PillButton active={filters.has_https === "no"} onClick={() => set("has_https", filters.has_https === "no" ? "" : "no")}>
              {t("common.no") || "No"}
            </PillButton>
          </div>
        </DrawerSection>

        <DrawerSection title={t("filters.sortBy") || "Sort by"} open={openSection === "sort"} onToggle={() => toggle("sort")} active={sort !== "domain"}>
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
