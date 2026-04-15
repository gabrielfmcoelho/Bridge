"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Fragment, useState, useEffect, useMemo } from "react";
import { permissionsAPI } from "@/lib/api";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";

const ROLES = ["viewer", "editor", "admin"] as const;

export default function PermissionsTab() {
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["permissions"], queryFn: permissionsAPI.get });

  // Local state: matrix[role] = Set<permission>
  const [matrix, setMatrix] = useState<Record<string, Set<string>>>({});
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    if (data?.matrix) {
      const m: Record<string, Set<string>> = {};
      for (const role of ROLES) {
        m[role] = new Set(data.matrix[role] ?? []);
      }
      setMatrix(m);
      setDirty(false);
    }
  }, [data]);

  // Group permissions by category.
  const grouped = useMemo(() => {
    if (!data?.permissions) return {};
    const groups: Record<string, { code: string; description: string }[]> = {};
    for (const p of data.permissions) {
      if (!groups[p.category]) groups[p.category] = [];
      groups[p.category].push({ code: p.code, description: p.description });
    }
    return groups;
  }, [data]);

  const toggle = (role: string, permission: string) => {
    if (role === "admin") return; // admin is always full
    setMatrix((prev) => {
      const next = { ...prev };
      const s = new Set(next[role]);
      if (s.has(permission)) {
        s.delete(permission);
      } else {
        s.add(permission);
      }
      next[role] = s;
      return next;
    });
    setDirty(true);
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      // Save viewer and editor (admin is immutable).
      for (const role of ["viewer", "editor"] as const) {
        await permissionsAPI.update(role, Array.from(matrix[role] ?? []));
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["permissions"] });
      setDirty(false);
    },
  });

  if (isLoading) {
    return (
      <Card>
        <div className="animate-pulse space-y-3">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="h-8 bg-[var(--bg-elevated)] rounded-[var(--radius-md)]" />
          ))}
        </div>
      </Card>
    );
  }

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">Role Permissions</h3>
        {dirty && (
          <Button size="sm" onClick={() => saveMutation.mutate()} loading={saveMutation.isPending}>
            Save Changes
          </Button>
        )}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[var(--border-subtle)]">
              <th className="text-left py-2 px-2 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider w-1/3">
                Permission
              </th>
              {ROLES.map((role) => (
                <th
                  key={role}
                  className="text-center py-2 px-2 text-xs font-semibold uppercase tracking-wider"
                  style={{
                    color:
                      role === "admin"
                        ? "var(--accent)"
                        : role === "editor"
                        ? "#a78bfa"
                        : "var(--text-muted)",
                  }}
                >
                  {role}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {Object.entries(grouped).map(([category, perms]) => (
              <Fragment key={category}>
                <tr>
                  <td
                    colSpan={ROLES.length + 1}
                    className="pt-4 pb-1 px-2 text-[10px] font-bold text-[var(--text-faint)] uppercase tracking-widest"
                  >
                    {category}
                  </td>
                </tr>
                {perms.map((p) => (
                  <tr
                    key={p.code}
                    className="border-b border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors"
                  >
                    <td className="py-2 px-2">
                      <div className="text-[var(--text-secondary)]">{p.description}</div>
                      <div className="text-[10px] text-[var(--text-faint)] font-mono">{p.code}</div>
                    </td>
                    {ROLES.map((role) => {
                      const checked = role === "admin" || (matrix[role]?.has(p.code) ?? false);
                      const isAdmin = role === "admin";
                      return (
                        <td key={role} className="text-center py-2 px-2">
                          <input
                            type="checkbox"
                            checked={checked}
                            disabled={isAdmin}
                            onChange={() => toggle(role, p.code)}
                            className="w-4 h-4 rounded border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--accent)] focus:ring-[var(--accent)] focus:ring-offset-0 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                          />
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>

      <p className="mt-3 text-[11px] text-[var(--text-faint)]">
        Admin role always has all permissions and cannot be modified.
      </p>
    </Card>
  );
}
