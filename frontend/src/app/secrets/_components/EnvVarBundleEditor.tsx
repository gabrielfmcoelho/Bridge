"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import type { Secret } from "@/lib/types";
import TabBar from "@/components/ui/TabBar";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import EmptyState from "@/components/ui/EmptyState";

interface EnvVarBundleEditorProps {
  // Where the bundle is attached. avulso means no parent.
  scope: "service" | "host" | "tool" | "projeto" | "avulso";
  parentID?: number;
  // Restrict the editor to one visibility — typically "shared" for service
  // env_var bundles; the personal counterpart can be embedded separately.
  visibility?: "shared" | "personal";
}

// Working-set entry — mirrors the bulk payload shape. `id` is set when this
// var originated from the server (so it can be deleted by id); blank for
// newly-added rows that exist only client-side until the next save.
interface VarDraft {
  id?: number;
  name: string;
  value: string;
  description?: string;
  // dirty when the user edited an existing row (so we know to include it
  // in the bulk save — unedited rows could be skipped to keep audit noise
  // down, but the bulk endpoint is upsert-only so a no-op resend is fine).
  dirty?: boolean;
}

// EnvVarBundleEditor renders a tabbed editor over env_var bundles grouped
// by `group_label` (Phase 2 Task 2.5). Each tab is one environment; adding
// a new tab creates a fresh group. Save flushes the current tab via the
// /api/secrets/env/bulk endpoint. Per-row delete hits /api/secrets/{id}.
export default function EnvVarBundleEditor({
  scope,
  parentID,
  visibility = "shared",
}: EnvVarBundleEditorProps) {
  const qc = useQueryClient();

  // Fetch the grouped bundle map for this scope+parent.
  const { data: grouped = {}, isLoading } = useQuery({
    queryKey: ["env-secrets", scope, parentID],
    queryFn: () => secretsAPI.envList({ scope, parent_id: parentID }),
  });

  const groupKeys = useMemo(() => Object.keys(grouped).sort(), [grouped]);
  const [activeGroup, setActiveGroup] = useState<string>("");

  // Pick a default tab when data arrives or the previously-active tab
  // disappears (e.g. all its vars were deleted in another tab).
  useEffect(() => {
    if (activeGroup && groupKeys.includes(activeGroup)) return;
    if (groupKeys.length > 0) setActiveGroup(groupKeys[0]);
  }, [groupKeys, activeGroup]);

  // Per-tab draft state. We materialize from server data the first time a
  // tab is opened and then let the user edit locally until Save.
  const [drafts, setDrafts] = useState<Record<string, VarDraft[]>>({});

  useEffect(() => {
    setDrafts((prev) => {
      const next = { ...prev };
      for (const [k, secrets] of Object.entries(grouped)) {
        if (next[k] === undefined) {
          next[k] = secrets.map(secretToDraft);
        }
      }
      return next;
    });
  }, [grouped]);

  const activeDraft = drafts[activeGroup] || [];
  const setActiveDraft = (rows: VarDraft[]) =>
    setDrafts((prev) => ({ ...prev, [activeGroup]: rows }));

  const saveMut = useMutation({
    mutationFn: async () => {
      if (!activeGroup) return;
      const vars = activeDraft
        .filter((d) => d.name && d.value)
        .map((d) => ({ name: d.name, value: d.value, description: d.description || undefined }));
      if (vars.length === 0) return;
      await secretsAPI.envBulk({
        scope,
        parent_id: parentID,
        visibility,
        group_label: activeGroup,
        vars,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["env-secrets", scope, parentID] });
    },
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => secretsAPI.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["env-secrets", scope, parentID] });
    },
  });

  const addNewGroup = () => {
    const name = prompt("New environment label (lowercase, e.g. prod, staging):");
    if (!name) return;
    setActiveGroup(name);
    setDrafts((prev) => ({ ...prev, [name]: [] }));
  };

  const addVarRow = () => {
    setActiveDraft([...activeDraft, { name: "", value: "", dirty: true }]);
  };

  const updateRow = (idx: number, patch: Partial<VarDraft>) => {
    setActiveDraft(activeDraft.map((d, i) => (i === idx ? { ...d, ...patch, dirty: true } : d)));
  };

  const removeRow = (idx: number) => {
    const row = activeDraft[idx];
    if (row.id != null) {
      // Server-backed row -> delete on the server.
      deleteMut.mutate(row.id);
    }
    setActiveDraft(activeDraft.filter((_, i) => i !== idx));
  };

  if (isLoading) {
    return <p className="text-sm text-[var(--text-muted)]">Loading...</p>;
  }

  const tabItems = groupKeys.map((g) => ({ key: g, label: g }));

  return (
    <Card>
      <div className="flex items-center justify-between mb-4 gap-3">
        {tabItems.length > 0 ? (
          <TabBar tabs={tabItems} activeTab={activeGroup} onChange={setActiveGroup} />
        ) : (
          <span className="text-sm text-[var(--text-muted)]">No environments yet</span>
        )}
        <Button size="sm" variant="secondary" onClick={addNewGroup}>
          + Environment
        </Button>
      </div>

      {!activeGroup ? (
        <EmptyState
          icon="key"
          title="No environments"
          description="Add an environment label (e.g. prod, staging) to start storing env vars."
        />
      ) : (
        <div className="space-y-3">
          <div className="space-y-2">
            {activeDraft.length === 0 ? (
              <p className="text-sm text-[var(--text-faint)]">No vars in this environment yet.</p>
            ) : (
              activeDraft.map((d, idx) => (
                <div key={d.id ?? `new-${idx}`} className="grid grid-cols-12 gap-2 items-start">
                  <div className="col-span-4">
                    <Input
                      value={d.name}
                      onChange={(e) => updateRow(idx, { name: e.target.value.toUpperCase() })}
                      placeholder="DB_URL"
                    />
                  </div>
                  <div className="col-span-6">
                    <Input
                      type="password"
                      value={d.value}
                      onChange={(e) => updateRow(idx, { value: e.target.value })}
                      placeholder="value"
                    />
                  </div>
                  <div className="col-span-2 flex justify-end">
                    <Button size="sm" variant="ghost" onClick={() => removeRow(idx)}>
                      Remove
                    </Button>
                  </div>
                  {d.description !== undefined && (
                    <div className="col-span-12">
                      <Input
                        value={d.description || ""}
                        onChange={(e) => updateRow(idx, { description: e.target.value })}
                        placeholder="Description (optional)"
                      />
                    </div>
                  )}
                </div>
              ))
            )}
          </div>

          <div className="flex items-center gap-2 pt-2">
            <Button size="sm" variant="secondary" onClick={addVarRow}>
              + Var
            </Button>
            <Button
              size="sm"
              onClick={() => saveMut.mutate()}
              disabled={saveMut.isPending}
            >
              {saveMut.isPending ? "Saving..." : "Save bundle"}
            </Button>
            {saveMut.isError && (
              <span className="text-xs text-red-400">
                Save failed: {(saveMut.error as Error).message}
              </span>
            )}
          </div>
        </div>
      )}
    </Card>
  );
}

function secretToDraft(s: Secret): VarDraft {
  return {
    id: s.id,
    name: s.name,
    value: "", // never echoed from list endpoint; user has to re-enter to edit
    description: s.description || undefined,
  };
}
