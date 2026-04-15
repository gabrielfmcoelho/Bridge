"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useRef } from "react";
import { enumsAPI, usersAPI, appearanceAPI, importAPI, backupAPI } from "@/lib/api";
import type { ImportResult } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import PageShell from "@/components/layout/PageShell";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Badge from "@/components/ui/Badge";
import Drawer from "@/components/ui/Drawer";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormError from "@/components/ui/FormError";
import IntegrationsTab from "./IntegrationsTab";
import PermissionsTab from "./PermissionsTab";
import RoleMappingsTab from "./RoleMappingsTab";

type Tab = "enums" | "users" | "appearance" | "import" | "backup" | "integrations" | "permissions" | "role-mappings";

const roleColors: Record<string, string> = {
  admin: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]",
  editor: "bg-purple-500/10 text-purple-400/70 border-purple-500/15",
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
  const [activeTab, setActiveTab] = useState<Tab>("enums");
  const [showTabDrawer, setShowTabDrawer] = useState(false);

  const tabs: { key: Tab; label: string }[] = [
    { key: "enums", label: t("settings.enums") },
    ...(isAdmin ? [{ key: "users" as Tab, label: t("settings.users") }] : []),
    ...(isAdmin ? [{ key: "appearance" as Tab, label: t("settings.appearance") }] : []),
    ...(isAdmin ? [{ key: "import" as Tab, label: t("settings.import") }] : []),
    ...(isAdmin ? [{ key: "backup" as Tab, label: t("settings.backup") }] : []),
    ...(isAdmin ? [{ key: "integrations" as Tab, label: "Integrations" }] : []),
    ...(isAdmin ? [{ key: "permissions" as Tab, label: "Permissions" }] : []),
    ...(isAdmin ? [{ key: "role-mappings" as Tab, label: "Role Mappings" }] : []),
  ];

  const activeLabel = tabs.find((t) => t.key === activeTab)?.label ?? "";

  return (
    <PageShell>
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("settings.title")}</h1>

        {/* Mobile: tab selector button */}
        <button
          onClick={() => setShowTabDrawer(true)}
          className="md:hidden flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-default)] text-[var(--text-primary)]"
        >
          {activeLabel}
          <svg className="w-4 h-4 text-[var(--text-muted)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
      </div>

      {/* Desktop: scrollable tab bar */}
      <div className="hidden md:flex gap-1 mb-6 p-1 rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] overflow-x-auto">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-4 py-2 text-sm font-medium rounded-[var(--radius-sm)] transition-all duration-150 whitespace-nowrap ${
              activeTab === tab.key
                ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Mobile: tab drawer */}
      <Drawer open={showTabDrawer} onClose={() => setShowTabDrawer(false)} title={t("settings.title")}>
        <div className="p-2 space-y-1">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => { setActiveTab(tab.key); setShowTabDrawer(false); }}
              className={`w-full text-left px-4 py-3 text-sm font-medium rounded-[var(--radius-md)] transition-all ${
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
      if (!editing) throw new Error("No item selected");
      return enumsAPI.update(editing.category, editing.value, editValue, editColor);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["enums"] });
      setEditing(null);
      setEditError("");
    },
    onError: (err) => setEditError(err instanceof Error ? err.message : "Failed"),
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
        <Card key={category} hover={false} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" } as React.CSSProperties}>
          <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
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
                    className="opacity-0 group-hover:opacity-100 text-[var(--text-faint)] hover:text-red-400 text-xs transition-opacity"
                  >
                    &times;
                  </button>
                )}
              </span>
            ))}
            {options.length === 0 && (
              <span className="text-xs text-[var(--text-faint)]">No values defined</span>
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
                placeholder={`New ${category} value`}
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
                    title="Status color"
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
          <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
            + New Category
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
              placeholder="Category name"
              className="max-w-[180px]"
            />
            <Input
              value={newCategoryValue}
              onChange={(e) => setNewCategoryValue(e.target.value)}
              placeholder="First value"
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
              <div className="bg-red-500/10 border border-red-500/25 text-red-400 text-sm rounded-[var(--radius-md)] p-3 animate-slide-down">{editError}</div>
            )}
            <Input
              label="Value"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              required
            />
            {editing.category === "situacao" && (
              <Input
                label="Color"
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
  return (
    <div className="flex items-center gap-1">
      <button
        onClick={onEdit}
        className="p-1.5 rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-overlay)] transition-colors"
        title="Edit"
      >
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z" />
        </svg>
      </button>
      <button
        onClick={onDelete}
        className="p-1.5 rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-red-400 hover:bg-red-500/10 transition-colors"
        title="Delete"
      >
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
        </svg>
      </button>
    </div>
  );
}

function UsersSection() {
  const { t, formatDate } = useLocale();
  const queryClient = useQueryClient();
  const [viewMode, setViewMode] = useState<"cards" | "table">("table");
  const [showForm, setShowForm] = useState(false);
  const [editUser, setEditUser] = useState<import("@/lib/types").User | null>(null);
  const [newUser, setNewUser] = useState({ username: "", password: "", display_name: "", role: "viewer" });
  const [editForm, setEditForm] = useState({ display_name: "", role: "", password: "" });
  const [error, setError] = useState("");

  const { data: users = [] } = useQuery({
    queryKey: ["users"],
    queryFn: usersAPI.list,
  });

  const createMutation = useMutation({
    mutationFn: () => usersAPI.create(newUser),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      setShowForm(false);
      setNewUser({ username: "", password: "", display_name: "", role: "viewer" });
      setError("");
    },
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editUser) return Promise.reject();
      const data: Record<string, string> = {};
      if (editForm.display_name !== editUser.display_name) data.display_name = editForm.display_name;
      if (editForm.role !== editUser.role) data.role = editForm.role;
      if (editForm.password) data.password = editForm.password;
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
    setEditForm({ display_name: u.display_name, role: u.role, password: "" });
    setError("");
  };

  const confirmDelete = (u: import("@/lib/types").User) => {
    if (confirm(`Delete user ${u.username}?`)) deleteMutation.mutate(u.id);
  };

  const roleAccentColor: Record<string, string> = {
    admin: "var(--accent)",
    editor: "#a78bfa",
    viewer: "#6b7280",
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold" style={{ color: "var(--text-secondary)", fontFamily: "var(--font-display)" }}>
          {t("settings.users")}
        </h2>
        <div className="flex items-center gap-1.5">
          {/* View toggle */}
          <div className="hidden sm:flex border border-[var(--border-default)] rounded-[var(--radius-md)] overflow-hidden">
            <button
              onClick={() => setViewMode("cards")}
              className={`px-2.5 py-1.5 transition-colors ${viewMode === "cards" ? "bg-[var(--accent-muted)] text-[var(--accent)]" : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"}`}
              title="Card view"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zm10 0a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zm10 0a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
              </svg>
            </button>
            <button
              onClick={() => setViewMode("table")}
              className={`px-2.5 py-1.5 transition-colors ${viewMode === "table" ? "bg-[var(--accent-muted)] text-[var(--accent)]" : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"}`}
              title="Table view"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
              </svg>
            </button>
          </div>
          <Button size="sm" onClick={() => setShowForm(true)}>+ {t("settings.addUser")}</Button>
        </div>
      </div>

      {/* Table view */}
      {viewMode === "table" ? (
        <div className="border border-[var(--border-default)] rounded-[var(--radius-lg)] overflow-x-auto animate-fade-in">
          <table className="w-full text-sm min-w-[600px]">
            <thead>
              <tr className="bg-[var(--bg-elevated)] text-[var(--text-muted)] text-[11px] uppercase tracking-wider">
                <th className="text-left px-4 py-3 font-semibold">{t("auth.username")}</th>
                <th className="text-left px-4 py-3 font-semibold">{t("settings.role")}</th>
                <th className="text-left px-4 py-3 font-semibold">{t("settings.authProvider")}</th>
                <th className="text-left px-4 py-3 font-semibold">{t("settings.createdAt")}</th>
                <th className="text-right px-4 py-3 w-20" />
              </tr>
            </thead>
            <tbody>
              {users.map((u, i) => (
                <tr
                  key={u.id}
                  className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
                >
                  <td className="px-4 py-2.5">
                    <div className="flex items-center gap-2.5">
                      <div className="w-8 h-8 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-xs font-semibold text-[var(--text-secondary)] shrink-0">
                        {(u.display_name || u.username).charAt(0).toUpperCase()}
                      </div>
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-[var(--text-primary)] truncate">{u.display_name || u.username}</p>
                        <p className="text-[11px] text-[var(--text-faint)]">@{u.username}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className={`text-[10px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[u.role] || roleColors.viewer}`}>
                      {u.role}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)] text-xs">
                    {providerLabels[u.auth_provider] || u.auth_provider || "Local"}
                  </td>
                  <td className="px-4 py-2.5 text-[var(--text-muted)] text-xs">
                    {u.created_at ? formatDate(u.created_at) : "-"}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <UserActions u={u} onEdit={() => openEdit(u)} onDelete={() => confirmDelete(u)} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        /* Card view */
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 animate-fade-in">
          {users.map((u, i) => (
            <Card
              key={u.id}
              className={`border-l-[3px] animate-slide-up stagger-${Math.min(i + 1, 9)}`}
              style={{ borderLeftColor: roleAccentColor[u.role] || roleAccentColor.viewer }}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex items-center gap-2.5 min-w-0">
                  <div className="w-9 h-9 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-sm font-semibold text-[var(--text-secondary)] shrink-0">
                    {(u.display_name || u.username).charAt(0).toUpperCase()}
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-[var(--text-primary)] truncate">{u.display_name || u.username}</p>
                    <p className="text-[11px] text-[var(--text-faint)]">@{u.username}</p>
                  </div>
                </div>
                <UserActions u={u} onEdit={() => openEdit(u)} onDelete={() => confirmDelete(u)} />
              </div>

              <div className="grid grid-cols-2 gap-x-4 gap-y-2 mt-3 pt-3 border-t border-[var(--border-subtle)]">
                <div>
                  <p className="text-[10px] text-[var(--text-faint)] uppercase tracking-wider">{t("settings.role")}</p>
                  <span className={`inline-block mt-0.5 text-[10px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[u.role] || roleColors.viewer}`}>
                    {u.role}
                  </span>
                </div>
                <div>
                  <p className="text-[10px] text-[var(--text-faint)] uppercase tracking-wider">{t("settings.authProvider")}</p>
                  <p className="text-xs text-[var(--text-secondary)] mt-0.5">{providerLabels[u.auth_provider] || u.auth_provider || "Local"}</p>
                </div>
                {u.email && (
                  <div className="col-span-2">
                    <p className="text-[10px] text-[var(--text-faint)] uppercase tracking-wider">Email</p>
                    <p className="text-xs text-[var(--text-secondary)] mt-0.5 truncate">{u.email}</p>
                  </div>
                )}
              </div>

              <div className="flex items-center justify-between mt-3 pt-2.5 border-t border-[var(--border-subtle)]">
                <span className="text-[10px] text-[var(--text-faint)]">
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
              <Input label={`${t("auth.password")} (optional)`} type="password" value={editForm.password} onChange={(e) => setEditForm({ ...editForm, password: e.target.value })} placeholder="Leave blank to keep current" />
            </div>
            <div className="flex justify-end">
              <Button type="submit" loading={updateMutation.isPending}>{t("common.save")}</Button>
            </div>
          </form>
        )}
      </ResponsiveModal>
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
      <Card hover={false} className="animate-slide-up stagger-1" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
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
      <Card hover={false} className="animate-slide-up stagger-2" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
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
              className="group relative w-8 h-8 rounded-[var(--radius-sm)] border-2 transition-all duration-150 hover:scale-110"
              style={{
                backgroundColor: c.value,
                borderColor: appColor === c.value ? "var(--text-primary)" : "transparent",
                boxShadow: appColor === c.value ? `0 0 12px ${c.value}40` : "none",
              }}
              title={c.label}
            >
              {appColor === c.value && (
                <svg className="w-4 h-4 text-white absolute inset-0 m-auto drop-shadow" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
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
      <Card hover={false} className="animate-slide-up stagger-3" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
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
              <img src={appLogo} alt="Logo" className="w-full h-full object-contain p-1" />
            ) : (
              <svg className="w-7 h-7" style={{ color: "var(--text-faint)" }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 17l6-6-6-6m8 14h8" />
              </svg>
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
                className="text-xs hover:text-red-400 transition-colors text-left"
                style={{ color: "var(--text-faint)" }}
              >
                {t("settings.removeLogo")}
              </button>
            )}
          </div>
        </div>
      </Card>

      {/* Save button */}
      <div className="flex items-center gap-3 animate-slide-up stagger-4" style={{ animationFillMode: "both" } as React.CSSProperties}>
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
      <Card hover={false} className="animate-slide-up stagger-1" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
          {t("settings.importType")}
        </h3>
        <div className="flex gap-2">
          <button
            onClick={() => setImportType("hosts")}
            className={`px-4 py-2 text-sm font-medium rounded-[var(--radius-md)] border transition-all ${
              importType === "hosts"
                ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
            }`}
          >
            <div className="flex items-center gap-2">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
              </svg>
              Hosts
            </div>
          </button>
          <button
            onClick={() => setImportType("dns")}
            className={`px-4 py-2 text-sm font-medium rounded-[var(--radius-md)] border transition-all ${
              importType === "dns"
                ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
            }`}
          >
            <div className="flex items-center gap-2">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9" />
              </svg>
              DNS
            </div>
          </button>
        </div>
      </Card>

      {/* File upload */}
      <Card hover={false} className="animate-slide-up stagger-2" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
          JSON File
        </h3>
        <p className="text-xs mb-3" style={{ color: "var(--text-faint)" }}>
          {importType === "hosts"
            ? "Upload a JSON array of host objects. Required fields: nickname, oficial_slug. Optional: hostname, user, password, tags, situacao, etc."
            : "Upload a JSON array of DNS objects. Required fields: domain. Optional: has_https, situacao, responsavel, tags, host_ids."
          }
        </p>

        {/* JSON example */}
        <details className="mb-4 group">
          <summary className="text-[11px] text-[var(--accent)] cursor-pointer hover:underline font-medium">
            Show JSON example
          </summary>
          <pre className="mt-2 p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-[11px] text-[var(--text-secondary)] overflow-x-auto" style={{ fontFamily: "var(--font-mono)" }}>
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
            className="w-full py-8 border-2 border-dashed border-[var(--border-default)] rounded-[var(--radius-lg)] hover:border-[var(--accent)] hover:bg-[var(--accent-muted)]/5 transition-all group"
          >
            <div className="flex flex-col items-center gap-2">
              <svg className="w-8 h-8 text-[var(--text-faint)] group-hover:text-[var(--accent)] transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
              </svg>
              <span className="text-sm text-[var(--text-muted)] group-hover:text-[var(--text-secondary)]">
                Click to select a .json file
              </span>
            </div>
          </button>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-default)]">
              <div className="flex items-center gap-2 min-w-0">
                <svg className="w-4 h-4 shrink-0 text-[var(--accent)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <span className="text-sm text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{fileName}</span>
              </div>
              <button onClick={reset} className="text-xs text-[var(--text-faint)] hover:text-red-400 transition-colors shrink-0 ml-2">
                Remove
              </button>
            </div>

            {/* Preview stats */}
            <div className="grid grid-cols-3 gap-3">
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{fileData.length}</div>
                <div className="text-[10px] uppercase tracking-wider text-[var(--text-faint)]">Records</div>
              </div>
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--accent)]" style={{ fontFamily: "var(--font-mono)" }}>
                  {importType === "hosts"
                    ? fileData.filter(d => d.user || d.password).length
                    : fileData.filter(d => d.responsavel).length
                  }
                </div>
                <div className="text-[10px] uppercase tracking-wider text-[var(--text-faint)]">
                  {importType === "hosts" ? "With Creds" : "With Owner"}
                </div>
              </div>
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>
                  {new Set(fileData.flatMap(d => (d.tags as string[]) || [])).size}
                </div>
                <div className="text-[10px] uppercase tracking-wider text-[var(--text-faint)]">Tags</div>
              </div>
            </div>

            {detectedType && detectedType !== importType && (
              <div className="p-2.5 rounded-[var(--radius-md)] bg-amber-500/10 border border-amber-500/25 text-amber-300 text-xs">
                File looks like <strong>{detectedType}</strong> data but import type is set to <strong>{importType}</strong>.
                <button onClick={() => setImportType(detectedType as "hosts" | "dns")} className="ml-1 underline hover:text-amber-200">
                  Switch to {detectedType}?
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
            Import {fileData.length} {importType === "hosts" ? "Hosts" : "DNS Records"}
          </Button>
        </div>
      )}

      {result && (
        <Card hover={false} className="animate-slide-up" style={{ animationFillMode: "both" } as React.CSSProperties}>
          <h3 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: "var(--text-muted)", fontFamily: "var(--font-display)" }}>
            Import Results
          </h3>
          <div className="grid grid-cols-3 gap-3 mb-4">
            <div className="p-3 rounded-[var(--radius-md)] bg-emerald-500/10 border border-emerald-500/25 text-center">
              <div className="text-lg font-bold text-emerald-400" style={{ fontFamily: "var(--font-mono)" }}>{result.created}</div>
              <div className="text-[10px] uppercase tracking-wider text-emerald-400/70">Created</div>
            </div>
            <div className="p-3 rounded-[var(--radius-md)] bg-amber-500/10 border border-amber-500/25 text-center">
              <div className="text-lg font-bold text-amber-400" style={{ fontFamily: "var(--font-mono)" }}>{result.skipped}</div>
              <div className="text-[10px] uppercase tracking-wider text-amber-400/70">Skipped</div>
            </div>
            <div className="p-3 rounded-[var(--radius-md)] bg-red-500/10 border border-red-500/25 text-center">
              <div className="text-lg font-bold text-red-400" style={{ fontFamily: "var(--font-mono)" }}>{result.failed}</div>
              <div className="text-[10px] uppercase tracking-wider text-red-400/70">Failed</div>
            </div>
          </div>

          {result.errors && result.errors.length > 0 && (
            <div className="max-h-48 overflow-y-auto space-y-1">
              {result.errors.map((err, i) => (
                <div key={i} className="flex items-start gap-2 text-xs p-2 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)]">
                  <span className="text-[var(--text-faint)] shrink-0 tabular-nums" style={{ fontFamily: "var(--font-mono)" }}>#{err.index}</span>
                  <span className="text-[var(--text-secondary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{err.name}</span>
                  <span className="text-[var(--text-faint)] shrink-0">{err.error}</span>
                </div>
              ))}
            </div>
          )}

          <div className="mt-4">
            <Button size="sm" variant="secondary" onClick={reset}>Import Another File</Button>
          </div>
        </Card>
      )}
    </div>
  );
}

function BackupSection() {
  const { t } = useLocale();
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
      setResult({ ok: true, message: parts.join(" ") });
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
      <Card hover={false} className="animate-slide-up stagger-1" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 rounded-[var(--radius-md)] bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center shrink-0">
            <svg className="w-5 h-5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
          </div>
          <div className="flex-1">
            <h3 className="text-sm font-semibold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>
              {t("settings.backupTitle")}
            </h3>
            <p className="text-xs text-[var(--text-faint)] mt-1 mb-3">
              {t("settings.backupDescription")}
            </p>
            <Button size="sm" variant="secondary" onClick={handleBackup} loading={downloading}>
              <svg className="w-3.5 h-3.5 mr-1.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              {t("settings.downloadBackup")}
            </Button>
          </div>
        </div>
      </Card>

      {/* Restore */}
      <Card hover={false} className="animate-slide-up stagger-2" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 rounded-[var(--radius-md)] bg-amber-500/10 border border-amber-500/20 flex items-center justify-center shrink-0">
            <svg className="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
            </svg>
          </div>
          <div className="flex-1">
            <h3 className="text-sm font-semibold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>
              {t("settings.restoreTitle")}
            </h3>
            <p className="text-xs text-[var(--text-faint)] mt-1 mb-1">
              {t("settings.restoreDescription")}
            </p>
            <p className="text-xs text-red-400/80 mb-3">
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
              <svg className="w-3.5 h-3.5 mr-1.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
              </svg>
              {t("settings.uploadRestore")}
            </Button>
          </div>
        </div>
      </Card>

      {/* Result message */}
      {result && (
        <div className={`p-3 rounded-[var(--radius-md)] text-sm animate-slide-up ${
          result.ok
            ? "bg-emerald-500/10 border border-emerald-500/25 text-emerald-400"
            : "bg-red-500/10 border border-red-500/25 text-red-400"
        }`}>
          {result.message}
        </div>
      )}
    </div>
  );
}
