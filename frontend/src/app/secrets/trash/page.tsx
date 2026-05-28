"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { secretsAPI } from "@/lib/api";
import type { Secret } from "@/lib/types";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import HistoryDrawer from "../_components/HistoryDrawer";

// Trash view (Phase 4 Task 4.5) — shows the soft-deleted secrets the
// caller is allowed to restore. The backend's ListTrash already filters
// to actionable items (see vault.ListTrash) so this page renders the
// list as-is without further visibility checks.
export default function SecretsTrashPage() {
  const qc = useQueryClient();
  const [historyTarget, setHistoryTarget] = useState<number | null>(null);

  const { data: items = [], isLoading, refetch } = useQuery({
    queryKey: ["secrets-trash"],
    queryFn: secretsAPI.trash,
  });

  const restore = useMutation({
    mutationFn: (id: number) => secretsAPI.restore(id),
    onSuccess: () => {
      refetch();
      qc.invalidateQueries({ queryKey: ["secrets-all"] });
    },
  });

  return (
    <PageShell>
      <PageHeader title="Trash" />
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-[var(--text-muted)]">
          Soft-deleted secrets you can restore. Items removed by a cascading parent
          delete are also listed here when the cascade reached you.
        </p>
        <Link href="/secrets" className="text-xs text-[var(--accent)] hover:underline">
          ← Back to vault
        </Link>
      </div>

      {isLoading ? (
        <p className="text-sm text-[var(--text-muted)]">Loading...</p>
      ) : items.length === 0 ? (
        <EmptyState
          icon="key"
          title="Trash is empty"
          description="Nothing to restore. Deleted secrets you can restore appear here."
        />
      ) : (
        <div className="space-y-2">
          {items.map((s) => (
            <TrashRow
              key={s.id}
              secret={s}
              onRestore={() => restore.mutate(s.id)}
              onHistory={() => setHistoryTarget(s.id)}
              restoring={restore.isPending}
            />
          ))}
        </div>
      )}

      <HistoryDrawer secretID={historyTarget} onClose={() => setHistoryTarget(null)} />
    </PageShell>
  );
}

function TrashRow({
  secret,
  onRestore,
  onHistory,
  restoring,
}: {
  secret: Secret;
  onRestore: () => void;
  onHistory: () => void;
  restoring: boolean;
}) {
  return (
    <Card>
      <div className="flex items-start gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-sm text-[var(--text-primary)]">{secret.name}</span>
            <Badge>{secret.type}</Badge>
            <Badge color={secret.visibility === "personal" ? "purple" : "amber"}>
              {secret.visibility}
            </Badge>
            <Badge>{secret.scope}</Badge>
          </div>
          {secret.deleted_at && (
            <p className="text-[10px] text-[var(--text-faint)] mt-1">
              deleted {new Date(secret.deleted_at).toLocaleString()}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="ghost" onClick={onHistory}>
            History
          </Button>
          <Button size="sm" onClick={onRestore} disabled={restoring}>
            {restoring ? "..." : "Restore"}
          </Button>
        </div>
      </div>
    </Card>
  );
}
