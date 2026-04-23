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

interface Props {
  projectId: number;
  projectName: string;
  profileID: number | null;
  canEdit: boolean;
}

// ChamadosTab shows all open GLPI tickets for the project (scoped by the
// project's GLPI profile + entity + optional category).
export default function ChamadosTab({ projectId, projectName, profileID, canEdit }: Props) {
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
        title="No GLPI profile linked"
        description="Link a GLPI token profile in the project edit drawer → Vínculos. Admins manage profiles in Settings → Integrations → GLPI."
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
          <p className="text-xs text-[var(--text-muted)]">GLPI chamados abertos</p>
          <p className="text-sm text-[var(--text-primary)]">{tickets.length} chamado{tickets.length === 1 ? "" : "s"}</p>
        </div>
        {canEdit && (
          <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
            Novo chamado
          </Button>
        )}
      </Card>

      {warning && warning !== "no_profile_linked" && (
        <div className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300 text-[11px] px-3 py-2">
          {warning}
        </div>
      )}

      <TicketList tickets={tickets} emptyLabel="Nenhum chamado aberto para este projeto." />

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
