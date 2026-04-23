"use client";

import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { glpiAPI, type GlpiDropdownCatalogueSummary } from "@/lib/api";
import Button from "@/components/ui/Button";
import DropdownCatalogueEditorModal from "./DropdownCatalogueEditorModal";

// DropdownCatalogueSection is embedded inside GLPIIntegrationSection. Lists
// the allow-listed itemtypes sshcm can serve picker data for, the current
// option count and last-updated timestamp, and an Edit button per row.
export default function DropdownCatalogueSection() {
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
        <span>Catálogo de dropdowns</span>
        <span className="text-[11px] font-normal text-[var(--text-muted)]">
          {data?.catalogues?.length ?? 0} itemtype
          {(data?.catalogues?.length ?? 0) === 1 ? "" : "s"} configurado
          {(data?.catalogues?.length ?? 0) === 1 ? "" : "s"}
        </span>
      </summary>

      <p className="mt-2 text-[11px] text-[var(--text-muted)]">
        Quando o perfil GLPI não tem permissão de leitura via REST para
        dropdowns (ex.: <code>/ITILCategory</code>), os pickers do Formcreator
        voltam vazios. Cole aqui manualmente a lista — uma vez por itemtype,
        como os formulários raramente mudam. O snippet abaixo do editor coleta
        os <code>&lt;option&gt;</code> da página do GLPI já aberta no seu
        navegador.
      </p>

      {isLoading ? (
        <p className="mt-3 text-xs text-[var(--text-muted)] animate-pulse">Carregando…</p>
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
                    <span className="ml-2 text-[11px] text-[var(--text-muted)]">
                      {summary.option_count} opção
                      {summary.option_count === 1 ? "" : "es"}
                      {" · atualizado "}
                      {new Date(summary.updated_at).toLocaleString()}
                    </span>
                  ) : (
                    <span className="ml-2 text-[11px] text-[var(--text-faint)]">
                      vazio — usa REST
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <Button type="button" size="sm" variant="secondary" onClick={() => setEditing(it)}>
                    Editar
                  </Button>
                  {summary && summary.option_count > 0 && (
                    <Button
                      type="button"
                      size="sm"
                      variant="secondary"
                      onClick={() => {
                        if (confirm(`Apagar o catálogo de ${it}? Os pickers voltam a tentar o REST.`)) {
                          deleteMutation.mutate(it);
                        }
                      }}
                      loading={deleteMutation.isPending}
                    >
                      Limpar
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
