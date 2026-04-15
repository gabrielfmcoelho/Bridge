"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { roleMappingsAPI } from "@/lib/api";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";

const PROVIDERS = ["ldap", "keycloak", "gitlab"];
const ROLES = ["viewer", "editor", "admin"];

export default function RoleMappingsTab() {
  const queryClient = useQueryClient();
  const { data: mappings = [], isLoading } = useQuery({
    queryKey: ["role-mappings"],
    queryFn: roleMappingsAPI.list,
  });

  const [newMapping, setNewMapping] = useState({
    provider_name: "keycloak",
    external_group: "",
    local_role: "viewer",
  });

  const createMutation = useMutation({
    mutationFn: roleMappingsAPI.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["role-mappings"] });
      setNewMapping({ provider_name: "keycloak", external_group: "", local_role: "viewer" });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: roleMappingsAPI.delete,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["role-mappings"] }),
  });

  const handleCreate = () => {
    if (!newMapping.external_group.trim()) return;
    createMutation.mutate(newMapping);
  };

  if (isLoading) {
    return (
      <Card>
        <div className="animate-pulse space-y-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-10 bg-[var(--bg-elevated)] rounded-[var(--radius-md)]" />
          ))}
        </div>
      </Card>
    );
  }

  return (
    <Card>
      <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-4">External Role Mappings</h3>
      <p className="text-xs text-[var(--text-muted)] mb-4">
        Map external groups/roles from auth providers to local roles. When a user logs in via an external provider, their groups are checked against these mappings to assign a local role.
      </p>

      {/* Existing mappings */}
      {mappings.length > 0 ? (
        <div className="overflow-x-auto mb-4">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--border-subtle)]">
                <th className="text-left py-2 px-2 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">
                  Provider
                </th>
                <th className="text-left py-2 px-2 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">
                  External Group
                </th>
                <th className="text-left py-2 px-2 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">
                  Local Role
                </th>
                <th className="w-10" />
              </tr>
            </thead>
            <tbody>
              {mappings.map((m) => (
                <tr key={m.id} className="border-b border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors">
                  <td className="py-2 px-2">
                    <span className="inline-flex items-center gap-1.5 text-[var(--text-secondary)]">
                      <span className="w-2 h-2 rounded-full" style={{
                        backgroundColor: m.provider_name === "keycloak" ? "#22c55e" : m.provider_name === "gitlab" ? "#e24329" : "#3b82f6"
                      }} />
                      {m.provider_name}
                    </span>
                  </td>
                  <td className="py-2 px-2 font-mono text-xs text-[var(--text-secondary)]">
                    {m.external_group}
                  </td>
                  <td className="py-2 px-2">
                    <span className={`inline-block px-2 py-0.5 rounded text-[11px] font-medium border ${
                      m.local_role === "admin"
                        ? "bg-cyan-500/15 text-cyan-400 border-cyan-500/25"
                        : m.local_role === "editor"
                        ? "bg-purple-500/15 text-purple-400 border-purple-500/25"
                        : "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]"
                    }`}>
                      {m.local_role}
                    </span>
                  </td>
                  <td className="py-2 px-2">
                    <button
                      onClick={() => deleteMutation.mutate(m.id)}
                      className="text-[var(--text-faint)] hover:text-red-400 transition-colors p-1"
                      title="Delete mapping"
                    >
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="text-center py-6 text-sm text-[var(--text-muted)] mb-4">
          No role mappings configured yet.
        </div>
      )}

      {/* Add new mapping */}
      <div className="border-t border-[var(--border-subtle)] pt-4">
        <p className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">
          Add Mapping
        </p>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3 items-end">
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">Provider</label>
            <select
              value={newMapping.provider_name}
              onChange={(e) => setNewMapping((p) => ({ ...p, provider_name: e.target.value }))}
              className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
            >
              {PROVIDERS.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
          </div>
          <Input
            label="External Group / Role"
            value={newMapping.external_group}
            onChange={(e) => setNewMapping((p) => ({ ...p, external_group: e.target.value }))}
            placeholder="e.g., admin-group"
          />
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">Local Role</label>
            <select
              value={newMapping.local_role}
              onChange={(e) => setNewMapping((p) => ({ ...p, local_role: e.target.value }))}
              className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)]"
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>{r}</option>
              ))}
            </select>
          </div>
          <Button
            onClick={handleCreate}
            loading={createMutation.isPending}
            disabled={!newMapping.external_group.trim()}
          >
            Add
          </Button>
        </div>
      </div>
    </Card>
  );
}
