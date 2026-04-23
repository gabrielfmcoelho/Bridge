"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { glpiAPI, integrationsAPI, type FormcreatorForm } from "@/lib/api";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import FormcreatorFormDrawer from "@/components/glpi/FormcreatorFormDrawer";

export default function FormcreatorFormsPage() {
  const [profileID, setProfileID] = useState<number | null>(null);
  const [search, setSearch] = useState("");
  const [openFormID, setOpenFormID] = useState<number | null>(null);

  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    staleTime: 60_000,
  });
  const glpiEnabled = integrations?.glpi?.glpi_enabled === "true";

  const { data: profiles } = useQuery({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    enabled: glpiEnabled,
    retry: false,
  });

  // Default to the first profile once profiles load.
  useMemo(() => {
    if (profileID == null && profiles && profiles.length > 0) {
      setProfileID(profiles[0].id);
    }
    return null;
  }, [profiles, profileID]);

  const { data, isLoading, error } = useQuery({
    queryKey: ["glpi-forms", profileID],
    queryFn: () => glpiAPI.listForms(profileID!),
    enabled: glpiEnabled && !!profileID,
    retry: false,
    staleTime: 5 * 60_000,
  });

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    const forms = data?.forms ?? [];
    if (!q) return forms;
    return forms.filter((f) =>
      f.name.toLowerCase().includes(q) || (f.description || "").toLowerCase().includes(q)
    );
  }, [data, search]);

  // Group by category id (labels unknown from the list endpoint → use "Sem categoria" for 0/missing).
  const grouped = useMemo(() => {
    const map = new Map<number, FormcreatorForm[]>();
    for (const f of filtered) {
      const key = f.plugin_formcreator_categories_id ?? 0;
      const arr = map.get(key) ?? [];
      arr.push(f);
      map.set(key, arr);
    }
    map.forEach((arr) => arr.sort((a, b) => a.name.localeCompare(b.name)));
    return [...map.entries()].sort(([a], [b]) => a - b);
  }, [filtered]);

  const body = (() => {
    if (!glpiEnabled) {
      return (
        <EmptyState
          icon="box"
          title="GLPI integração desabilitada"
          description="Ativar em Settings → Integrations → GLPI."
        />
      );
    }
    if ((profiles?.length ?? 0) === 0) {
      return (
        <EmptyState
          icon="folder"
          title="Nenhum perfil configurado"
          description="Adicione um perfil GLPI em Settings → Integrations → GLPI → Token profiles."
        />
      );
    }
    if (isLoading) {
      return (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-[var(--radius-md)]" />
          ))}
        </div>
      );
    }
    if (error) {
      return (
        <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-sm px-4 py-3">
          Falha: {(error as Error).message}
        </div>
      );
    }
    if (grouped.length === 0) {
      return (
        <EmptyState
          icon="folder"
          title="Nenhum formulário visível"
          description={
            search
              ? `Sem resultados para "${search}".`
              : "O perfil selecionado não tem acesso a nenhum formulário Formcreator."
          }
        />
      );
    }

    return (
      <div className="space-y-6">
        {grouped.map(([categoryID, forms]) => (
          <section key={categoryID}>
            <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
              {categoryID === 0 ? "Sem categoria" : `Categoria #${categoryID}`}
            </h3>
            <ul className="space-y-1.5">
              {forms.map((f) => (
                <li key={f.id}>
                  <button
                    type="button"
                    onClick={() => setOpenFormID(f.id)}
                    className="w-full text-left rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-3 py-2.5 hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)] transition-colors"
                  >
                    <p className="text-sm font-medium text-[var(--text-primary)]">{f.name}</p>
                    {f.description && (
                      <p className="text-[11px] text-[var(--text-muted)] mt-0.5 line-clamp-2">
                        {f.description}
                      </p>
                    )}
                  </button>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    );
  })();

  return (
    <PageShell>
      <PageHeader title="Formulários GLPI" />

      <Card hover={false} className="!p-3 mb-4 flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-3 flex-wrap">
          <label className="text-xs text-[var(--text-muted)]">Perfil</label>
          <select
            value={profileID ?? ""}
            onChange={(e) => setProfileID(e.target.value ? parseInt(e.target.value, 10) : null)}
            className="bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-2 py-1 text-sm"
            disabled={!glpiEnabled}
          >
            {(profiles ?? []).map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Buscar formulário…"
            className="bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-1 text-sm min-w-[220px]"
          />
        </div>
        <Link
          href="/chamados"
          className="text-xs text-[var(--text-muted)] hover:text-[var(--accent)] hover:underline"
        >
          ← Voltar aos chamados
        </Link>
      </Card>

      {body}

      <FormcreatorFormDrawer
        open={openFormID != null}
        onClose={() => setOpenFormID(null)}
        formID={openFormID}
        profileID={profileID}
      />
    </PageShell>
  );
}
