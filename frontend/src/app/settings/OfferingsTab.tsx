"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { entidadesAPI, offeringsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { Offering } from "@/lib/types";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import FormError from "@/components/ui/FormError";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { SkeletonTable } from "@/components/ui/Skeleton";
import SortableTable, { sortRows, type SortableColumn } from "@/components/ui/SortableTable";
import OfferingFormModal, { GLPI_MODE_KEY } from "./OfferingFormModal";

type ColKey = "sort_order" | "name" | "category" | "request_type" | "approver" | "glpi_mode" | "fields" | "is_active" | "actions";

// Admin CRUD over what the catalog offers. The list is the whole set —
// inactive and all — because this is where an admin turns an offering back on.
export default function OfferingsTab() {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const { data: offerings = [], isLoading, isError, error } = useQuery({
    queryKey: ["offerings", "admin"],
    queryFn: () => offeringsAPI.list(),
  });
  const { data: entidades = [] } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list });
  const [editing, setEditing] = useState<Offering | "new" | null>(null);
  const [deleteError, setDeleteError] = useState("");

  const deleteMutation = useMutation({
    mutationFn: (id: number) => offeringsAPI.delete(id),
    // Prefix match: also refreshes ["offerings", {...}] queries on /catalog.
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["offerings"] }); setDeleteError(""); },
    onError: (err: unknown) => setDeleteError(err instanceof Error ? err.message : "Failed"),
  });

  const entidadeName = (id: number | null | undefined) => entidades.find((e) => e.id === id)?.name ?? "—";
  const categories = Array.from(new Set(offerings.map((o) => o.category).filter(Boolean))).sort();

  const columns: SortableColumn<ColKey>[] = [
    { key: "sort_order", label: t("settings.offerings.sortOrder"), align: "right" },
    { key: "name", label: t("settings.offerings.name") },
    { key: "category", label: t("settings.offerings.category") },
    { key: "request_type", label: t("settings.offerings.requestType") },
    { key: "approver", label: t("settings.offerings.approver") },
    { key: "glpi_mode", label: t("settings.offerings.glpiMode") },
    { key: "fields", label: t("settings.offerings.fields"), align: "right" },
    { key: "is_active", label: t("common.status") },
    { key: "actions", label: "", sortable: false },
  ];
  const comparators: Record<ColKey, (a: Offering, b: Offering) => number> = {
    sort_order: (a, b) => a.sort_order - b.sort_order,
    name: (a, b) => a.name.localeCompare(b.name),
    category: (a, b) => a.category.localeCompare(b.category),
    request_type: (a, b) => a.request_type.localeCompare(b.request_type),
    approver: (a, b) => entidadeName(a.approver_entidade_id).localeCompare(entidadeName(b.approver_entidade_id)),
    glpi_mode: (a, b) => a.glpi_mode.localeCompare(b.glpi_mode),
    fields: (a, b) => a.form_schema.fields.length - b.form_schema.fields.length,
    is_active: (a, b) => Number(b.is_active) - Number(a.is_active),
    actions: () => 0,
  };

  const addButton = <Button size="sm" onClick={() => setEditing("new")}>{t("settings.offerings.add")}</Button>;

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex items-start justify-between gap-3 mb-4">
          <div>
            <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-1">{t("settings.offerings.title")}</h3>
            <p className="text-xs text-[var(--text-muted)]">{t("settings.offerings.intro")}</p>
          </div>
          {offerings.length > 0 && addButton}
        </div>
        <FormError message={deleteError} />

        {isLoading ? (
          <SkeletonTable rows={6} />
        ) : isError ? (
          <StatusAlert variant="error">{(error as Error)?.message || t("settings.offerings.loadError")}</StatusAlert>
        ) : offerings.length === 0 ? (
          <EmptyState icon="box" title={t("settings.offerings.empty")} description={t("settings.offerings.emptyHint")} action={addButton} compact />
        ) : (
          <SortableTable columns={columns} defaultSort="sort_order">
            {(sortKey, sortDir) =>
              sortRows(offerings, sortKey, sortDir, comparators).map((o, i) => (
                <tr key={o.id} className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}>
                  <td className="px-4 py-2.5 text-right text-[var(--text-muted)] tabular-nums">{o.sort_order}</td>
                  <td className="px-4 py-2.5">
                    <div className="font-medium text-[var(--text-primary)]">{o.name}</div>
                    <div className="font-mono text-xs text-[var(--text-faint)]">{o.slug}</div>
                  </td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{o.category || "—"}</td>
                  <td className="px-4 py-2.5"><Badge color="cyan">{t(`catalog.requestType.${o.request_type}`)}</Badge></td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{entidadeName(o.approver_entidade_id)}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{t(GLPI_MODE_KEY[o.glpi_mode])}</td>
                  <td className="px-4 py-2.5 text-right text-[var(--text-muted)] tabular-nums">{o.form_schema.fields.length}</td>
                  <td className="px-4 py-2.5">
                    <Badge color={o.is_active ? "emerald" : "gray"} dot>{o.is_active ? t("common.active") : t("common.inactive")}</Badge>
                  </td>
                  <td className="px-4 py-2.5 text-right whitespace-nowrap">
                    <button onClick={() => setEditing(o)} className="text-xs text-[var(--text-muted)] hover:text-[var(--accent)] mr-3">{t("common.edit")}</button>
                    <button
                      onClick={() => { if (confirm(t("settings.offerings.deleteConfirm", { name: o.name }))) deleteMutation.mutate(o.id); }}
                      className="text-xs text-[var(--text-faint)] hover:text-[var(--danger)]"
                    >
                      {t("common.delete")}
                    </button>
                  </td>
                </tr>
              ))
            }
          </SortableTable>
        )}
      </Card>

      {editing && (
        <OfferingFormModal
          offering={editing === "new" ? null : editing}
          categories={categories}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}
