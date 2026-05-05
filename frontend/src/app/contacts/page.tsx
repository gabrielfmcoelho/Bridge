"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { contactsAPI, enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Checkbox from "@/components/ui/Checkbox";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import PageHeader from "@/components/ui/PageHeader";
import FormError from "@/components/ui/FormError";
import EmptyState from "@/components/ui/EmptyState";
import Badge from "@/components/ui/Badge";
import { SkeletonCard } from "@/components/ui/Skeleton";
import type { Contact } from "@/lib/types";

function formatPhone(raw: string): string {
  const d = raw.replace(/\D/g, "");
  if (d.length === 0) return "";
  if (d.length <= 2) return `(${d}`;
  if (d.length <= 4) return `(${d.slice(0, 2)}) ${d.slice(2)}`;
  if (d.length <= 5) return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4)}`;
  if (d.length <= 9) return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4, 5)} ${d.slice(5)}`;
  return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4, 5)} ${d.slice(5, 9)}-${d.slice(9, 13)}`;
}

function toRawDigits(val: string): string {
  return val.replace(/\D/g, "");
}

export default function ContactsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Contact | null>(null);
  const [search, setSearch] = useState("");
  const canEdit = user?.role === "admin" || user?.role === "editor";

  const { data: contacts = [], isLoading } = useQuery({
    queryKey: ["contacts"],
    queryFn: contactsAPI.list,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => contactsAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["contacts"] }),
  });

  const filtered = contacts.filter((c) => {
    const q = search.toLowerCase();
    return (
      c.name.toLowerCase().includes(q) ||
      c.phone.includes(search) ||
      c.role.toLowerCase().includes(q) ||
      c.entity.toLowerCase().includes(q)
    );
  });

  return (
    <PageShell>
      <PageHeader title="Contacts" addLabel={canEdit ? t("common.add") : undefined} onAdd={canEdit ? () => { setEditing(null); setShowForm(true); } : undefined} />

      <div className="mb-5 max-w-xs">
        <Input
          placeholder={t("common.search")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 3 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon="folder"
          title="No contacts"
          description={search ? "Try adjusting your search" : "Add contacts to reuse across hosts, projects, and services."}
          action={canEdit && !search ? <Button size="sm" onClick={() => setShowForm(true)}>+ {t("common.add")}</Button> : undefined}
        />
      ) : (
        <div className="border border-[var(--border-default)] rounded-[var(--radius-lg)] overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-[var(--bg-elevated)] text-[var(--text-muted)] text-xs">
                <th className="text-left px-4 py-2.5 font-medium">{t("responsavel.name")}</th>
                <th className="text-left px-4 py-2.5 font-medium">{t("responsavel.phone")}</th>
                <th className="text-left px-4 py-2.5 font-medium">{t("responsavel.role")}</th>
                <th className="text-left px-4 py-2.5 font-medium">{t("responsavel.entity")}</th>
                <th className="text-left px-4 py-2.5 font-medium w-24">{t("responsavel.external")}</th>
                {canEdit && <th className="text-right px-4 py-2.5 font-medium w-24">Actions</th>}
              </tr>
            </thead>
            <tbody>
              {filtered.map((c, i) => (
                <tr key={c.id} className={`border-t border-[var(--border-subtle)] ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}>
                  <td className="px-4 py-2.5 text-[var(--text-primary)] font-medium">{c.name}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{c.phone ? formatPhone(c.phone) : "-"}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{c.role || "-"}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{c.entity || "-"}</td>
                  <td className="px-4 py-2.5">{c.is_external && <Badge color="amber">{t("responsavel.external")}</Badge>}</td>
                  {canEdit && (
                    <td className="px-4 py-2.5 text-right">
                      <div className="flex justify-end gap-1">
                        <Button size="sm" variant="secondary" onClick={() => { setEditing(c); setShowForm(true); }}>
                          {t("common.edit")}
                        </Button>
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={() => { if (confirm(`Delete "${c.name}"?`)) deleteMutation.mutate(c.id); }}
                        >
                          {t("common.delete")}
                        </Button>
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ResponsiveModal open={showForm} onClose={() => setShowForm(false)} title={editing ? "Edit Contact" : "Add Contact"}>
        <ContactForm
          initial={editing}
          onSuccess={() => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["contacts"] });
          }}
        />
      </ResponsiveModal>
    </PageShell>
  );
}

function ContactForm({ initial, onSuccess }: { initial: Contact | null; onSuccess: () => void }) {
  const { t } = useLocale();
  const [name, setName] = useState(initial?.name ?? "");
  const [phone, setPhone] = useState(initial?.phone ?? "");
  const [role, setRole] = useState(initial?.role ?? "");
  const [entity, setEntity] = useState(initial?.entity ?? "");
  const [notes, setNotes] = useState(initial?.notes ?? "");
  const [isExternal, setIsExternal] = useState(initial?.is_external ?? false);
  const [error, setError] = useState("");
  const [phoneError, setPhoneError] = useState("");

  const { data: entidades = [] } = useQuery({
    queryKey: ["enums", "entidade_responsavel"],
    queryFn: () => enumsAPI.list("entidade_responsavel"),
  });

  const validatePhone = (raw: string) => {
    if (!raw) { setPhoneError(""); return true; }
    if (raw.length < 10) { setPhoneError("Phone too short"); return false; }
    if (raw.length > 13) { setPhoneError("Phone too long"); return false; }
    setPhoneError("");
    return true;
  };

  const handlePhoneChange = (inputValue: string) => {
    const raw = toRawDigits(inputValue).slice(0, 13);
    setPhone(raw);
    validatePhone(raw);
  };

  const mutation = useMutation({
    mutationFn: () => {
      const payload = { name, phone, role, entity, notes, is_external: isExternal };
      return initial ? contactsAPI.update(initial.id, payload) : contactsAPI.create(payload);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const entidadeOptions = [
    { value: "", label: "—" },
    ...entidades.map((e) => ({ value: e.value, label: e.value })),
  ];

  return (
    <form onSubmit={(e) => {
      e.preventDefault();
      if (!validatePhone(phone)) return;
      mutation.mutate();
    }} className="space-y-4">
      <FormError message={error} />
      <Input label={t("responsavel.name")} value={name} onChange={(e) => setName(e.target.value)} required />
      <Input
        label={t("responsavel.phone")}
        type="tel"
        value={formatPhone(phone)}
        onChange={(e) => handlePhoneChange(e.target.value)}
        error={phoneError}
        placeholder="(XX) XX 9 XXXX-XXXX"
      />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Input label={t("responsavel.role")} value={role} onChange={(e) => setRole(e.target.value)} />
        <Select
          label={t("responsavel.entity")}
          value={entity}
          options={entidadeOptions}
          onChange={(e) => setEntity(e.target.value)}
        />
      </div>
      <div className="space-y-1.5">
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {t("responsavel.notes")}
        </label>
        <textarea
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          rows={3}
          className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
        />
      </div>
      <Checkbox
        label={t("responsavel.external")}
        checked={isExternal}
        onChange={setIsExternal}
      />
      <div className="flex justify-end gap-2">
        <Button type="submit" loading={mutation.isPending}>
          {initial ? t("common.save") : t("common.create")}
        </Button>
      </div>
    </form>
  );
}
