"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { glpiAPI } from "@/lib/api";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import TicketList from "@/components/glpi/TicketList";
import CreateTicketModal from "@/components/glpi/CreateTicketModal";
import { useLocale } from "@/contexts/LocaleContext";

interface Props {
  projectId: number;
  projectName: string;
  profileID: number | null;
  canEdit: boolean;
}

// ChamadosTab shows all open GLPI tickets for the project (scoped by the
// project's GLPI profile + entity + optional category).
export default function ChamadosTab({ projectId, projectName, profileID, canEdit }: Props) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["project-glpi-tickets", projectId],
    queryFn: () => glpiAPI.projectTickets(projectId),
    retry: false,
  });

  if (!profileID) {
    return (
      <EmptyState
        icon="folder"
        title={t("chamado.tab.noProfileTitle")}
        description={t("chamado.tab.noProfileDesc")}
        compact
      />
    );
  }

  if (isLoading) {
    return <Skeleton className="h-32 w-full rounded-[var(--radius-md)]" />;
  }

  const tickets = data?.tickets ?? [];
  const warning = data?.warning;

  return (
    <div className="space-y-4 animate-fade-in">
      <Card hover={false} className="!p-3 flex items-center justify-between gap-3">
        <div>
          <p className="text-xs text-[var(--text-muted)]">{t("chamado.tab.openTicketsLabel")}</p>
          <p className="text-sm text-[var(--text-primary)]">{t("chamado.count", { n: String(tickets.length) })}</p>
        </div>
        {canEdit && (
          <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
            {t("chamado.newAction")}
          </Button>
        )}
      </Card>

      {warning && warning !== "no_profile_linked" && (
        <div className="rounded-[var(--radius-md)] border border-[var(--warning)]/30 bg-[var(--warning)]/10 text-[var(--warning)] text-xs px-3 py-2">
          {warning}
        </div>
      )}

      <TicketList tickets={tickets} emptyLabel={t("chamado.tab.emptyOpen")} />

      <CreateTicketModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        defaultProfileID={profileID}
        defaultTitle={`[${projectName}] `}
        onCreated={() => {
          queryClient.invalidateQueries({ queryKey: ["project-glpi-tickets", projectId] });
        }}
      />
    </div>
  );
}
