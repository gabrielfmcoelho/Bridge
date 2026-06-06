"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { hostsAPI, servicesAPI, projectsAPI } from "@/lib/api";
import type { Host, Service, Project } from "@/lib/types";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";

// Unified trash (R3 soft-delete). Hosts/services/projects deleted via the app
// are soft-deleted (cascade also soft-deletes their secrets); this page lists
// the soft-deleted rows and lets an admin restore them — restore cascade-undoes
// the child secrets too. A host's slug stays reserved while it sits here, so
// restoring is how you bring back a host whose slug you want to keep.
export default function TrashPage() {
  const qc = useQueryClient();

  const hosts = useQuery({ queryKey: ["hosts-trash"], queryFn: hostsAPI.trash });
  const services = useQuery({ queryKey: ["services-trash"], queryFn: servicesAPI.trash });
  const projects = useQuery({ queryKey: ["projects-trash"], queryFn: projectsAPI.trash });

  const restoreHost = useMutation({
    mutationFn: (id: number) => hostsAPI.restore(id),
    onSuccess: () => {
      hosts.refetch();
      qc.invalidateQueries({ queryKey: ["hosts"] });
    },
  });
  const restoreService = useMutation({
    mutationFn: (id: number) => servicesAPI.restore(id),
    onSuccess: () => {
      services.refetch();
      qc.invalidateQueries({ queryKey: ["services"] });
    },
  });
  const restoreProject = useMutation({
    mutationFn: (id: number) => projectsAPI.restore(id),
    onSuccess: () => {
      projects.refetch();
      qc.invalidateQueries({ queryKey: ["projects"] });
    },
  });

  const total =
    (hosts.data?.length ?? 0) + (services.data?.length ?? 0) + (projects.data?.length ?? 0);
  const loading = hosts.isLoading || services.isLoading || projects.isLoading;

  return (
    <PageShell>
      <PageHeader title="Trash" />
      <p className="text-sm text-[var(--text-muted)] mb-4">
        Soft-deleted hosts, services and projects. Restoring one also brings back
        the secrets that were cascade-deleted with it. Admin only.
      </p>

      {loading ? (
        <p className="text-sm text-[var(--text-muted)]">Loading...</p>
      ) : total === 0 ? (
        <EmptyState
          icon="server"
          title="Trash is empty"
          description="Deleted hosts, services and projects you can restore appear here."
        />
      ) : (
        <div className="space-y-6">
          <TrashSection<Host>
            title="Hosts"
            items={hosts.data ?? []}
            restoring={restoreHost.isPending}
            onRestore={(h) => restoreHost.mutate(h.id)}
            label={(h) => h.nickname || h.oficial_slug}
            meta={(h) => h.oficial_slug}
          />
          <TrashSection<Service>
            title="Services"
            items={services.data ?? []}
            restoring={restoreService.isPending}
            onRestore={(s) => restoreService.mutate(s.id)}
            label={(s) => s.nickname}
            meta={(s) => s.service_type}
          />
          <TrashSection<Project>
            title="Projects"
            items={projects.data ?? []}
            restoring={restoreProject.isPending}
            onRestore={(p) => restoreProject.mutate(p.id)}
            label={(p) => p.name}
            meta={(p) => p.situacao}
          />
        </div>
      )}
    </PageShell>
  );
}

function TrashSection<T extends { id: number }>({
  title,
  items,
  onRestore,
  restoring,
  label,
  meta,
}: {
  title: string;
  items: T[];
  onRestore: (item: T) => void;
  restoring: boolean;
  label: (item: T) => string;
  meta: (item: T) => string;
}) {
  if (items.length === 0) return null;
  return (
    <div>
      <h2 className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] mb-2">
        {title} <span className="text-[var(--text-faint)]">({items.length})</span>
      </h2>
      <div className="space-y-2">
        {items.map((item) => (
          <Card key={item.id}>
            <div className="flex items-center gap-3">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-medium text-sm text-[var(--text-primary)] truncate">
                    {label(item)}
                  </span>
                  {meta(item) ? <Badge>{meta(item)}</Badge> : null}
                </div>
              </div>
              <Button size="sm" onClick={() => onRestore(item)} disabled={restoring}>
                {restoring ? "..." : "Restore"}
              </Button>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
