"use client";

import { useQuery } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import Drawer from "@/components/ui/Drawer";

interface HistoryDrawerProps {
  secretID: number | null;
  onClose: () => void;
}

// HistoryDrawer renders the audit-log timeline for one secret. Each row
// is one action (create / update / reveal / delete / restore /
// share_*); for `update` actions the metadata.changed_fields list is
// surfaced inline so operators can see what changed without seeing the
// values (D3b — names only).
export default function HistoryDrawer({ secretID, onClose }: HistoryDrawerProps) {
  const open = secretID != null;

  const { data: rows = [], isLoading } = useQuery({
    queryKey: ["secret-history", secretID],
    queryFn: () => (secretID ? secretsAPI.history(secretID) : Promise.resolve([])),
    enabled: open,
  });

  return (
    <Drawer open={open} onClose={onClose} title="History">
      {isLoading ? (
        <p className="text-sm text-[var(--text-muted)]">Loading...</p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-[var(--text-faint)]">No audit entries.</p>
      ) : (
        <div className="space-y-2">
          {rows.map((r) => (
            <div
              key={r.id}
              className="text-xs p-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)]"
            >
              <div className="flex items-center justify-between gap-2">
                <span className={`font-medium ${actionColor(r.action)}`}>{r.action}</span>
                <span className="text-[10px] text-[var(--text-faint)]">
                  {new Date(r.at).toLocaleString()}
                </span>
              </div>
              {r.actor_user_id && (
                <p className="text-[10px] text-[var(--text-muted)] mt-0.5">
                  actor: user #{r.actor_user_id}
                </p>
              )}
              {r.action === "update" && hasChangedFields(r.metadata) && (
                <p className="text-[10px] text-[var(--text-secondary)] mt-1">
                  changed: {(r.metadata as { changed_fields: string[] }).changed_fields.join(", ")}
                </p>
              )}
              {(r.action === "create" || r.action === "delete" || r.action === "restore") &&
                hasCascadeFrom(r.metadata) && (
                  <p className="text-[10px] text-[var(--text-muted)] mt-1">
                    cascade from {(r.metadata as { cascade_from: string }).cascade_from}
                  </p>
                )}
            </div>
          ))}
        </div>
      )}
    </Drawer>
  );
}

function actionColor(a: string): string {
  switch (a) {
    case "create":
      return "text-emerald-400";
    case "update":
      return "text-amber-400";
    case "reveal":
      return "text-blue-400";
    case "delete":
      return "text-red-400";
    case "restore":
      return "text-emerald-400";
    case "share_create":
    case "share_redeem":
      return "text-purple-400";
    case "share_revoke":
      return "text-red-400";
    default:
      return "text-[var(--text-secondary)]";
  }
}

// hasChangedFields / hasCascadeFrom are tiny type narrows — the metadata
// field is `unknown` from the API client type, so we runtime-check before
// rendering. Skipping these on malformed/older rows is intentional: render
// the row stripped down rather than crashing the drawer.
function hasChangedFields(m: unknown): boolean {
  if (m == null || typeof m !== "object") return false;
  const cf = (m as { changed_fields?: unknown }).changed_fields;
  return Array.isArray(cf) && cf.length > 0;
}
function hasCascadeFrom(m: unknown): boolean {
  if (m == null || typeof m !== "object") return false;
  return typeof (m as { cascade_from?: unknown }).cascade_from === "string";
}
