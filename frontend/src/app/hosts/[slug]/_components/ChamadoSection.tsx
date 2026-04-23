"use client";

import { useState, useEffect } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { hostChamadosAPI, usersAPI } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import EmptyState from "@/components/ui/EmptyState";
import ChamadoDrawer from "./ChamadoDrawer";
import GlpiHostTicketsBlock from "./GlpiHostTicketsBlock";
import type { HostChamado } from "@/lib/types";

const STATUS_LABELS: Record<string, string> = {
  in_execution: "Em Execução",
  solved: "Resolvido",
};

const STATUS_DOT: Record<string, string> = {
  in_execution: "bg-amber-400",
  solved: "bg-emerald-400",
};

const STATUS_BADGE: Record<string, string> = {
  in_execution: "bg-amber-500/15 text-amber-400 border-amber-500/30",
  solved: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30",
};

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
  const [view, setView] = useState<"cards" | "table">("cards");
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
      <SectionHeading actions={
        <div className="flex items-center gap-2">
          {chamados.length > 0 && (
            <ViewToggle
              value={view}
              onChange={(v) => setView(v as "cards" | "table")}
              options={[
                { key: "cards", label: t("common.cards"), icon: VIEW_ICONS.cards },
                { key: "table", label: t("common.table"), icon: VIEW_ICONS.table },
              ]}
            />
          )}
          {canEdit && (
            <span className="hidden md:contents">
              <Button size="sm" onClick={openCreateDrawer}><span className="mr-1">+</span> {t("host.addChamado")}</Button>
            </span>
          )}
        </div>
      }>
        {t("host.chamados")}
      </SectionHeading>

      {chamados.length === 0 ? (
        <EmptyState icon="search" title={t("host.noChamados")} description={t("host.noChamadosDesc") || "No active tickets for this host."} compact />
      ) : view === "cards" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-2">
          {chamados.map((c, i) => {
            const isSolved = c.status === "solved";
            const statusColor = isSolved ? "#10b981" : "#f59e0b";
            const statusLabel = c.status === "in_execution" ? t("chamado.inExecution") : c.status === "solved" ? t("chamado.solved") : c.status;
            return (
              <Card key={c.id ?? i} onClick={() => openDetail(c)} clickIndicator="drawer" className="!p-3">
                <div className="flex items-center gap-2.5 mb-2">
                  {/* Ticket icon */}
                  <div className="w-8 h-8 rounded-full flex items-center justify-center shrink-0 bg-orange-500/15 text-orange-400">
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M15 5v2m0 4v2m0 4v2M5 5a2 2 0 00-2 2v3a2 2 0 110 4v3a2 2 0 002 2h14a2 2 0 002-2v-3a2 2 0 110-4V7a2 2 0 00-2-2H5z" />
                    </svg>
                  </div>
                  <div className="min-w-0 flex-1">
                    <span className="text-sm font-medium text-[var(--text-primary)] truncate block">
                      {c.title || c.chamado_id || "--"}
                    </span>
                    <span className="text-[10px] text-[var(--text-faint)] truncate block" style={{ fontFamily: "var(--font-mono)" }}>
                      {c.chamado_id || "--"}
                    </span>
                  </div>
                  {/* Expandable status dot — same pattern as host situacao compact badge */}
                  <span className="group/status inline-flex items-center gap-0 rounded-full transition-all duration-300 cursor-default shrink-0">
                    <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: statusColor }} />
                    <span
                      className="max-w-0 overflow-hidden whitespace-nowrap text-xs font-medium opacity-0 group-hover/status:max-w-[120px] group-hover/status:opacity-100 group-hover/status:ml-1.5 group-hover/status:pr-1 transition-all duration-300"
                      style={{ color: statusColor }}
                    >
                      {statusLabel}
                    </span>
                  </span>
                </div>
                <div className="grid grid-cols-2 gap-3 text-xs">
                  <div>
                    <span className="text-[var(--text-faint)] block mb-0.5">{t("host.chamadoUser") || "User"}</span>
                    <span className="text-[var(--text-muted)]">{c.user_display_name || "--"}</span>
                  </div>
                  <div>
                    <span className="text-[var(--text-faint)] block mb-0.5">{t("host.chamadoDate") || "Date"}</span>
                    <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{c.date || "--"}</span>
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      ) : (
        <SortableTable
          columns={[
            { key: "chamado_id" as const, label: "ID" },
            { key: "title" as const, label: t("common.title") },
            { key: "status" as const, label: t("common.status") },
            { key: "user" as const, label: t("host.chamadoUser") || "User" },
            { key: "date" as const, label: t("host.chamadoDate") || "Date" },
          ]}
          defaultSort="date"
          defaultDir="desc"
        >
          {(sk, sd) => {
            const sorted = sortRows(chamados, sk, sd, {
              chamado_id: (a, b) => a.chamado_id.localeCompare(b.chamado_id),
              title: (a, b) => (a.title || "").localeCompare(b.title || ""),
              status: (a, b) => a.status.localeCompare(b.status),
              user: (a, b) => (a.user_display_name || "").localeCompare(b.user_display_name || ""),
              date: (a, b) => a.date.localeCompare(b.date),
            });
            return sorted.map((c, i) => (
              <tr key={c.id ?? i} className={`border-t border-[var(--border-subtle)] cursor-pointer hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`} onClick={() => openDetail(c)}>
                <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{c.chamado_id || "--"}</td>
                <td className="px-4 py-2.5 text-[var(--text-secondary)]">{c.title || "--"}</td>
                <td className="px-4 py-2.5">
                  <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${STATUS_BADGE[c.status] || STATUS_BADGE.in_execution}`}>
                    {c.status === "in_execution" ? t("chamado.inExecution") : c.status === "solved" ? t("chamado.solved") : c.status}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-[var(--text-muted)]">{c.user_display_name || "--"}</td>
                <td className="px-4 py-2.5 text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{c.date || "--"}</td>
              </tr>
            ));
          }}
        </SortableTable>
      )}

      <ChamadoDrawer
        open={drawerOpen}
        onClose={() => { setDrawerOpen(false); setSelectedChamado(null); }}
        chamado={selectedChamado}
        users={users}
        onCreate={(data) => createMutation.mutate(data)}
        onUpdate={(id, data) => updateMutation.mutate({ id, ...data })}
        onDelete={(id) => { if (confirm("Delete this chamado?")) deleteMutation.mutate(id); }}
        loading={createMutation.isPending || updateMutation.isPending}
        t={t}
        slug={slug}
      />

      <GlpiHostTicketsBlock slug={slug} />
    </>
  );
}
