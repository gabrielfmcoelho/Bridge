"use client";

import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { glpiAPI, type GlpiDropdownCatalogueSummary } from "@/lib/api";
import Button from "@/components/ui/Button";
import DropdownCatalogueEditorModal from "./DropdownCatalogueEditorModal";
import { useLocale } from "@/contexts/LocaleContext";

// DropdownCatalogueSection is embedded inside GLPIIntegrationSection. Lists
// the allow-listed itemtypes sshcm can serve picker data for, the current
// option count and last-updated timestamp, and an Edit button per row.
export default function DropdownCatalogueSection() {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["glpi-dropdown-catalogues"],
    queryFn: glpiAPI.listDropdownCatalogues,
    retry: false,
  });

  const deleteMutation = useMutation({
    mutationFn: (itemtype: string) => glpiAPI.deleteDropdownCatalogue(itemtype),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["glpi-dropdown-catalogues"] }),
  });

  const summaryByItemtype = useMemo(() => {
    const map = new Map<string, GlpiDropdownCatalogueSummary>();
    for (const c of data?.catalogues ?? []) map.set(c.itemtype, c);
    return map;
  }, [data]);

  const allowed = data?.allowed_itemtypes ?? [];

  return (
    <details className="border-t border-[var(--border-default)] pt-4" open>
      <summary className="cursor-pointer list-none text-sm font-semibold text-[var(--text-primary)] flex items-center justify-between gap-2">
        <span>{t("glpi.dropdownCatalogueTitle")}</span>
        <span className="text-xs font-normal text-[var(--text-muted)]">
          {t("glpi.itemtypesConfigured", { n: String(data?.catalogues?.length ?? 0) })}
        </span>
      </summary>

      <p className="mt-2 text-xs text-[var(--text-muted)]">
        {t("glpi.dropdownCatalogueHint")}
      </p>

      {isLoading ? (
        <p className="mt-3 text-xs text-[var(--text-muted)] animate-pulse">{t("common.loading")}</p>
      ) : (
        <ul className="mt-3 space-y-1">
          {allowed.map((it) => {
            const summary = summaryByItemtype.get(it);
            return (
              <li
                key={it}
                className="flex items-center justify-between gap-3 border border-[var(--border-subtle)] bg-[var(--bg-elevated)] rounded-[var(--radius-md)] px-3 py-2 text-sm"
              >
                <div className="min-w-0 flex-1">
                  <code className="font-mono text-[var(--text-primary)]">{it}</code>
                  {summary ? (
                    <span className="ml-2 text-xs text-[var(--text-muted)]">
                      {t("glpi.catalogueOptionCountUpdated", {
                        n: String(summary.option_count),
                        date: new Date(summary.updated_at).toLocaleString(),
                      })}
                    </span>
                  ) : (
                    <span className="ml-2 text-xs text-[var(--text-faint)]">
                      {t("glpi.catalogueEmptyUsesRest")}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <Button type="button" size="sm" variant="secondary" onClick={() => setEditing(it)}>
                    {t("common.edit")}
                  </Button>
                  {summary && summary.option_count > 0 && (
                    <Button
                      type="button"
                      size="sm"
                      variant="secondary"
                      onClick={() => {
                        if (confirm(t("glpi.catalogueDeleteConfirm", { itemtype: it }))) {
                          deleteMutation.mutate(it);
                        }
                      }}
                      loading={deleteMutation.isPending}
                    >
                      {t("glpi.clearCatalogue")}
                    </Button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}

      <DropdownCatalogueEditorModal
        itemtype={editing}
        open={editing !== null}
        onClose={() => setEditing(null)}
        onSaved={() => {
          queryClient.invalidateQueries({ queryKey: ["glpi-dropdown-catalogues"] });
          setEditing(null);
        }}
      />
    </details>
  );
}
