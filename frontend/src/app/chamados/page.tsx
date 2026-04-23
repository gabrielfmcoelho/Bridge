"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectsAPI, glpiAPI, integrationsAPI, type GlpiTicketSummary } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import Link from "next/link";
import TicketList from "@/components/glpi/TicketList";
import CreateTicketModal from "@/components/glpi/CreateTicketModal";
import TicketDetailDrawer from "@/components/glpi/TicketDetailDrawer";

// Global /chamados — lists GLPI tickets across every project the profile covers.
// The page iterates known projects with a glpi_token_id and aggregates their
// tickets; a profile filter lets the operator scope to one account at a time.
export default function ChamadosPage() {
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [createOpen, setCreateOpen] = useState(false);
  const [profileFilter, setProfileFilter] = useState<number | "all">("all");
  // Which ticket the detail drawer is showing. Resolves profile from the
  // active filter (specific profile) or falls back to the first configured
  // profile when "all" is selected.
  const [detailTicketID, setDetailTicketID] = useState<number | null>(null);
  // "projects" = aggregate per sshcm project (original behavior).
  // "profile"  = dump every ticket visible to the selected profile, ignoring
  //              project linkage. Requires a specific profile to be selected.
  const [viewMode, setViewMode] = useState<"projects" | "profile">("projects");
  const [includeClosed, setIncludeClosed] = useState(false);

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
    retry: false,
    enabled: glpiEnabled,
  });

  const { data: projects } = useQuery({
    queryKey: ["projects"],
    queryFn: projectsAPI.list,
    enabled: glpiEnabled,
  });

  // Compose a dedup'd list of (projectID, projectName, profileID) to query.
  const projectsWithProfile = (projects ?? [])
    .filter((p) => p.glpi_token_id != null)
    .filter((p) => profileFilter === "all" || p.glpi_token_id === profileFilter);

  // Profile-scoped ticket dump — used when viewMode === "profile".
  // "All profiles" fans out to every configured profile and merges results
  // (dedup'd by ticket id, since the same ticket can be visible to multiple).
  const profileTickets = useQuery({
    queryKey: [
      "profile-chamados",
      profileFilter,
      includeClosed,
      (profiles ?? []).map((p) => p.id).sort().join(","),
    ],
    enabled:
      glpiEnabled &&
      viewMode === "profile" &&
      (profileFilter === "all" ? (profiles?.length ?? 0) > 0 : profileFilter > 0),
    queryFn: async () => {
      const targetIDs =
        profileFilter === "all"
          ? (profiles ?? []).map((p) => p.id)
          : [profileFilter as number];
      const results = await Promise.all(
        targetIDs.map((id) =>
          glpiAPI
            .profileTickets(id, { includeClosed })
            .then((r) => ({ id, tickets: r.tickets, warning: undefined as string | undefined }))
            .catch((err) => ({
              id,
              tickets: [],
              warning: err instanceof Error ? err.message : "unknown error",
            }))
        )
      );
      // Dedup by ticket id (same ticket may surface under several profiles).
      const seen = new Set<number>();
      const merged: GlpiTicketSummary[] = [];
      for (const r of results) {
        for (const t of r.tickets) {
          if (seen.has(t.id)) continue;
          seen.add(t.id);
          merged.push(t);
        }
      }
      merged.sort((a, b) => (b.date ?? "").localeCompare(a.date ?? ""));
      const warnings = results.filter((r) => r.warning).map((r) => `profile ${r.id}: ${r.warning}`);
      return { tickets: merged, count: merged.length, warnings };
    },
    retry: false,
  });

  // Run one query per project. useQueries would be cleaner — but the aggregated
  // count is usually small and repeating useQuery in a loop is simpler to read.
  const ticketsByProject = useQuery({
    queryKey: ["global-chamados", projectsWithProfile.map((p) => p.id).sort()],
    enabled: glpiEnabled && viewMode === "projects" && projectsWithProfile.length > 0,
    queryFn: async () => {
      const results = await Promise.all(
        projectsWithProfile.map(async (p) => {
          try {
            const r = await glpiAPI.projectTickets(p.id);
            return { project: p, tickets: r.tickets, warning: r.warning };
          } catch (err) {
            return {
              project: p,
              tickets: [],
              warning: err instanceof Error ? err.message : "unknown error",
            };
          }
        })
      );
      return results;
    },
  });

  const body = (() => {
    if (!glpiEnabled) {
      return (
        <EmptyState
          icon="box"
          title="GLPI integration is disabled"
          description="Ask an admin to enable it in Settings → Integrations → GLPI."
        />
      );
    }

    if (viewMode === "profile") {
      if ((profiles?.length ?? 0) === 0) {
        return (
          <EmptyState
            icon="folder"
            title="No profiles configured"
            description="Add a GLPI profile in Settings → Integrations → GLPI → Token profiles to use this view."
          />
        );
      }
      if (profileTickets.isLoading) {
        return (
          <div className="space-y-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-[var(--radius-md)]" />
            ))}
          </div>
        );
      }
      if (profileTickets.isError) {
        return (
          <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-sm px-4 py-3">
            Falha: {(profileTickets.error as Error).message}
          </div>
        );
      }
      const tickets = profileTickets.data?.tickets ?? [];
      const warnings = profileTickets.data?.warnings ?? [];
      const scopeLabel =
        profileFilter === "all"
          ? `${(profiles ?? []).length} perfis`
          : (profiles ?? []).find((p) => p.id === profileFilter)?.name ?? "perfil";
      return (
        <div className="space-y-3">
          <p className="text-xs text-[var(--text-muted)]">
            {tickets.length} chamado{tickets.length === 1 ? "" : "s"}
            {" "}· {scopeLabel}
            {" "}· {includeClosed ? "abertos e fechados" : "apenas abertos"}
          </p>
          {warnings.length > 0 && (
            <div className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300 text-[11px] px-3 py-2 space-y-1">
              {warnings.map((w, i) => (
                <p key={i}>{w}</p>
              ))}
            </div>
          )}
          <TicketList
            tickets={tickets}
            emptyLabel="Sem chamados para este perfil."
            onOpenDetails={(id) => setDetailTicketID(id)}
          />
        </div>
      );
    }

    if (ticketsByProject.isLoading) {
      return (
        <div className="space-y-3">
          <Skeleton className="h-10 w-full rounded-[var(--radius-md)]" />
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-20 w-full rounded-[var(--radius-md)]" />
          ))}
        </div>
      );
    }
    if (!projectsWithProfile.length) {
      return (
        <EmptyState
          icon="folder"
          title="No projects linked to GLPI"
          description="Edit any project and link a GLPI profile in the Vínculos section."
        />
      );
    }
    const groups = ticketsByProject.data ?? [];
    const totalOpen = groups.reduce((n, g) => n + g.tickets.length, 0);
    return (
      <div className="space-y-5">
        <p className="text-xs text-[var(--text-muted)]">
          {totalOpen} chamado{totalOpen === 1 ? "" : "s"} aberto{totalOpen === 1 ? "" : "s"} · {groups.length} projeto{groups.length === 1 ? "" : "s"}
        </p>
        {groups.map((g) => (
          <div key={g.project.id} className="space-y-2">
            <div className="flex items-center justify-between">
              <p className="text-sm font-semibold text-[var(--text-primary)]">{g.project.name}</p>
              <p className="text-[10px] text-[var(--text-faint)]">
                {g.tickets.length} aberto{g.tickets.length === 1 ? "" : "s"}
              </p>
            </div>
            {g.warning && (
              <div className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300 text-[11px] px-3 py-2">
                {g.warning}
              </div>
            )}
            <TicketList
              tickets={g.tickets}
              emptyLabel="Sem chamados."
              onOpenDetails={(id) => setDetailTicketID(id)}
            />
          </div>
        ))}
      </div>
    );
  })();

  return (
    <PageShell>
      <PageHeader title="Chamados" />
      <Card hover={false} className="!p-3 mb-4 flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-3 flex-wrap">
          {/* View mode toggle */}
          <div className="inline-flex rounded-[var(--radius-md)] border border-[var(--border-default)] overflow-hidden">
            <button
              type="button"
              onClick={() => setViewMode("projects")}
              className={`px-3 py-1 text-xs font-medium ${
                viewMode === "projects"
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              By project
            </button>
            <button
              type="button"
              onClick={() => setViewMode("profile")}
              className={`px-3 py-1 text-xs font-medium border-l border-[var(--border-default)] ${
                viewMode === "profile"
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              All tickets (profile)
            </button>
          </div>
          <label className="text-xs text-[var(--text-muted)]">Profile</label>
          <select
            value={profileFilter === "all" ? "all" : String(profileFilter)}
            onChange={(e) => setProfileFilter(e.target.value === "all" ? "all" : parseInt(e.target.value, 10))}
            className="bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-2 py-1 text-sm"
            disabled={!glpiEnabled}
          >
            <option value="all">All profiles</option>
            {(profiles ?? []).map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          {viewMode === "profile" && (
            <label className="text-xs text-[var(--text-muted)] inline-flex items-center gap-1.5">
              <input
                type="checkbox"
                checked={includeClosed}
                onChange={(e) => setIncludeClosed(e.target.checked)}
              />
              Incluir fechados
            </label>
          )}
        </div>
        <div className="flex items-center gap-2">
          {glpiEnabled && (
            <Link
              href="/chamados/forms"
              className="inline-flex items-center gap-1 px-3 py-1 text-xs font-medium rounded-[var(--radius-md)] border border-[var(--border-default)] text-[var(--text-secondary)] hover:text-[var(--accent)] hover:border-[var(--accent)]/40 transition-colors"
            >
              Formulários
            </Link>
          )}
          {canEdit && glpiEnabled && (
            <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
              Novo chamado
            </Button>
          )}
        </div>
      </Card>
      {body}
      <CreateTicketModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        defaultProfileID={profileFilter === "all" ? null : profileFilter}
        onCreated={() => queryClient.invalidateQueries({ queryKey: ["global-chamados"] })}
      />

      {(() => {
        // Pick a profile to fetch with: the active filter, or the first
        // configured profile when "all" is selected.
        const detailProfile =
          profileFilter !== "all"
            ? (profileFilter as number)
            : profiles?.[0]?.id ?? null;
        return (
          <TicketDetailDrawer
            open={detailTicketID != null}
            onClose={() => setDetailTicketID(null)}
            ticketID={detailTicketID}
            profileID={detailProfile}
          />
        );
      })()}
    </PageShell>
  );
}
