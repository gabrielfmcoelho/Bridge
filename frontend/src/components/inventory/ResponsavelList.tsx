"use client";

import { useState, useRef, useEffect } from "react";
import type { EntityResponsavel, Contact } from "@/lib/types";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Checkbox from "@/components/ui/Checkbox";
import IconButton from "@/components/ui/IconButton";
import Button from "@/components/ui/Button";

interface ResponsavelListProps {
  value: EntityResponsavel[];
  onChange: (v: EntityResponsavel[]) => void;
  contacts: Contact[];
  entidades: { value: string }[];
  t: (k: string) => string;
}

function toRawDigits(val: string): string {
  return val.replace(/\D/g, "");
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

function ContactAutocomplete({
  value,
  onChange,
  onSelect,
  contacts,
  label,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  onSelect: (c: Contact) => void;
  contacts: Contact[];
  label?: string;
  placeholder?: string;
}) {
  const [showSuggestions, setShowSuggestions] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const filtered = value
    ? contacts
        .filter(
          (c) =>
            c.name &&
            c.name.toLowerCase().includes(value.toLowerCase()) &&
            c.name !== value
        )
        .slice(0, 6)
    : [];

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (target.closest("[data-portal-dropdown]")) return;
      if (ref.current && !ref.current.contains(target)) {
        setShowSuggestions(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  return (
    <div className="space-y-1.5 relative" ref={ref}>
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <input
        type="text"
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
          setShowSuggestions(true);
        }}
        onFocus={() => setShowSuggestions(true)}
        className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
        placeholder={placeholder}
      />
      {showSuggestions && filtered.length > 0 && (
        <div className="absolute z-20 left-0 right-0 mt-1 bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg max-h-40 overflow-y-auto">
          {filtered.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => {
                onSelect(c);
                setShowSuggestions(false);
              }}
              className="w-full text-left px-3 py-1.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
            >
              <span>{c.name}</span>
              {c.phone && (
                <span className="text-[var(--text-faint)] ml-2 text-xs">
                  {formatPhone(c.phone)}
                </span>
              )}
              {c.entity && (
                <span className="text-[var(--text-faint)] ml-2 text-xs">
                  ({c.entity})
                </span>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

export default function ResponsavelList({
  value,
  onChange,
  contacts,
  entidades,
  t,
}: ResponsavelListProps) {
  const empty: EntityResponsavel = {
    is_main: value.length === 0,
    is_externo: false,
    name: "",
    phone: "",
    role: "",
    entity: "",
  };

  const update = (idx: number, patch: Partial<EntityResponsavel>) => {
    const next = value.map((item, i) => (i === idx ? { ...item, ...patch } : item));
    onChange(next);
  };

  const remove = (idx: number) => {
    const next = value.filter((_, i) => i !== idx);
    if (next.length > 0 && !next.some((r) => r.is_main)) {
      next[0] = { ...next[0], is_main: true };
    }
    onChange(next);
  };

  const add = () => {
    onChange([...value, { ...empty, is_main: value.length === 0 }]);
  };

  const setMain = (idx: number) => {
    const next = value.map((item, i) => ({
      ...item,
      is_main: i === idx,
    }));
    onChange(next);
  };

  const handleContactSelect = (idx: number, contact: Contact) => {
    update(idx, {
      contact_id: contact.id,
      name: contact.name,
      phone: contact.phone,
      role: contact.role,
      entity: contact.entity,
    });
  };

  const handlePhoneChange = (idx: number, inputValue: string) => {
    const raw = toRawDigits(inputValue).slice(0, 13);
    update(idx, { phone: raw });
  };

  const entidadeOptions = entidades.map((e) => ({ value: e.value, label: e.value }));

  return (
    <div className="space-y-3">
      {value.map((item, idx) => (
        <div key={idx} className="relative pb-3 mb-3 border-b border-[var(--border-subtle)] last:border-0 last:mb-0 last:pb-0">
          <div className="absolute top-0 right-0">
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

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pr-8">
            <ContactAutocomplete
              label={t("responsavel.name")}
              value={item.name}
              onChange={(v) => {
                const match = contacts.find(
                  (c) => c.name.toLowerCase() === v.toLowerCase()
                );
                if (match) {
                  update(idx, {
                    name: v,
                    contact_id: match.id,
                    phone: match.phone || item.phone,
                    role: match.role || item.role,
                    entity: match.entity || item.entity,
                  });
                } else {
                  update(idx, { name: v, contact_id: undefined });
                }
              }}
              onSelect={(c) => handleContactSelect(idx, c)}
              contacts={contacts}
              placeholder={t("responsavel.searchPlaceholder")}
            />

            <div className="space-y-1.5">
              <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
                {t("responsavel.phone")}
              </label>
              <input
                type="tel"
                value={formatPhone(item.phone)}
                onChange={(e) => handlePhoneChange(idx, e.target.value)}
                placeholder="(XX) XX 9 XXXX-XXXX"
                className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
              />
            </div>

            <Input
              label={t("responsavel.role")}
              value={item.role}
              onChange={(e) => update(idx, { role: e.target.value })}
            />

            <div className="relative z-10">
              <Select
                label={t("responsavel.entity")}
                value={item.entity}
                options={entidadeOptions}
                onChange={(e) => update(idx, { entity: e.target.value })}
              />
            </div>
          </div>

          <div className="flex items-center gap-4 mt-2">
            <button
              type="button"
              onClick={() => setMain(idx)}
              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                item.is_main
                  ? "bg-[var(--accent)] text-white"
                  : "bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)] border border-[var(--border-default)]"
              }`}
            >
              {item.is_main && (
                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
              )}
              {t("responsavel.main")}
            </button>

            <Checkbox
              label={t("responsavel.external")}
              checked={item.is_externo}
              onChange={(checked) => update(idx, { is_externo: checked })}
            />
          </div>
        </div>
      ))}

      <Button type="button" variant="secondary" size="sm" onClick={add}>
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
        </svg>
        {t("responsavel.add")}
      </Button>
    </div>
  );
}
