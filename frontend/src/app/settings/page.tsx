"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useRef } from "react";
import { useRouter } from "next/navigation";
import { enumsAPI, usersAPI, appearanceAPI, importAPI, backupAPI, entidadesAPI } from "@/lib/api";
import { indentedLabel, withDepth } from "@/lib/entidades";
import type { ImportResult } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import { tableClasses } from "@/components/ui/Table";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Avatar from "@/components/ui/Avatar";
import TabBar from "@/components/ui/TabBar";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import IconButton from "@/components/ui/IconButton";
import PillButton from "@/components/ui/PillButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Badge from "@/components/ui/Badge";
import Drawer from "@/components/ui/Drawer";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormError from "@/components/ui/FormError";
import CheckboxList from "@/components/ui/CheckboxList";
import IntegrationsTab from "./IntegrationsTab";
import PermissionsTab from "./PermissionsTab";
import RoleMappingsTab from "./RoleMappingsTab";
import EntidadesTab from "./EntidadesTab";
import OfferingsTab from "./OfferingsTab";

type Tab = "enums" | "users" | "entidades" | "offerings" | "appearance" | "import" | "backup" | "integrations" | "permissions" | "role-mappings";

const roleColors: Record<string, string> = {
  admin: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]",
  editor: "bg-[var(--purple)]/10 text-[var(--purple)]/70 border-[var(--purple)]/15",
  viewer: "bg-[var(--bg-overlay)] text-[var(--text-faint)] border-[var(--border-subtle)]",
};

const providerLabels: Record<string, string> = {
  local: "Local",
  ldap: "LDAP",
  gitlab: "GitLab",
  keycloak: "SSO",
};

export default function SettingsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  // ponytail: role gate until Phase B ships the catalog.manage permission;
  // then this becomes user?.permissions?.includes("catalog.manage").
  const canManageCatalog = isAdmin || user?.role === "editor";
  const [activeTab, setActiveTab] = useState<Tab>("enums");
  const [showTabDrawer, setShowTabDrawer] = useState(false);

  const tabs: { key: Tab; label: string }[] = [
    { key: "enums", label: t("settings.enums") },
    ...(isAdmin ? [{ key: "users" as Tab, label: t("settings.users") }] : []),
    ...(isAdmin ? [{ key: "entidades" as Tab, label: t("entidades.title") }] : []),
    ...(canManageCatalog ? [{ key: "offerings" as Tab, label: t("settings.offerings.title") }] : []),
    ...(isAdmin ? [{ key: "appearance" as Tab, label: t("settings.appearance") }] : []),
    ...(isAdmin ? [{ key: "import" as Tab, label: t("settings.import") }] : []),
    ...(isAdmin ? [{ key: "backup" as Tab, label: t("settings.backup") }] : []),
    ...(isAdmin ? [{ key: "integrations" as Tab, label: t("settings.integrationsTabLabel") }] : []),
    ...(isAdmin ? [{ key: "permissions" as Tab, label: t("settings.permissions.tabLabel") }] : []),
    ...(isAdmin ? [{ key: "role-mappings" as Tab, label: t("settings.roleMappings.tabLabel") }] : []),
  ];

  const activeLabel = tabs.find((t) => t.key === activeTab)?.label ?? "";

  return (
    <PageShell>
      <PageHeader
        title={t("settings.title")}
        actions={
          <Button variant="secondary" size="sm" className="md:hidden" onClick={() => setShowTabDrawer(true)}>
            {activeLabel}
            <Icon path={ICON_PATHS.chevronUp} className="w-4 h-4 rotate-180 text-[var(--text-muted)]" />
          </Button>
        }
      />

      {/* Desktop: scrollable tab bar */}
      <div className="hidden md:block mb-6">
        <TabBar tabs={tabs} activeTab={activeTab} onChange={(k) => setActiveTab(k as Tab)} />
      </div>

      {/* Mobile: tab drawer */}
      <Drawer open={showTabDrawer} onClose={() => setShowTabDrawer(false)} title={t("settings.title")}>
        <div className="p-2 space-y-1">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => { setActiveTab(tab.key); setShowTabDrawer(false); }}
              className={`w-full text-left px-4 py-3 text-sm font-medium rounded-[var(--radius-md)] transition ${
                activeTab === tab.key
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </Drawer>

      <div className="animate-fade-in">
        {activeTab === "enums" && <EnumSection />}
        {activeTab === "users" && isAdmin && <UsersSection />}
        {activeTab === "entidades" && isAdmin && <EntidadesTab />}
        {activeTab === "offerings" && canManageCatalog && <OfferingsTab />}
        {activeTab === "appearance" && isAdmin && <AppearanceSection />}
        {activeTab === "import" && isAdmin && <ImportSection />}
        {activeTab === "backup" && isAdmin && <BackupSection />}
        {activeTab === "integrations" && isAdmin && <IntegrationsTab />}
        {activeTab === "permissions" && isAdmin && <PermissionsTab />}
        {activeTab === "role-mappings" && isAdmin && <RoleMappingsTab />}
      </div>
    </PageShell>
  );
}

function EnumSection() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [newValue, setNewValue] = useState<Record<string, string>>({});
  const [newColor, setNewColor] = useState<Record<string, string>>({});
  const [editing, setEditing] = useState<{ category: string; value: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const [editColor, setEditColor] = useState("");
  const [editError, setEditError] = useState("");
  const [newCategory, setNewCategory] = useState("");
  const [newCategoryValue, setNewCategoryValue] = useState("");
  const isAdmin = user?.role === "admin";

  const { data: allEnums = {} } = useQuery({
    queryKey: ["enums"],
    queryFn: enumsAPI.listAll,
  });

  const addMutation = useMutation({
    mutationFn: ({ category, value, color }: { category: string; value: string; color?: string }) =>
      enumsAPI.create(category, value, color),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["enums"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ category, value }: { category: string; value: string }) =>
      enumsAPI.delete(category, value),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["enums"] }),
  });

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editing) throw new Error(t("settings.enumsNoItemSelected"));
      return enumsAPI.update(editing.category, editing.value, editValue, editColor);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["enums"] });
      setEditing(null);
      setEditError("");
    },
    onError: (err) => setEditError(err instanceof Error ? err.message : t("common.requestFailed")),
  });

  const openEdit = (category: string, value: string) => {
    setEditing({ category, value });
    setEditValue(value);
    const option = (allEnums[category] || []).find((opt) => opt.value === value);
    setEditColor(option?.color || "");
    setEditError("");
  };

  const isSituacaoCategory = (category: string) => category === "situacao";

  // System categories that must always appear even if empty
  const REQUIRED_CATEGORIES = [
    "hospedagem", "situacao", "tipo_maquina", "orchestrator_type",
    "entidade_responsavel", "issue_status", "issue_priority",
  ];
  const mergedEnums = { ...allEnums };
  for (const cat of REQUIRED_CATEGORIES) {
    if (!mergedEnums[cat]) mergedEnums[cat] = [];
  }

  return (
    <div className="space-y-4">
      {Object.entries(mergedEnums).map(([category, options], i) => (
        <Card key={category} hover={false} className="stagger-in" style={{ "--i": i } as React.CSSProperties}>
          <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
            {category}
          </h3>
          <div className="flex flex-wrap gap-1.5 mb-3">
            {options.map((opt) => (
              <span key={opt.value} className="group inline-flex items-center gap-1">
                <button type="button" onClick={() => isAdmin && openEdit(category, opt.value)} className={isAdmin ? "cursor-pointer" : ""}>
                  <Badge>
                    {isSituacaoCategory(category) && opt.color && (
                      <span className="w-2 h-2 rounded-full border border-white/20" style={{ backgroundColor: opt.color }} />
                    )}
                    {opt.value}
                  </Badge>
                </button>
                {isAdmin && (
                  <button
                    onClick={() => deleteMutation.mutate({ category, value: opt.value })}
                    className="opacity-0 group-hover:opacity-100 text-[var(--text-faint)] hover:text-[var(--danger)] text-xs transition-opacity"
                  >
                    &times;
                  </button>
                )}
              </span>
            ))}
            {options.length === 0 && (
              <span className="text-xs text-[var(--text-faint)]">{t("settings.enumsNoValues")}</span>
            )}
          </div>
          {isAdmin && (
            <form className="flex gap-2" onSubmit={(e) => {
              e.preventDefault();
              if (newValue[category]) {
                addMutation.mutate({
                  category,
                  value: newValue[category],
                  color: isSituacaoCategory(category) ? (newColor[category] || "") : "",
                });
                setNewValue((v) => ({ ...v, [category]: "" }));
                setNewColor((v) => ({ ...v, [category]: "" }));
              }
            }}>
              <Input
                value={newValue[category] || ""}
                onChange={(e) => setNewValue((v) => ({ ...v, [category]: e.target.value }))}
                placeholder={t("settings.enumsNewValuePlaceholder", { category })}
                className="max-w-xs"
              />
              <Button size="sm" type="submit">
                {t("common.add")}
              </Button>
              {isSituacaoCategory(category) && (
                <div className="flex items-end gap-2">
                  <Input
                    type="color"
                    value={newColor[category] || "#06b6d4"}
                    onChange={(e) => setNewColor((v) => ({ ...v, [category]: e.target.value }))}
                    className="w-12 h-9 p-1"
                    title={t("settings.enumsStatusColorTitle")}
                  />
                </div>
              )}
            </form>
          )}
        </Card>
      ))}

      {/* Add new category */}
      {isAdmin && (
        <Card hover={false}>
          <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
            {t("settings.enumsNewCategory")}
          </h3>
          <form className="flex gap-2 flex-wrap" onSubmit={(e) => {
            e.preventDefault();
            if (newCategory && newCategoryValue) {
              addMutation.mutate({ category: newCategory, value: newCategoryValue, color: newCategory === "situacao" ? "#06b6d4" : "" });
              setNewCategory("");
              setNewCategoryValue("");
            }
          }}>
            <Input
              value={newCategory}
              onChange={(e) => setNewCategory(e.target.value)}
              placeholder={t("settings.enumsCategoryNamePlaceholder")}
              className="max-w-[180px]"
            />
            <Input
              value={newCategoryValue}
              onChange={(e) => setNewCategoryValue(e.target.value)}
              placeholder={t("settings.enumsFirstValuePlaceholder")}
              className="max-w-[180px]"
            />
            <Button size="sm" type="submit">
              {t("common.create")}
            </Button>
          </form>
        </Card>
      )}

      {/* Edit enum value modal */}
      <ResponsiveModal open={editing !== null} onClose={() => setEditing(null)} title={`${t("common.edit")} — ${editing?.category}`}>
        {editing && (
          <form onSubmit={(e) => { e.preventDefault(); updateMutation.mutate(); }} className="space-y-4">
            {editError && (
              <div className="bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)] text-sm rounded-[var(--radius-md)] p-3 animate-slide-down">{editError}</div>
            )}
            <Input
              label={t("settings.enumsValueLabel")}
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              required
            />
            {editing.category === "situacao" && (
              <Input
                label={t("settings.enumsColorLabel")}
                type="color"
                value={editColor || "#06b6d4"}
                onChange={(e) => setEditColor(e.target.value)}
                className="w-16 h-10 p-1"
              />
            )}
            <div className="flex justify-end gap-2">
              <Button type="button" variant="secondary" onClick={() => setEditing(null)}>
                {t("common.cancel")}
              </Button>
              <Button type="submit" loading={updateMutation.isPending}>
                {t("common.save")}
              </Button>
            </div>
          </form>
        )}
      </ResponsiveModal>
    </div>
  );
}

function UserActions({ u, onEdit, onDelete }: { u: import("@/lib/types").User; onEdit: () => void; onDelete: () => void }) {
  const { t } = useLocale();
  return (
    <div className="flex items-center gap-1">
      <IconButton onClick={onEdit} title={t("common.edit")}>
        <Icon path={ICON_PATHS.pencil} className="w-3.5 h-3.5" />
      </IconButton>
      <IconButton variant="danger" onClick={onDelete} title={t("common.delete")}>
        <Icon path={ICON_PATHS.trash} className="w-3.5 h-3.5" />
      </IconButton>
    </div>
  );
}

function UsersSection() {
  const { t, formatDate } = useLocale();
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

  const confirmDelete = (u: import("@/lib/types").User) => {
    if (confirm(`Delete user ${u.username}?`)) deleteMutation.mutate(u.id);
  };

  const roleAccentColor: Record<string, string> = {
    admin: "accent",
    editor: "purple",
    viewer: "muted",
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold font-display" style={{ color: "var(--text-secondary)" }}>
          {t("settings.users")}
        </h2>
        <div className="flex items-center gap-1.5">
          {/* View toggle */}
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
        </div>
      </div>

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

const PRESET_COLORS = [
  { label: "Cyan", value: "#06b6d4" },
  { label: "Blue", value: "#3b82f6" },
  { label: "Violet", value: "#8b5cf6" },
  { label: "Purple", value: "#a855f7" },
  { label: "Pink", value: "#ec4899" },
  { label: "Rose", value: "#f43f5e" },
  { label: "Orange", value: "#f97316" },
  { label: "Amber", value: "#f59e0b" },
  { label: "Emerald", value: "#10b981" },
  { label: "Teal", value: "#14b8a6" },
];

function AppearanceSection() {
  const { t } = useLocale();
  const appearance = useAppearance();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [appName, setAppName] = useState(appearance.appName);
  const [appColor, setAppColor] = useState(appearance.appColor);
  const [appLogo, setAppLogo] = useState(appearance.appLogo);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [success, setSuccess] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    setSuccess(false);
    try {
      await appearanceAPI.update({ app_name: appName, app_color: appColor, app_logo: appLogo });
      await appearance.refresh();
      setSuccess(true);
      setTimeout(() => setSuccess(false), 2000);
    } catch {
      // error handled silently
    } finally {
      setSaving(false);
    }
  };

  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const result = await appearanceAPI.uploadLogo(file);
      setAppLogo(result.logo);
      await appearance.refresh();
    } catch {
      // error handled silently
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleRemoveLogo = async () => {
    try {
      await appearanceAPI.deleteLogo();
      setAppLogo("");
      await appearance.refresh();
    } catch {
      // error handled silently
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      {/* App Name */}
      <Card hover={false} className="stagger-in" style={{ "--i": 0 } as React.CSSProperties}>
        <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
          {t("settings.appName")}
        </h3>
        <p className="text-xs mb-3" style={{ color: "var(--text-faint)" }}>
          {t("settings.appNameDescription")}
        </p>
        <Input
          value={appName}
          onChange={(e) => setAppName(e.target.value)}
          placeholder="Bridge"
          className="max-w-xs"
        />
      </Card>

      {/* Main Color */}
      <Card hover={false} className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
        <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
          {t("settings.mainColor")}
        </h3>
        <p className="text-xs mb-4" style={{ color: "var(--text-faint)" }}>
          {t("settings.mainColorDescription")}
        </p>

        {/* Preset swatches */}
        <div className="flex flex-wrap gap-2 mb-4">
          {PRESET_COLORS.map((c) => (
            <button
              key={c.value}
              onClick={() => setAppColor(c.value)}
              className="group relative w-8 h-8 rounded-[var(--radius-sm)] border-2 transition duration-150 hover:scale-110"
              style={{
                backgroundColor: c.value,
                borderColor: appColor === c.value ? "var(--text-primary)" : "transparent",
                boxShadow: appColor === c.value ? `0 0 12px ${c.value}40` : "none",
              }}
              title={c.label}
            >
              {appColor === c.value && (
                <Icon path={ICON_PATHS.check} className="w-4 h-4 text-white absolute inset-0 m-auto drop-shadow" strokeWidth={3} />
              )}
            </button>
          ))}
        </div>

        {/* Custom color picker */}
        <div className="flex items-center gap-3">
          <div className="relative">
            <input
              type="color"
              value={appColor}
              onChange={(e) => setAppColor(e.target.value)}
              className="w-10 h-10 rounded-[var(--radius-sm)] border border-[var(--border-default)] cursor-pointer bg-transparent"
            />
          </div>
          <Input
            value={appColor}
            onChange={(e) => setAppColor(e.target.value)}
            placeholder="#06b6d4"
            className="max-w-[140px] font-mono"
          />
          <div className="h-8 flex-1 rounded-[var(--radius-sm)]" style={{ background: `linear-gradient(135deg, ${appColor}, ${appColor}80)` }} />
        </div>
      </Card>

      {/* Logo */}
      <Card hover={false} className="stagger-in" style={{ "--i": 2 } as React.CSSProperties}>
        <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
          {t("settings.logo")}
        </h3>
        <p className="text-xs mb-4" style={{ color: "var(--text-faint)" }}>
          {t("settings.logoDescription")}
        </p>

        <div className="flex items-center gap-4">
          {/* Preview */}
          <div
            className="w-16 h-16 rounded-[var(--radius-lg)] border border-[var(--border-default)] flex items-center justify-center overflow-hidden"
            style={{ backgroundColor: "var(--bg-elevated)" }}
          >
            {appLogo ? (
              <img src={appLogo} alt={t("settings.logoAlt")} className="w-full h-full object-contain p-1" />
            ) : (
              <Icon path={ICON_PATHS.prompt} className="w-7 h-7" strokeWidth={1.5} style={{ color: "var(--text-faint)" }} />
            )}
          </div>

          <div className="flex flex-col gap-2">
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              onChange={handleLogoUpload}
              className="hidden"
            />
            <Button
              size="sm"
              variant="secondary"
              loading={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              {t("settings.uploadLogo")}
            </Button>
            {appLogo && (
              <button
                onClick={handleRemoveLogo}
                className="text-xs hover:text-[var(--danger)] transition-colors text-left"
                style={{ color: "var(--text-faint)" }}
              >
                {t("settings.removeLogo")}
              </button>
            )}
          </div>
        </div>
      </Card>

      {/* Save button */}
      <div className="flex items-center gap-3 stagger-in" style={{ "--i": 3 } as React.CSSProperties}>
        <Button onClick={handleSave} loading={saving}>
          {t("common.save")}
        </Button>
        {success && (
          <span className="text-xs animate-fade-in" style={{ color: "var(--success)" }}>
            {t("settings.saved")}
          </span>
        )}
      </div>
    </div>
  );
}

function ImportSection() {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [importType, setImportType] = useState<"hosts" | "dns">("hosts");
  const [fileData, setFileData] = useState<Record<string, unknown>[] | null>(null);
  const [fileName, setFileName] = useState("");
  const [parseError, setParseError] = useState("");
  const [result, setResult] = useState<ImportResult | null>(null);
  const [importing, setImporting] = useState(false);

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setResult(null);
    setParseError("");
    setFileName(file.name);

    const reader = new FileReader();
    reader.onload = (ev) => {
      try {
        const raw = JSON.parse(ev.target?.result as string);
        const arr = Array.isArray(raw) ? raw : raw.data ? raw.data : null;
        if (!arr || !Array.isArray(arr) || arr.length === 0) {
          setParseError("JSON must be an array of objects, or an object with a \"data\" array field");
          setFileData(null);
          return;
        }
        // Strip internal metadata fields
        const cleaned = arr.map((item: Record<string, unknown>) => {
          const copy = { ...item };
          for (const key of Object.keys(copy)) {
            if (key.startsWith("_")) delete copy[key];
          }
          return copy;
        });
        setFileData(cleaned);
      } catch {
        setParseError("Invalid JSON file");
        setFileData(null);
      }
    };
    reader.readAsText(file);
  };

  const handleImport = async () => {
    if (!fileData) return;
    setImporting(true);
    setResult(null);
    try {
      const res = importType === "hosts"
        ? await importAPI.hosts(fileData)
        : await importAPI.dns(fileData);
      setResult(res);
      queryClient.invalidateQueries({ queryKey: [importType === "hosts" ? "hosts" : "dns"] });
    } catch (err) {
      setParseError(err instanceof Error ? err.message : "Import failed");
    } finally {
      setImporting(false);
    }
  };

  const reset = () => {
    setFileData(null);
    setFileName("");
    setParseError("");
    setResult(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  // Detect first field to auto-suggest type
  const detectedType = fileData && fileData.length > 0
    ? ("domain" in fileData[0] ? "dns" : "hosts")
    : null;

  return (
    <div className="space-y-6 max-w-2xl">
      {/* Import type selector */}
      <Card hover={false} className="stagger-in" style={{ "--i": 0 } as React.CSSProperties}>
        <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
          {t("settings.importType")}
        </h3>
        <div className="flex gap-2">
          <PillButton size="lg" active={importType === "hosts"} onClick={() => setImportType("hosts")}>
            <Icon path={ICON_PATHS.serverStack} />
            {t("nav.hosts")}
          </PillButton>
          <PillButton size="lg" active={importType === "dns"} onClick={() => setImportType("dns")}>
            <Icon path={ICON_PATHS.globeMeridian} />
            {t("nav.dns")}
          </PillButton>
        </div>
      </Card>

      {/* File upload */}
      <Card hover={false} className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
        <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
          {t("settings.importer.jsonFile")}
        </h3>
        <p className="text-xs mb-3" style={{ color: "var(--text-faint)" }}>
          {importType === "hosts"
            ? t("settings.importer.hostsHint")
            : t("settings.importer.dnsHint")
          }
        </p>

        {/* JSON example */}
        <details className="mb-4 group">
          <summary className="text-xs text-[var(--accent)] cursor-pointer hover:underline font-medium">
            {t("settings.importer.showExample")}
          </summary>
          <pre className="mt-2 p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] overflow-x-auto font-mono">
{importType === "hosts" ? `[
  {
    "nickname": "My Server",
    "oficial_slug": "MY-SERVER",
    "hostname": "10.0.1.10",
    "hospedagem": "ETIPI",
    "user": "admin",
    "password": "secret123",
    "has_key": false,
    "situacao": "active",
    "setor_responsavel": "SEAD/NTGD",
    "responsavel_interno": "John Doe",
    "description": "Production server",
    "tags": ["prod", "web"]
  }
]` : `[
  {
    "domain": "app.example.gov.br",
    "has_https": true,
    "situacao": "active",
    "responsavel": "John Doe",
    "observacoes": "Main application",
    "tags": ["prod"],
    "host_ids": [1, 2]
  }
]`}
          </pre>
        </details>

        <input
          ref={fileInputRef}
          type="file"
          accept=".json,application/json"
          onChange={handleFileSelect}
          className="hidden"
        />

        {!fileData ? (
          <button
            onClick={() => fileInputRef.current?.click()}
            className="w-full py-8 border-2 border-dashed border-[var(--border-default)] rounded-[var(--radius-lg)] hover:border-[var(--accent)] hover:bg-[var(--accent-muted)]/5 transition group"
          >
            <div className="flex flex-col items-center gap-2">
              <Icon path={ICON_PATHS.cloudUpload} className="w-8 h-8 text-[var(--text-faint)] group-hover:text-[var(--accent)] transition-colors" strokeWidth={1.5} />
              <span className="text-sm text-[var(--text-muted)] group-hover:text-[var(--text-secondary)]">
                {t("settings.importer.selectFile")}
              </span>
            </div>
          </button>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-default)]">
              <div className="flex items-center gap-2 min-w-0">
                <Icon path={ICON_PATHS.document} className="w-4 h-4 shrink-0 text-[var(--accent)]" />
                <span className="text-sm text-[var(--text-primary)] truncate font-mono">{fileName}</span>
              </div>
              <button onClick={reset} className="text-xs text-[var(--text-faint)] hover:text-[var(--danger)] transition-colors shrink-0 ml-2">
                {t("common.remove")}
              </button>
            </div>

            {/* Preview stats */}
            <div className="grid grid-cols-3 gap-3">
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--text-primary)] font-mono">{fileData.length}</div>
                <div className="text-2xs text-[var(--text-faint)]">{t("settings.importer.records")}</div>
              </div>
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--accent)] font-mono">
                  {importType === "hosts"
                    ? fileData.filter(d => d.user || d.password).length
                    : fileData.filter(d => d.responsavel).length
                  }
                </div>
                <div className="text-2xs text-[var(--text-faint)]">
                  {importType === "hosts" ? t("settings.importer.withCreds") : t("settings.importer.withOwner")}
                </div>
              </div>
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--text-secondary)] font-mono">
                  {new Set(fileData.flatMap(d => (d.tags as string[]) || [])).size}
                </div>
                <div className="text-2xs text-[var(--text-faint)]">{t("common.tags")}</div>
              </div>
            </div>

            {detectedType && detectedType !== importType && (
              <div className="p-2.5 rounded-[var(--radius-md)] bg-[var(--warning)]/10 border border-[var(--warning)]/25 text-[var(--warning)] text-xs">
                {t("settings.importer.typeMismatch", { detected: detectedType, current: importType })}
                <button onClick={() => setImportType(detectedType as "hosts" | "dns")} className="ml-1 underline hover:text-[var(--warning)]">
                  {t("settings.importer.switchTo", { type: detectedType })}
                </button>
              </div>
            )}
          </div>
        )}

        {parseError && <FormError message={parseError} />}
      </Card>

      {/* Import button + results */}
      {fileData && !result && (
        <div className="animate-slide-up" style={{ animationFillMode: "both" }}>
          <Button onClick={handleImport} loading={importing} className="w-full">
            {t("settings.importer.importButton", { count: String(fileData.length), type: importType === "hosts" ? t("nav.hosts") : t("settings.importer.dnsRecords") })}
          </Button>
        </div>
      )}

      {result && (
        <Card hover={false} className="animate-slide-up" style={{ animationFillMode: "both" } as React.CSSProperties}>
          <h3 className="text-xs font-semibold mb-3 font-display" style={{ color: "var(--text-muted)" }}>
            {t("settings.importer.results")}
          </h3>
          <div className="grid grid-cols-3 gap-3 mb-4">
            <div className="p-3 rounded-[var(--radius-md)] bg-[var(--success)]/10 border border-[var(--success)]/25 text-center">
              <div className="text-lg font-bold text-[var(--success)] font-mono">{result.created}</div>
              <div className="text-2xs text-[var(--success)]/70">{t("settings.importer.created")}</div>
            </div>
            <div className="p-3 rounded-[var(--radius-md)] bg-[var(--warning)]/10 border border-[var(--warning)]/25 text-center">
              <div className="text-lg font-bold text-[var(--warning)] font-mono">{result.skipped}</div>
              <div className="text-2xs text-[var(--warning)]/70">{t("settings.importer.skipped")}</div>
            </div>
            <div className="p-3 rounded-[var(--radius-md)] bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-center">
              <div className="text-lg font-bold text-[var(--danger)] font-mono">{result.failed}</div>
              <div className="text-2xs text-[var(--danger)]/70">{t("settings.importer.failed")}</div>
            </div>
          </div>

          {result.errors && result.errors.length > 0 && (
            <div className="max-h-48 overflow-y-auto space-y-1">
              {result.errors.map((err, i) => (
                <div key={i} className="flex items-start gap-2 text-xs p-2 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)]">
                  <span className="text-[var(--text-faint)] shrink-0 tabular-nums font-mono">#{err.index}</span>
                  <span className="text-[var(--text-secondary)] truncate font-mono">{err.name}</span>
                  <span className="text-[var(--text-faint)] shrink-0">{err.error}</span>
                </div>
              ))}
            </div>
          )}

          <div className="mt-4">
            <Button size="sm" variant="secondary" onClick={reset}>{t("settings.importer.another")}</Button>
          </div>
        </Card>
      )}
    </div>
  );
}

function BackupSection() {
  const { t } = useLocale();
  const { logout } = useAuth();
  const router = useRouter();
  const restoreInputRef = useRef<HTMLInputElement>(null);
  const [downloading, setDownloading] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);

  const handleBackup = async () => {
    setDownloading(true);
    setResult(null);
    try {
      await backupAPI.download();
      setResult({ ok: true, message: t("settings.backupSuccess") });
    } catch (err) {
      setResult({ ok: false, message: err instanceof Error ? err.message : "Backup failed" });
    } finally {
      setDownloading(false);
    }
  };

  const handleRestore = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!confirm(t("settings.restoreConfirm"))) {
      if (restoreInputRef.current) restoreInputRef.current.value = "";
      return;
    }
    setRestoring(true);
    setResult(null);
    try {
      const res = await backupAPI.restore(file);
      const parts: string[] = [res.message];
      if (res.row_count != null) {
        parts.push(`${res.row_count} rows restored.`);
      }
      if (res.cross_dialect && res.source_dialect && res.target_dialect) {
        parts.push(`Cross-dialect: ${res.source_dialect} → ${res.target_dialect}.`);
      }
      parts.push(t("settings.restoreSuccessLogout"));
      setResult({ ok: true, message: parts.join(" ") });
      if (restoreInputRef.current) restoreInputRef.current.value = "";
      // The restore wiped the users and sessions tables, so the current
      // session cookie now points to nothing. Give the user a moment to
      // read the success banner, then force a clean re-login.
      setTimeout(async () => {
        try {
          await logout();
        } catch {
          // Server-side session is already gone — ignore and continue.
        }
        router.push("/login");
      }, 2500);
      return;
    } catch (err) {
      setResult({ ok: false, message: err instanceof Error ? err.message : "Restore failed" });
    } finally {
      setRestoring(false);
      if (restoreInputRef.current) restoreInputRef.current.value = "";
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      {/* Backup */}
      <Card hover={false} className="stagger-in" style={{ "--i": 0 } as React.CSSProperties}>
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 rounded-[var(--radius-md)] bg-[var(--success)]/10 border border-[var(--success)]/20 flex items-center justify-center shrink-0">
            <Icon path={ICON_PATHS.exportDoc} className="w-5 h-5 text-[var(--success)]" />
          </div>
          <div className="flex-1">
            <h3 className="text-sm font-semibold text-[var(--text-primary)] font-display">
              {t("settings.backupTitle")}
            </h3>
            <p className="text-xs text-[var(--text-faint)] mt-1 mb-3">
              {t("settings.backupDescription")}
            </p>
            <Button size="sm" variant="secondary" onClick={handleBackup} loading={downloading}>
              <Icon path={ICON_PATHS.exportDoc} className="w-3.5 h-3.5 mr-1.5" />
              {t("settings.downloadBackup")}
            </Button>
          </div>
        </div>
      </Card>

      {/* Restore */}
      <Card hover={false} className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 rounded-[var(--radius-md)] bg-[var(--warning)]/10 border border-[var(--warning)]/20 flex items-center justify-center shrink-0">
            <Icon path={ICON_PATHS.upload} className="w-5 h-5 text-[var(--warning)]" />
          </div>
          <div className="flex-1">
            <h3 className="text-sm font-semibold text-[var(--text-primary)] font-display">
              {t("settings.restoreTitle")}
            </h3>
            <p className="text-xs text-[var(--text-faint)] mt-1 mb-1">
              {t("settings.restoreDescription")}
            </p>
            <p className="text-xs text-[var(--danger)]/80 mb-3">
              {t("settings.restoreWarning")}
            </p>
            <input
              ref={restoreInputRef}
              type="file"
              accept=".sshcmbak,.db,.sqlite,.sqlite3,.gz,application/gzip"
              onChange={handleRestore}
              className="hidden"
            />
            <Button size="sm" variant="danger" onClick={() => restoreInputRef.current?.click()} loading={restoring}>
              <Icon path={ICON_PATHS.upload} className="w-3.5 h-3.5 mr-1.5" />
              {t("settings.uploadRestore")}
            </Button>
          </div>
        </div>
      </Card>

      {/* Result message */}
      {result && (
        <div className={`p-3 rounded-[var(--radius-md)] text-sm animate-slide-up ${
          result.ok
            ? "bg-[var(--success)]/10 border border-[var(--success)]/25 text-[var(--success)]"
            : "bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)]"
        }`}>
          {result.message}
        </div>
      )}
    </div>
  );
}
