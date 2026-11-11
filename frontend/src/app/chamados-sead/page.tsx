"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { glpiSeadAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import ChamadosSource from "@/components/chamados/ChamadosSource";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import CreateSeadTicketModal from "@/components/glpiSead/CreateSeadTicketModal";
import SeadTicketDetailDrawer from "@/components/glpiSead/SeadTicketDetailDrawer";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";

export default function ChamadosSeadPage() {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [createOpen, setCreateOpen] = useState(false);
  const [detailID, setDetailID] = useState<number | null>(null);
  const [search, setSearch] = useState("");

  const { data: status } = useQuery({
    queryKey: ["glpi-sead-status"],
    queryFn: glpiSeadAPI.status,
    retry: false,
    staleTime: 60_000,
  });
  const enabled = status?.enabled === true;

  const { data, isLoading, error } = useQuery({
    queryKey: ["glpi-sead-tickets", search],
    queryFn: () => glpiSeadAPI.listTickets({ q: search || undefined, limit: 50 }),
    enabled,
    retry: false,
  });

  const body = (() => {
    if (!enabled) {
      return (
        <EmptyState
          icon="box"
          title={t("chamado.glpiDisabledTitle")}
          description={t("chamados.source.glpiSeadHint")}
        />
      );
    }
    if (isLoading) {
      return (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-[var(--radius-md)]" />
          ))}
        </div>
      );
    }
    if (error) {
      return (
        <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-sm px-4 py-3">
          {(error as Error).message}
        </div>
      );
    }
    const tickets = data?.tickets ?? [];
    if (tickets.length === 0) {
      return <EmptyState icon="folder" title={t("glpi.noTickets")} />;
    }
    return (
      <ul className="space-y-1.5">
        {tickets.map((tk) => (
          <li key={tk.id}>
            <button
              type="button"
              onClick={() => setDetailID(tk.id)}
              className="block w-full text-left rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-3 py-2 hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)] transition-colors"
            >
              <div className="flex items-center gap-3">
                <code className="text-xs text-[var(--text-muted)] shrink-0">#{tk.id}</code>
                <span className="text-sm text-[var(--text-primary)] truncate flex-1">
                  {tk.name || t("glpi.untitled")}
                </span>
                {tk.date && (
                  <span className="shrink-0 text-2xs text-[var(--text-faint)] font-mono">
                    {getTimeAgo(tk.date.replace(" ", "T"), locale)}
                  </span>
                )}
                <span className="shrink-0 text-2xs px-1.5 py-0.5 rounded border border-[var(--border-subtle)] text-[var(--text-muted)]">
                  {tk.status.name}
                </span>
              </div>
            </button>
          </li>
        ))}
      </ul>
    );
  })();

  return (
    <PageShell>
      <PageHeader
        title={t("nav.chamados")}
        subtitle={t("chamados.source.glpiSeadHint")}
        status={<ChamadosSource active="glpi-sead" />}
      />
      <Card hover={false} className="!p-3 mb-4 flex items-center justify-between gap-3 flex-wrap">
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("chamado.forms.searchPlaceholder")}
          className="bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-1 text-sm min-w-[220px]"
          disabled={!enabled}
        />
        {canEdit && enabled && (
          <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
            {t("chamado.newAction")}
          </Button>
        )}
      </Card>
      {body}

      <CreateSeadTicketModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => queryClient.invalidateQueries({ queryKey: ["glpi-sead-tickets"] })}
      />
      <SeadTicketDetailDrawer open={detailID != null} onClose={() => setDetailID(null)} ticketID={detailID} />
    </PageShell>
  );
}
