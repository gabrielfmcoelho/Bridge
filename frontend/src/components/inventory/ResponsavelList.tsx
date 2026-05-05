"use client";

import { useMemo } from "react";
import type { EntityResponsavel, Contact } from "@/lib/types";
import Select from "@/components/ui/Select";
import IconButton from "@/components/ui/IconButton";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";

interface ResponsavelListProps {
  value: EntityResponsavel[];
  onChange: (v: EntityResponsavel[]) => void;
  contacts: Contact[];
  t: (k: string) => string;
}

function formatPhone(raw: string): string {
  const d = raw.replace(/\D/g, "");
  if (d.length === 0) return "";
  if (d.length <= 2) return `(${d}`;
  if (d.length <= 4) return `(${d.slice(0, 2)}) ${d.slice(2)}`;
  if (d.length <= 5) return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4)}`;
  if (d.length <= 9) return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4, 5)} ${d.slice(5)}`;
  return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4, 5)} ${d.slice(5, 9)}-${d.slice(9, 13)}`;
}

function fromContact(c: Contact, isMain: boolean): EntityResponsavel {
  return {
    contact_id: c.id,
    is_main: isMain,
    name: c.name,
    phone: c.phone,
    role: c.role,
    entity: c.entity,
    notes: c.notes,
    is_external: c.is_external,
  };
}

export default function ResponsavelList({ value, onChange, contacts, t }: ResponsavelListProps) {
  const linkedIds = useMemo(() => new Set(value.map((v) => v.contact_id)), [value]);
  const availableContacts = useMemo(
    () => contacts.filter((c) => !linkedIds.has(c.id)),
    [contacts, linkedIds]
  );

  const options = [
    { value: "", label: t("responsavel.select") },
    ...availableContacts.map((c) => ({
      value: String(c.id),
      label: c.entity ? `${c.name} — ${c.entity}` : c.name,
    })),
  ];

  const setMain = (idx: number) => {
    onChange(value.map((item, i) => ({ ...item, is_main: i === idx })));
  };

  const remove = (idx: number) => {
    const next = value.filter((_, i) => i !== idx);
    if (next.length > 0 && !next.some((r) => r.is_main)) {
      next[0] = { ...next[0], is_main: true };
    }
    onChange(next);
  };

  const addContact = (contactId: number) => {
    const c = contacts.find((x) => x.id === contactId);
    if (!c) return;
    const isMain = value.length === 0;
    onChange([...value, fromContact(c, isMain)]);
  };

  if (contacts.length === 0) {
    return (
      <EmptyState
        icon="folder"
        title={t("responsavel.noContacts")}
        compact
      />
    );
  }

  return (
    <div className="space-y-2">
      {value.map((item, idx) => (
        <div
          key={item.contact_id}
          className="flex items-start gap-3 p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)]"
        >
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm font-medium text-[var(--text-primary)]">{item.name}</span>
              {item.is_external && <Badge color="amber">{t("responsavel.external")}</Badge>}
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-x-4 gap-y-1 text-xs text-[var(--text-muted)]">
              {item.phone && (
                <div>
                  <span className="text-[var(--text-faint)] block">{t("responsavel.phone")}</span>
                  <span style={{ fontFamily: "var(--font-mono)" }}>{formatPhone(item.phone)}</span>
                </div>
              )}
              {item.role && (
                <div>
                  <span className="text-[var(--text-faint)] block">{t("responsavel.role")}</span>
                  <span>{item.role}</span>
                </div>
              )}
              {item.entity && (
                <div>
                  <span className="text-[var(--text-faint)] block">{t("responsavel.entity")}</span>
                  <span>{item.entity}</span>
                </div>
              )}
            </div>
          </div>

          <div className="flex flex-col items-end gap-2 shrink-0">
            <button
              type="button"
              onClick={() => setMain(idx)}
              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                item.is_main
                  ? "bg-[var(--accent)] text-white"
                  : "bg-[var(--bg-surface)] text-[var(--text-muted)] hover:text-[var(--text-secondary)] border border-[var(--border-default)]"
              }`}
            >
              {item.is_main && (
                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
              )}
              {t("responsavel.main")}
            </button>
            <IconButton
              variant="danger"
              size="sm"
              type="button"
              onClick={() => remove(idx)}
              title={t("common.remove")}
            >
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </IconButton>
          </div>
        </div>
      ))}

      {availableContacts.length > 0 ? (
        <div className="flex items-center gap-2">
          <div className="flex-1">
            <Select
              value=""
              options={options}
              onChange={(e) => {
                const id = Number(e.target.value);
                if (id) addContact(id);
              }}
            />
          </div>
          <Button type="button" variant="secondary" size="sm" disabled>
            {t("responsavel.add")}
          </Button>
        </div>
      ) : (
        value.length > 0 && (
          <p className="text-xs text-[var(--text-faint)]">{t("responsavel.alreadyLinked")}</p>
        )
      )}
    </div>
  );
}
