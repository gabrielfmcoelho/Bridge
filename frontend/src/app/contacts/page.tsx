"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { contactsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import PageHeader from "@/components/ui/PageHeader";
import FormError from "@/components/ui/FormError";
import EmptyState from "@/components/ui/EmptyState";
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

  const filtered = contacts.filter(
    (c) =>
      c.name.toLowerCase().includes(search.toLowerCase()) ||
      c.phone.includes(search)
  );

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
                <th className="text-left px-4 py-2.5 font-medium">Name</th>
                <th className="text-left px-4 py-2.5 font-medium">Phone</th>
                {canEdit && <th className="text-right px-4 py-2.5 font-medium w-24">Actions</th>}
              </tr>
            </thead>
            <tbody>
              {filtered.map((c, i) => (
                <tr key={c.id} className={`border-t border-[var(--border-subtle)] ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}>
                  <td className="px-4 py-2.5 text-[var(--text-primary)] font-medium">{c.name}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{c.phone ? formatPhone(c.phone) : "-"}</td>
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
  const [phone, setPhone] = useState(initial?.phone ?? ""); // raw digits
  const [error, setError] = useState("");
  const [phoneError, setPhoneError] = useState("");

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
      if (initial) {
        return contactsAPI.delete(initial.id).then(() =>
          contactsAPI.create({ name, phone })
        );
      }
      return contactsAPI.create({ name, phone });
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  return (
    <form onSubmit={(e) => {
      e.preventDefault();
      if (!validatePhone(phone)) return;
      mutation.mutate();
    }} className="space-y-4">
      <FormError message={error} />
      <Input label="Name" value={name} onChange={(e) => setName(e.target.value)} required />
      <Input
        label="Phone"
        type="tel"
        value={formatPhone(phone)}
        onChange={(e) => handlePhoneChange(e.target.value)}
        error={phoneError}
        placeholder="(XX) XX 9 XXXX-XXXX"
      />
      <div className="flex justify-end gap-2">
        <Button type="submit" loading={mutation.isPending}>
          {initial ? t("common.save") : t("common.create")}
        </Button>
      </div>
    </form>
  );
}
