"use client";

import { useState, useMemo, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { dnsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useExportCSV } from "@/hooks/useExportCSV";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";
import EmptyState from "@/components/ui/EmptyState";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import ListToolbar from "@/components/ui/ListToolbar";
import { SkeletonCard, SkeletonTable } from "@/components/ui/Skeleton";
import DnsCard from "./_components/DnsCard";
import DnsTableView from "./_components/DnsTableView";
import KpiSection from "./_components/KpiSection";
import DnsFAB from "./_components/DnsFAB";
import DnsForm from "./DnsForm";
import DnsFilterDrawer, { emptyFilters, type DNSFilters } from "./FilterDrawer";
import type { DNSRecord } from "@/lib/types";

export default function DNSPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const isMobile = useMediaQuery("(max-width: 639px)");

  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<DNSRecord | null>(null);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<DNSFilters>(emptyFilters);
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [sort, setSort] = useLocalStorage("dns_sort", "domain");

  const canEdit = user?.role === "admin" || user?.role === "editor";
  const activeFilterCount = Object.values(filters).filter(Boolean).length;

  const { data: allRecords = [], isLoading } = useQuery({
    queryKey: ["dns"],
    queryFn: dnsAPI.list,
  });

  const filteredAndSorted = useMemo(() => {
    let result = [...allRecords];
    if (search) {
      const s = search.toLowerCase();
      result = result.filter(d => d.domain.toLowerCase().includes(s) || d.responsavel?.toLowerCase().includes(s));
    }
    if (filters.situacao) result = result.filter(d => d.situacao === filters.situacao);
    if (filters.tag) result = result.filter(d => d.tags?.includes(filters.tag));
    if (filters.responsavel) result = result.filter(d => d.responsavel === filters.responsavel);
    if (filters.has_https === "yes") result = result.filter(d => d.has_https);
    else if (filters.has_https === "no") result = result.filter(d => !d.has_https);

    result.sort((a, b) => {
      switch (sort) {
        case "situacao": return a.situacao.localeCompare(b.situacao);
        case "responsavel": return (a.responsavel || "").localeCompare(b.responsavel || "");
        default: return a.domain.localeCompare(b.domain);
      }
    });
    return result;
  }, [allRecords, search, filters, sort]);

  const deleteMutation = useMutation({
    mutationFn: (id: number) => dnsAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["dns"] }),
  });

  const exportCSV = useExportCSV(
    allRecords,
    [
      { key: "domain", header: "domain" },
      { key: "has_https", header: "has_https", transform: (d) => d.has_https ? "yes" : "no" },
      { key: "situacao", header: "situacao" },
      { key: "responsavel", header: "responsavel" },
      { key: "observacoes", header: "observacoes" },
      { key: "tags", header: "tags", transform: (d) => ((d as unknown as DNSRecord).tags || []).join("; ") },
    ],
    "dns_export",
  );

  const openCreate = useCallback(() => { setEditing(null); setShowForm(true); }, []);

  return (
    <PageShell>
      {/* Header */}
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("dns.title")}</h1>
        <div className="flex items-center gap-1.5">
          <div className="hidden sm:flex">
            <ViewToggle
              value={viewMode}
              onChange={(v) => setViewMode(v as "cards" | "table")}
              options={[
                { key: "cards", label: t("common.cardView"), icon: VIEW_ICONS.cards },
                { key: "table", label: t("common.tableView"), icon: VIEW_ICONS.table },
              ]}
            />
          </div>
          {canEdit && (
            <div className="hidden sm:block">
              <Button size="sm" onClick={openCreate}><span className="mr-1">+</span> {t("dns.addDns")}</Button>
            </div>
          )}
        </div>
      </div>

      {!isLoading && allRecords.length > 0 && <KpiSection records={allRecords} t={t} />}

      {search && (
        <div className="flex items-center gap-2 mb-4">
          <span className="text-xs text-[var(--text-muted)]">{t("common.search")}:</span>
          <span className="text-xs text-[var(--text-primary)] font-medium">&ldquo;{search}&rdquo;</span>
          <button onClick={() => setSearch("")} className="text-xs text-[var(--text-faint)] hover:text-[var(--text-secondary)]">&times;</button>
        </div>
      )}

      {!isLoading && allRecords.length > 0 && (
        <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("dns.listing") || "DNS Records"}</h2>
      )}

      <ListToolbar
        search={search}
        onSearchChange={setSearch}
        onFilterClick={() => setShowFilters(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("common.search")}
        actions={
          allRecords.length > 0 ? (
            <button
              onClick={exportCSV}
              className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)] transition-all"
              title={t("common.export") || "Export"}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <span className="hidden sm:inline">{t("common.export") || "Export"}</span>
            </button>
          ) : undefined
        }
      />

      {/* Content */}
      {isLoading ? (
        viewMode === "cards" ? (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}
          </div>
        ) : (
          <SkeletonTable rows={5} />
        )
      ) : filteredAndSorted.length === 0 ? (
        <EmptyState
          icon="globe"
          title={t("common.noResults")}
          description={search || activeFilterCount ? t("host.emptyStateFilter") || "Try adjusting your filters" : t("dns.emptyStateAdd") || "Add your first DNS record"}
          action={canEdit && !search && !activeFilterCount ? <Button size="sm" onClick={openCreate}>+ {t("dns.addDns")}</Button> : undefined}
        />
      ) : viewMode === "cards" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {filteredAndSorted.map((dns, i) => (
            <div key={dns.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
              <DnsCard dns={dns} />
            </div>
          ))}
        </div>
      ) : (
        <DnsTableView
          records={filteredAndSorted}
          t={t}
        />
      )}

      <Drawer open={showForm} onClose={() => setShowForm(false)} title={editing ? t("common.edit") : t("dns.addDns")} subHeader={formSubHeader}>
        <DnsForm
          initial={editing}
          onSubHeaderChange={setFormSubHeader}
          onSuccess={() => { setShowForm(false); setEditing(null); queryClient.invalidateQueries({ queryKey: ["dns"] }); }}
        />
      </Drawer>

      <DnsFilterDrawer
        open={showFilters}
        onClose={() => setShowFilters(false)}
        filters={filters}
        onFiltersChange={setFilters}
        sort={sort}
        onSortChange={setSort}
        search={search}
        onSearchChange={setSearch}
      />

      <DnsFAB
        canEdit={canEdit}
        hasRecords={allRecords.length > 0}
        activeFilterCount={activeFilterCount}
        onAdd={openCreate}
        onFilter={() => setShowFilters(true)}
        onExport={exportCSV}
      />
    </PageShell>
  );
}
