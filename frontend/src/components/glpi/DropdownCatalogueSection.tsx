"use client";

import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { glpiAPI, type GlpiDropdownCatalogueSummary } from "@/lib/api";
import Button from "@/components/ui/Button";
import SectionCard from "@/components/ui/SectionCard";
import DropdownCatalogueEditorModal from "./DropdownCatalogueEditorModal";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";

// DropdownCatalogueSection is embedded inside GLPIIntegrationSection. Lists
// the allow-listed itemtypes sshcm can serve picker data for, the current
// option count and last-updated timestamp, and an Edit button per row.
export default function DropdownCatalogueSection() {
  const confirm = useConfirm();
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
    // Plain: it sits inside the GLPI integration card, so no second card chrome.
    <SectionCard
      as="h3"
      variant="plain"
      collapsible
      title={t("glpi.dropdownCatalogueTitle")}
      count={data?.catalogues?.length ?? 0}
      description={t("glpi.dropdownCatalogueHint")}
    >
      {isLoading ? (
        <p className="text-xs text-[var(--text-muted)] animate-pulse">{t("common.loading")}</p>
      ) : (
        <ul className="space-y-1">
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
                      onClick={async () => {
                        if (await confirm({ title: t("confirm.deleteTitle", { name: it }), message: t("glpi.catalogueDeleteConfirm", { itemtype: it }), danger: true, confirmLabel: t("common.delete") })) {
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
    </SectionCard>
  );
}
