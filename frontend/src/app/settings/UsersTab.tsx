"use client";

import RowActions from "@/components/ui/RowActions";
import { ROLE_COLORS } from "@/lib/constants";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { usersAPI, entidadesAPI } from "@/lib/api";
import { indentedLabel, withDepth } from "@/lib/entidades";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { tableClasses } from "@/components/ui/Table";
import Card from "@/components/ui/Card";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import Avatar from "@/components/ui/Avatar";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import { ICON_PATHS } from "@/lib/icon-paths";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormError from "@/components/ui/FormError";
import CheckboxList from "@/components/ui/CheckboxList";

const roleColors = ROLE_COLORS;

const providerLabels: Record<string, string> = {
  local: "Local",
  ldap: "LDAP",
  gitlab: "GitLab",
  keycloak: "SSO",
};

function UserActions({ u, onEdit, onDelete }: { u: import("@/lib/types").User; onEdit: () => void; onDelete: () => void }) {
  const { t } = useLocale();
  return (
    <RowActions
      name={u.display_name || u.username}
      actions={[
        { label: t("common.edit"), icon: ICON_PATHS.pencil, onClick: onEdit },
        { label: t("common.delete"), icon: ICON_PATHS.trash, onClick: onDelete, danger: true },
      ]}
    />
  );
}

export default function UsersTab() {
  const { t, formatDate } = useLocale();
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const [viewMode, setViewMode] = useState<"cards" | "table">("table");
  const [showForm, setShowForm] = useState(false);
  const [editUser, setEditUser] = useState<import("@/lib/types").User | null>(null);
  const emptyNew = { username: "", password: "", display_name: "", role: "viewer", entidade_ids: [] as number[], primary_entidade_id: null as number | null };
  const [newUser, setNewUser] = useState(emptyNew);
  const [editForm, setEditForm] = useState({ display_name: "", role: "", password: "", entidade_ids: [] as number[], primary_entidade_id: null as number | null });
  const [error, setError] = useState("");

  const { data: users = [] } = useQuery({
    queryKey: ["users"],
    queryFn: usersAPI.list,
  });
  const { data: entidades = [] } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list });
  const entidadeNodes = withDepth(entidades);

  const createMutation = useMutation({
    mutationFn: () => usersAPI.create(newUser),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      setShowForm(false);
      setNewUser(emptyNew);
      setError("");
    },
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editUser) return Promise.reject();
      const data: Parameters<typeof usersAPI.update>[1] = {};
      if (editForm.display_name !== editUser.display_name) data.display_name = editForm.display_name;
      if (editForm.role !== editUser.role) data.role = editForm.role as typeof editUser.role;
      if (editForm.password) data.password = editForm.password;
      const currentIDs = (editUser.entidades ?? []).map((e) => e.id);
      const currentPrimary = editUser.entidades?.find((e) => e.is_primary)?.id ?? null;
      if (
        editForm.entidade_ids.length !== currentIDs.length ||
        editForm.entidade_ids.some((id) => !currentIDs.includes(id)) ||
        editForm.primary_entidade_id !== currentPrimary
      ) {
        data.entidade_ids = editForm.entidade_ids;
        data.primary_entidade_id = editForm.primary_entidade_id;
      }
      return usersAPI.update(editUser.id, data);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      setEditUser(null);
      setError("");
    },
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => usersAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  const openEdit = (u: import("@/lib/types").User) => {
    setEditUser(u);
    setEditForm({
      display_name: u.display_name,
      role: u.role,
      password: "",
      entidade_ids: (u.entidades ?? []).map((e) => e.id),
      primary_entidade_id: u.entidades?.find((e) => e.is_primary)?.id ?? null,
    });
    setError("");
  };

  const confirmDelete = async (u: import("@/lib/types").User) => {
    if (await confirm({ title: t("confirm.deleteUser", { name: u.username }), message: t("confirm.cannotUndo"), danger: true, confirmLabel: t("common.delete") })) deleteMutation.mutate(u.id);
  };

  const roleAccentColor: Record<string, string> = {
    admin: "accent",
    editor: "accent",
    viewer: "muted",
  };

  return (
    <div className="space-y-4">
      <SectionCard
        variant="plain"
        title={t("settings.users")}
        count={users.length}
        controls={
          <>
            <div className="hidden sm:flex">
              <ViewToggle
                value={viewMode}
                onChange={(v) => setViewMode(v as "cards" | "table")}
                options={[
                  { key: "cards", label: "Card view", icon: VIEW_ICONS.cards },
                  { key: "table", label: "Table view", icon: VIEW_ICONS.table },
                ]}
              />
            </div>
            <Button size="sm" onClick={() => setShowForm(true)}>+ {t("settings.addUser")}</Button>
          </>
        }
      >

      {/* Table view */}
      {viewMode === "table" ? (
        <div className="border border-[var(--border-default)] rounded-[var(--radius-lg)] overflow-x-auto animate-fade-in">
          <table className="w-full text-sm min-w-[600px]">
            <thead>
              <tr className={tableClasses.headRow}>
                <th className={tableClasses.th}>{t("auth.username")}</th>
                <th className={tableClasses.th}>{t("settings.role")}</th>
                <th className={tableClasses.th}>{t("entidades.title")}</th>
                <th className={tableClasses.th}>{t("settings.authProvider")}</th>
                <th className={tableClasses.th}>{t("settings.createdAt")}</th>
                <th className={`${tableClasses.th} text-right w-20`} />
              </tr>
            </thead>
            <tbody>
              {users.map((u, i) => (
                <tr
                  key={u.id}
                  className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
                >
                  <td className={tableClasses.td}>
                    <div className="flex items-center gap-2.5">
                      <Avatar name={u.display_name || u.username} size="sm" />
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-[var(--text-primary)] truncate">{u.display_name || u.username}</p>
                        <p className="text-xs text-[var(--text-faint)]">@{u.username}</p>
                      </div>
                    </div>
                  </td>
                  <td className={tableClasses.td}>
                    <span className={`text-2xs px-1.5 py-0.5 rounded-full border font-medium ${roleColors[u.role] || roleColors.viewer}`}>
                      {u.role}
                    </span>
                  </td>
                  <td className={`${tableClasses.td} text-xs`}>
                    <div className="flex flex-wrap gap-1">
                      {(u.entidades ?? []).map((e) => (
                        <span key={e.id} className={`px-1.5 py-0.5 rounded border text-2xs ${e.is_primary ? "border-[var(--accent)]/40 text-[var(--accent)] bg-[var(--accent-muted)]" : "border-[var(--border-subtle)] text-[var(--text-muted)]"}`} title={e.is_primary ? t("entidades.primary") : undefined}>
                          {e.name}
                        </span>
                      ))}
                      {(u.entidades ?? []).length === 0 && <span className="text-[var(--text-faint)]">-</span>}
                    </div>
                  </td>
                  <td className={`${tableClasses.td} text-[var(--text-secondary)] text-xs`}>
                    {providerLabels[u.auth_provider] || u.auth_provider || "Local"}
                  </td>
                  <td className={`${tableClasses.td} text-[var(--text-muted)] text-xs`}>
                    {u.created_at ? formatDate(u.created_at) : "-"}
                  </td>
                  <td className={`${tableClasses.td} text-right`}>
                    <UserActions u={u} onEdit={() => openEdit(u)} onDelete={() => confirmDelete(u)} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        /* Card view */
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {users.map((u, i) => (
            <Card
              key={u.id}
              accent={roleAccentColor[u.role] || roleAccentColor.viewer}
              className="stagger-in" style={{ "--i": i } as React.CSSProperties}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex items-center gap-2.5 min-w-0">
                  <Avatar name={u.display_name || u.username} size="md" />
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-[var(--text-primary)] truncate">{u.display_name || u.username}</p>
                    <p className="text-xs text-[var(--text-faint)]">@{u.username}</p>
                  </div>
                </div>
                <UserActions u={u} onEdit={() => openEdit(u)} onDelete={() => confirmDelete(u)} />
              </div>

              <div className="grid grid-cols-2 gap-x-4 gap-y-2 mt-3 pt-3 border-t border-[var(--border-subtle)]">
                <div>
                  <p className="text-2xs text-[var(--text-faint)]">{t("settings.role")}</p>
                  <span className={`inline-block mt-0.5 text-2xs px-1.5 py-0.5 rounded-full border font-medium ${roleColors[u.role] || roleColors.viewer}`}>
                    {u.role}
                  </span>
                </div>
                <div>
                  <p className="text-2xs text-[var(--text-faint)]">{t("settings.authProvider")}</p>
                  <p className="text-xs text-[var(--text-secondary)] mt-0.5">{providerLabels[u.auth_provider] || u.auth_provider || "Local"}</p>
                </div>
                {u.email && (
                  <div className="col-span-2">
                    <p className="text-2xs text-[var(--text-faint)]">{t("common.email")}</p>
                    <p className="text-xs text-[var(--text-secondary)] mt-0.5 truncate">{u.email}</p>
                  </div>
                )}
              </div>

              <div className="flex items-center justify-between mt-3 pt-2.5 border-t border-[var(--border-subtle)]">
                <span className="text-2xs text-[var(--text-faint)]">
                  {u.created_at ? formatDate(u.created_at) : ""}
                </span>
              </div>
            </Card>
          ))}
        </div>
      )}
      </SectionCard>

      {/* Create user modal */}
      <ResponsiveModal open={showForm} onClose={() => setShowForm(false)} title={t("settings.addUser")}>
        <form
          onSubmit={(e) => { e.preventDefault(); createMutation.mutate(); }}
          className="space-y-4"
        >
          <FormError message={error} />
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Input label={t("auth.displayName")} value={newUser.display_name} onChange={(e) => setNewUser({ ...newUser, display_name: e.target.value })} />
            <Input label={t("auth.username")} value={newUser.username} onChange={(e) => setNewUser({ ...newUser, username: e.target.value })} required />
            <Input label={t("auth.password")} type="password" value={newUser.password} onChange={(e) => setNewUser({ ...newUser, password: e.target.value })} required />
            <Select
              label={t("settings.role")}
              value={newUser.role}
              onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}
              options={[
                { value: "viewer", label: "Viewer" },
                { value: "editor", label: "Editor" },
                { value: "admin", label: "Admin" },
              ]}
            />
          </div>
          <UserEntidadeFields
            nodes={entidadeNodes}
            ids={newUser.entidade_ids}
            primary={newUser.primary_entidade_id}
            onChange={(ids, primary) => setNewUser({ ...newUser, entidade_ids: ids, primary_entidade_id: primary })}
          />
          <div className="flex justify-end">
            <Button type="submit" loading={createMutation.isPending}>{t("common.create")}</Button>
          </div>
        </form>
      </ResponsiveModal>

      {/* Edit user modal */}
      <ResponsiveModal open={!!editUser} onClose={() => setEditUser(null)} title={t("settings.editUser")}>
        {editUser && (
          <form
            onSubmit={(e) => { e.preventDefault(); updateMutation.mutate(); }}
            className="space-y-4"
          >
            <FormError message={error} />
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Input label={t("auth.displayName")} value={editForm.display_name} onChange={(e) => setEditForm({ ...editForm, display_name: e.target.value })} />
              <Select
                label={t("settings.role")}
                value={editForm.role}
                onChange={(e) => setEditForm({ ...editForm, role: e.target.value })}
                options={[
                  { value: "viewer", label: "Viewer" },
                  { value: "editor", label: "Editor" },
                  { value: "admin", label: "Admin" },
                ]}
              />
              <Input label={t("settings.userPasswordOptional")} type="password" value={editForm.password} onChange={(e) => setEditForm({ ...editForm, password: e.target.value })} placeholder={t("settings.userPasswordKeepCurrent")} />
            </div>
            <UserEntidadeFields
              nodes={entidadeNodes}
              ids={editForm.entidade_ids}
              primary={editForm.primary_entidade_id}
              onChange={(ids, primary) => setEditForm({ ...editForm, entidade_ids: ids, primary_entidade_id: primary })}
            />
            <div className="flex justify-end">
              <Button type="submit" loading={updateMutation.isPending}>{t("common.save")}</Button>
            </div>
          </form>
        )}
      </ResponsiveModal>
    </div>
  );
}

/** Membership picker for user forms: N entidades + which one is primary. */
function UserEntidadeFields({
  nodes,
  ids,
  primary,
  onChange,
}: {
  nodes: ReturnType<typeof withDepth>;
  ids: number[];
  primary: number | null;
  onChange: (ids: number[], primary: number | null) => void;
}) {
  const { t } = useLocale();
  const chosen = nodes.filter((n) => ids.includes(n.id));
  return (
    <div className="space-y-3">
      <CheckboxList
        label={t("entidades.title")}
        items={nodes.map((n) => ({ id: n.id, name: indentedLabel(n) }))}
        selected={ids}
        onChange={(next) => onChange(next, primary != null && next.includes(primary) ? primary : next[0] ?? null)}
      />
      {chosen.length > 1 && (
        <Select
          label={t("entidades.primary")}
          value={primary != null ? String(primary) : ""}
          onChange={(e) => onChange(ids, e.target.value ? Number(e.target.value) : null)}
          options={chosen.map((n) => ({ value: String(n.id), label: n.name }))}
        />
      )}
    </div>
  );
}
