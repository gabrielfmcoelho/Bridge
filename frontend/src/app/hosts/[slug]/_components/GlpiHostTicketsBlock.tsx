"use client";

import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { glpiAPI, integrationsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import StatusAlert from "@/components/ui/StatusAlert";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import { ICON_PATHS } from "@/lib/icon-paths";
import TicketList from "@/components/glpi/TicketList";

interface Props {
  slug: string;
}

// GlpiHostTicketsBlock surfaces any GLPI ticket linked to a Computer asset whose
// name matches this host's oficial_slug. Appears below the sshcm-local chamados
// list. A profile picker appears when more than one is configured; otherwise
// the first profile is used automatically.
export default function GlpiHostTicketsBlock({ slug }: Props) {
  const { t } = useLocale();
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

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["host-glpi-tickets", slug, profileID],
    queryFn: () => glpiAPI.hostTickets(slug, profileID!),
    enabled: glpiEnabled && !!profileID,
    retry: false,
  });

  if (!glpiEnabled) return null;
  if ((profiles?.length ?? 0) === 0) return null;

  const profileSelect = (profiles?.length ?? 0) > 1 && (
    <ToolbarSelect
      name={t("chamado.profileLabel")}
      icon={ICON_PATHS.user}
      value={profileID != null ? String(profileID) : ""}
      options={(profiles ?? []).map((p) => ({ value: String(p.id), label: p.name }))}
      onChange={(v) => setProfileID(v ? parseInt(v, 10) : null)}
    />
  );

  return (
    <SectionCard
      title={t("host.glpiTicketsTitle")}
      as="h3"
      controls={profileSelect || undefined}
      className="mt-6"
      empty={
        isError ? undefined
        : isLoading ? t("host.glpiQuerying")
        : data?.computer == null ? `${t("host.glpiNoComputerBefore")} ${slug}${t("host.glpiNoComputerAfter")}`
        : undefined
      }
    >
      {isError ? (
        <StatusAlert variant="error">{t("glpi.ticketLoadError", { message: (error as Error)?.message ?? "" })}</StatusAlert>
      ) : data?.computer == null ? null : (
        <>
          <div className="mb-3 flex items-center justify-between gap-3 flex-wrap">
            <div className="min-w-0">
              <p className="text-xs text-[var(--text-muted)]">GLPI Computer</p>
              <p className="text-sm font-mono text-[var(--text-primary)] truncate">
                #{data.computer.id} · {data.computer.name}
              </p>
            </div>
            <span className="text-2xs text-[var(--text-faint)]">
              {t("host.glpiOpenTicketsCount", { n: String(data.tickets.length) })}
            </span>
          </div>
          {data.warning && (
            <StatusAlert variant="warning" className="mb-2">{data.warning}</StatusAlert>
          )}
          <TicketList tickets={data.tickets} emptyLabel={t("host.glpiNoOpenTickets")} />
        </>
      )}
    </SectionCard>
  );
}
