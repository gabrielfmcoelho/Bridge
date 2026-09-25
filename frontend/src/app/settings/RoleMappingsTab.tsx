"use client";

import RowActions from "@/components/ui/RowActions";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { roleMappingsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import Card from "@/components/ui/Card";
import SectionCard from "@/components/ui/SectionCard";
import { tableClasses } from "@/components/ui/Table";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import { ICON_PATHS } from "@/lib/icon-paths";

const PROVIDERS = ["ldap", "keycloak", "gitlab"];
const ROLES = ["viewer", "editor", "admin"];

export default function RoleMappingsTab() {
  const confirm = useConfirm();
  const { t } = useLocale();
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
    <SectionCard as="h3" title={t("settings.roleMappings.title")} description={t("settings.roleMappings.intro")}>

      {/* Existing mappings */}
      {mappings.length > 0 ? (
        <div className="overflow-x-auto mb-4">
          <table className={tableClasses.compact.table}>
            <thead>
              <tr className={tableClasses.compact.headRow}>
                <th className={tableClasses.compact.th}>
                  {t("settings.roleMappings.provider")}
                </th>
                <th className={tableClasses.compact.th}>
                  {t("settings.roleMappings.externalGroupHeader")}
                </th>
                <th className={tableClasses.compact.th}>
                  {t("settings.roleMappings.localRole")}
                </th>
                <th className="w-10" />
              </tr>
            </thead>
            <tbody>
              {mappings.map((m) => (
                <tr key={m.id} className={tableClasses.compact.row}>
                  <td className={tableClasses.compact.td}>
                    <span className="inline-flex items-center gap-1.5 text-[var(--text-secondary)]">
                      <span className="w-2 h-2 rounded-full" style={{
                        backgroundColor: m.provider_name === "keycloak" ? "#22c55e" : m.provider_name === "gitlab" ? "#e24329" : "#3b82f6"
                      }} />
                      {m.provider_name}
                    </span>
                  </td>
                  <td className={`${tableClasses.compact.td} font-mono text-xs text-[var(--text-secondary)]`}>
                    {m.external_group}
                  </td>
                  <td className={tableClasses.compact.td}>
                    <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium border ${
                      m.local_role === "admin"
                        ? "bg-[var(--cyan)]/15 text-[var(--cyan)] border-[var(--cyan)]/25"
                        : m.local_role === "editor"
                        ? "bg-[var(--accent)]/15 text-[var(--accent)] border-[var(--accent)]/25"
                        : "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]"
                    }`}>
                      {m.local_role}
                    </span>
                  </td>
                  <td className={tableClasses.compact.td}>
                    <RowActions
                      name={m.external_group}
                      actions={[{ label: t("settings.roleMappings.deleteMapping"), icon: ICON_PATHS.trash, danger: true, onClick: async () => { if (await confirm({ title: t("confirm.deleteRoleMapping"), danger: true, confirmLabel: t("common.delete") })) deleteMutation.mutate(m.id); } }]}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="text-center py-6 text-sm text-[var(--text-muted)] mb-4">
          {t("settings.roleMappings.empty")}
        </div>
      )}

      {/* Add new mapping */}
      <div className="border-t border-[var(--border-subtle)] pt-4">
        <p className="text-xs font-semibold text-[var(--text-muted)] mb-3">
          {t("settings.roleMappings.addMapping")}
        </p>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3 items-end">
          <div>
            <NativeSelect label={t("settings.roleMappings.provider")}
              value={newMapping.provider_name}
              onChange={(e) => setNewMapping((p) => ({ ...p, provider_name: e.target.value }))}
            >
              {PROVIDERS.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </NativeSelect>
          </div>
          <Input
            label={t("settings.roleMappings.externalGroupLabel")}
            value={newMapping.external_group}
            onChange={(e) => setNewMapping((p) => ({ ...p, external_group: e.target.value }))}
            placeholder={t("settings.roleMappings.groupPlaceholder")}
          />
          <div>
            <NativeSelect label={t("settings.roleMappings.localRole")}
              value={newMapping.local_role}
              onChange={(e) => setNewMapping((p) => ({ ...p, local_role: e.target.value }))}
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>{r}</option>
              ))}
            </NativeSelect>
          </div>
          <Button
            onClick={handleCreate}
            loading={createMutation.isPending}
            disabled={!newMapping.external_group.trim()}
          >
            {t("common.add")}
          </Button>
        </div>
      </div>
    </SectionCard>
  );
}
