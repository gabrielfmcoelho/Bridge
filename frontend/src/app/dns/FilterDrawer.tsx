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

export type DNSFilters = {
  situacao: string;
  tag: string;
  responsavel: string;
  has_https: string;
  cert: string;
};

export const emptyFilters: DNSFilters = { situacao: "", tag: "", responsavel: "", has_https: "", cert: "" };

interface SortConfig { field: string; direction: "asc" | "desc" }

interface FilterDrawerProps {
  open: boolean;
  onClose: () => void;
  filters: DNSFilters;
  onFiltersChange: (f: DNSFilters) => void;
  sort: SortConfig;
  onSortChange: (s: SortConfig) => void;
  search: string;
  onSearchChange: (s: string) => void;
}

export default function DnsFilterDrawer({
  open, onClose, filters, onFiltersChange, sort, onSortChange, search, onSearchChange,
}: FilterDrawerProps) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<string | null>(null);

  const { data: situacoes = [] } = useQuery({ queryKey: ["enums", "situacao"], queryFn: () => enumsAPI.list("situacao") });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const { data: allTags = [] } = useQuery({ queryKey: ["tags", "dns"], queryFn: () => tagsAPI.list("dns") });
  const tags = Array.isArray(allTags) ? allTags : [];

  const set = (key: keyof DNSFilters, value: string) => onFiltersChange({ ...filters, [key]: value });
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
        { field: "domain", label: t("dns.domain") },
        { field: "situacao", label: t("host.situacao") },
        { field: "responsavel", label: t("dns.responsavel") },
        { field: "cert_expires_at", label: t("dns.certExpires") },
      ]}
      defaultSortField="domain"
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

      <DrawerSection title={t("dns.responsavel")} open={openSection === "responsavel"} onToggle={() => toggle("responsavel")} active={!!filters.responsavel}>
        <Select value={filters.responsavel} onChange={(e) => set("responsavel", e.target.value)} options={[{ value: "", label: "--" }, ...contactsToOptions(contacts)]} />
      </DrawerSection>

      <DrawerSection title={t("topology.https")} open={openSection === "https"} onToggle={() => toggle("https")} active={!!filters.has_https}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.has_https} onClick={() => set("has_https", "")}>{t("common.all")}</PillButton>
          <PillButton active={filters.has_https === "yes"} onClick={() => set("has_https", filters.has_https === "yes" ? "" : "yes")}>{t("common.yes")}</PillButton>
          <PillButton active={filters.has_https === "no"} onClick={() => set("has_https", filters.has_https === "no" ? "" : "no")}>{t("common.no")}</PillButton>
        </div>
      </DrawerSection>

      <DrawerSection title={t("dns.certificate")} open={openSection === "cert"} onToggle={() => toggle("cert")} active={!!filters.cert}>
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={!filters.cert} onClick={() => set("cert", "")}>{t("common.all")}</PillButton>
          {[
            { value: "expired", label: t("dns.certExpired") },
            { value: "7", label: t("dns.certWithin7") },
            { value: "30", label: t("dns.certWithin30") },
            { value: "error", label: t("dns.certWithError") },
            { value: "unscanned", label: t("dns.certUnscanned") },
          ].map((o) => (
            <PillButton key={o.value} active={filters.cert === o.value} onClick={() => set("cert", filters.cert === o.value ? "" : o.value)}>{o.label}</PillButton>
          ))}
        </div>
      </DrawerSection>
    </InventoryFilterDrawer>
  );
}
