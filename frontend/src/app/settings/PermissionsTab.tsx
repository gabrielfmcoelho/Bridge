"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Fragment, useState, useEffect, useMemo } from "react";
import { permissionsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import SectionCard from "@/components/ui/SectionCard";
import { tableClasses } from "@/components/ui/Table";
import Button from "@/components/ui/Button";

const ROLES = ["viewer", "editor", "admin"] as const;

export default function PermissionsTab() {
  const { t } = useLocale();
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
    <SectionCard
      as="h3"
      title={t("settings.permissions.title")}
      description={t("settings.permissions.adminNote")}
      controls={dirty ? (
        <Button size="sm" onClick={() => saveMutation.mutate()} loading={saveMutation.isPending}>
          {t("settings.permissions.saveChanges")}
        </Button>
      ) : undefined}
    >

      <div className="overflow-x-auto">
        <table className={tableClasses.compact.table}>
          <thead>
            <tr className={tableClasses.compact.headRow}>
              <th className={`${tableClasses.compact.th} w-1/3`}>
                {t("settings.permissions.permission")}
              </th>
              {ROLES.map((role) => (
                <th
                  key={role}
                  className="text-center py-2 px-2 text-xs font-semibold"
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
                    className="pt-4 pb-1 px-2 text-2xs font-bold text-[var(--text-faint)]"
                  >
                    {category}
                  </td>
                </tr>
                {perms.map((p) => (
                  <tr
                    key={p.code}
                    className="border-b border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors"
                  >
                    <td className={tableClasses.compact.td}>
                      <div className="text-[var(--text-secondary)]">{p.description}</div>
                      <div className="text-2xs text-[var(--text-faint)] font-mono">{p.code}</div>
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
    </SectionCard>
  );
}
