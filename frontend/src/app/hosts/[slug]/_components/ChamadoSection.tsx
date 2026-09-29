"use client";

import { useState, useEffect } from "react";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { hostChamadosAPI, usersAPI } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import ChamadoDrawer from "./ChamadoDrawer";
import GlpiHostTicketsBlock from "./GlpiHostTicketsBlock";
import type { HostChamado } from "@/lib/types";

interface ChamadoSectionProps {
  chamados: HostChamado[];
  hostId: number;
  slug: string;
  canEdit: boolean;
  t: (k: string) => string;
  openCreate?: boolean;
  onCreateDone?: () => void;
}

export default function ChamadoSection({ chamados: initialChamados, hostId, slug, canEdit, t, openCreate, onCreateDone }: ChamadoSectionProps) {
  const confirm = useConfirm();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedChamado, setSelectedChamado] = useState<HostChamado | null>(null);
  const queryClient = useQueryClient();

  const { data: users = [] } = useQuery({ queryKey: ["users"], queryFn: usersAPI.list });
  const { data: rawChamados } = useQuery({
    queryKey: ["chamados", slug],
    queryFn: () => hostChamadosAPI.list(slug),
    initialData: initialChamados,
  });
  const chamados = rawChamados ?? [];

  // FAB trigger
  useEffect(() => { if (openCreate) { setSelectedChamado(null); setDrawerOpen(true); onCreateDone?.(); } }, [openCreate]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["chamados", slug] });
    queryClient.invalidateQueries({ queryKey: ["host", slug] });
    queryClient.invalidateQueries({ queryKey: ["hosts"] });
  };

  const createMutation = useMutation({
    mutationFn: (data: { chamado_id: string; title: string; status: string; user_id: number; date: string }) =>
      hostChamadosAPI.create(slug, data),
    onSuccess: () => { invalidate(); setDrawerOpen(false); },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, ...data }: { id: number; chamado_id: string; title: string; status: string; user_id: number; date: string }) =>
      hostChamadosAPI.update(slug, id, data),
    onSuccess: () => { invalidate(); setDrawerOpen(false); },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => hostChamadosAPI.delete(slug, id),
    onSuccess: () => { invalidate(); setDrawerOpen(false); },
  });

  const openDetail = (c: HostChamado) => {
    setSelectedChamado(c);
    setDrawerOpen(true);
  };

  const openCreateDrawer = () => {
    setSelectedChamado(null);
    setDrawerOpen(true);
  };

  return (
    <>
      <SectionCard as="h3" title={t("host.chamados")} count={chamados.length} body="flush" empty={chamados.length === 0 ? t("host.noChamadosDesc") : undefined} controls={
        canEdit && (
          <span className="hidden md:contents">
            <IconButton onClick={openCreateDrawer} label={t("host.addChamado")}><Icon path={ICON_PATHS.plus} /></IconButton>
          </span>
        )
      }>
        <div className="divide-y divide-[var(--border-subtle)]">
          {[...chamados].sort((a, b) => (b.date || "").localeCompare(a.date || "")).map((c, i) => {
            const solved = c.status === "solved";
            const statusLabel = c.status === "in_execution" ? t("chamado.inExecution") : solved ? t("chamado.solved") : c.status;
            return (
              <button
                key={c.id ?? i}
                type="button"
                onClick={() => openDetail(c)}
                className="w-full flex items-center gap-3 px-5 py-2.5 text-left hover:bg-[var(--bg-elevated)] transition-colors"
              >
                <span className={`w-2 h-2 rounded-full shrink-0 ${solved ? "bg-[var(--success)]" : "bg-[var(--warning)]"}`} title={statusLabel} />
                <span className="min-w-0 flex-1">
                  <span className="block text-sm text-[var(--text-primary)] truncate">{c.title || c.chamado_id || "–"}</span>
                  <span className="flex gap-3 text-2xs text-[var(--text-muted)] truncate">
                    <span className="font-mono">{c.chamado_id || "–"}</span>
                    <span className="truncate">{c.user_display_name || "–"}</span>
                  </span>
                </span>
                <span className="text-2xs text-[var(--text-muted)] shrink-0">{statusLabel}</span>
                <span className="text-2xs text-[var(--text-muted)] font-mono tabular-nums shrink-0">{c.date || "–"}</span>
              </button>
            );
          })}
        </div>
      </SectionCard>

      <ChamadoDrawer
        open={drawerOpen}
        onClose={() => { setDrawerOpen(false); setSelectedChamado(null); }}
        chamado={selectedChamado}
        users={users}
        onCreate={(data) => createMutation.mutate(data)}
        onUpdate={(id, data) => updateMutation.mutate({ id, ...data })}
        onDelete={async (id) => { if (await confirm({ title: t("chamado.deleteConfirm"), danger: true, confirmLabel: t("common.delete") })) deleteMutation.mutate(id); }}
        loading={createMutation.isPending || updateMutation.isPending}
        t={t}
        slug={slug}
      />

      <GlpiHostTicketsBlock slug={slug} />
    </>
  );
}
