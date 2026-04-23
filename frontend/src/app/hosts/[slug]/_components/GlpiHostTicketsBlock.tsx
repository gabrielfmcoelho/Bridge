"use client";

import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { glpiAPI, integrationsAPI } from "@/lib/api";
import Card from "@/components/ui/Card";
import TicketList from "@/components/glpi/TicketList";

interface Props {
  slug: string;
}

// GlpiHostTicketsBlock surfaces any GLPI ticket linked to a Computer asset whose
// name matches this host's oficial_slug. Appears below the sshcm-local chamados
// list. A profile picker appears when more than one is configured; otherwise
// the first profile is used automatically.
export default function GlpiHostTicketsBlock({ slug }: Props) {
  const [profileID, setProfileID] = useState<number | null>(null);

  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const glpiEnabled = integrations?.glpi?.glpi_enabled === "true";

  const { data: profiles } = useQuery({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    enabled: glpiEnabled,
    retry: false,
  });

  useEffect(() => {
    if (profiles && profiles.length > 0 && profileID == null) {
      setProfileID(profiles[0].id);
    }
  }, [profiles, profileID]);

  const { data, isLoading } = useQuery({
    queryKey: ["host-glpi-tickets", slug, profileID],
    queryFn: () => glpiAPI.hostTickets(slug, profileID!),
    enabled: glpiEnabled && !!profileID,
    retry: false,
  });

  if (!glpiEnabled) return null;
  if ((profiles?.length ?? 0) === 0) return null;

  return (
    <div className="mt-6">
      <div className="flex items-center justify-between mb-3 gap-3 flex-wrap">
        <h3 className="text-sm font-semibold text-[var(--text-secondary)] tracking-wide uppercase">
          Tickets GLPI (asset)
        </h3>
        {(profiles?.length ?? 0) > 1 && (
          <select
            value={profileID ?? ""}
            onChange={(e) => setProfileID(e.target.value ? parseInt(e.target.value, 10) : null)}
            className="bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-2 py-1 text-xs"
          >
            {(profiles ?? []).map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        )}
      </div>

      {isLoading ? (
        <Card hover={false} className="!p-3">
          <p className="text-xs text-[var(--text-muted)] animate-pulse">Querying GLPI…</p>
        </Card>
      ) : data?.computer == null ? (
        <Card hover={false} className="!p-3">
          <p className="text-xs text-[var(--text-muted)]">
            No GLPI Computer asset matched host slug <code className="font-mono text-[var(--text-secondary)]">{slug}</code>.
            Create one in GLPI with the same name to pull its tickets here.
          </p>
        </Card>
      ) : (
        <>
          <Card hover={false} className="!p-3 mb-2 flex items-center justify-between gap-3 flex-wrap">
            <div className="min-w-0">
              <p className="text-xs text-[var(--text-muted)]">GLPI Computer</p>
              <p className="text-sm font-mono text-[var(--text-primary)] truncate">
                #{data.computer.id} · {data.computer.name}
              </p>
            </div>
            <span className="text-[10px] text-[var(--text-faint)]">
              {data.tickets.length} ticket{data.tickets.length === 1 ? "" : "s"} aberto{data.tickets.length === 1 ? "" : "s"}
            </span>
          </Card>
          {data.warning && (
            <div className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300 text-[11px] px-3 py-2 mb-2">
              {data.warning}
            </div>
          )}
          <TicketList tickets={data.tickets} emptyLabel="Nenhum ticket GLPI aberto para este asset." />
        </>
      )}
    </div>
  );
}
