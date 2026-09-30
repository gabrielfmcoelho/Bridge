"use client";

import { useId, useMemo, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { contactsAPI } from "@/lib/api";
import type { EntityResponsavel, Contact } from "@/lib/types";
import FormField, { INPUT_CLASS } from "@/components/ui/FormField";
import Input from "@/components/ui/Input";
import Checkbox from "@/components/ui/Checkbox";
import Button from "@/components/ui/Button";
import IconButton from "@/components/ui/IconButton";
import Badge from "@/components/ui/Badge";
import FormError from "@/components/ui/FormError";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { formatPhone, phoneDigits, phoneProblem } from "@/lib/phone";
import { contactErrorMessage } from "@/components/contacts/ContactForm";

type T = (k: string, v?: Record<string, string>) => string;
const MAX_RESULTS = 8;

function fromContact(c: Contact, isMain: boolean): EntityResponsavel {
  return { contact_id: c.id, is_main: isMain, name: c.name, phone: c.phone, role: c.role, entity: c.entity, notes: c.notes, is_external: c.is_external };
}
const metaOf = (c: { role?: string; phone?: string; entity?: string }) =>
  [c.role, c.phone ? formatPhone(c.phone) : "", c.entity].filter(Boolean).join(" · ");

/**
 * An asset's responsáveis: search a contact (name, role, phone) to add it,
 * or create one right here without leaving the form. One is the main one —
 * the name lists and cards show.
 */
export default function ResponsavelList({ value, onChange, contacts, t }: {
  value: EntityResponsavel[];
  onChange: (v: EntityResponsavel[]) => void;
  contacts: Contact[];
  t: T;
}) {
  const id = useId();
  const qc = useQueryClient();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [creating, setCreating] = useState<{ name: string; phone: string; email: string; is_external: boolean } | null>(null);
  const [error, setError] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const linked = useMemo(() => new Set(value.map((v) => v.contact_id)), [value]);
  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    const digits = q.replace(/\D/g, "");
    const pool = contacts.filter((c) => !linked.has(c.id));
    return (q ? pool.filter((c) => [c.name, c.role, c.entity].some((v) => v.toLowerCase().includes(q)) || (!!digits && c.phone.includes(digits))) : pool)
      .slice(0, MAX_RESULTS);
  }, [contacts, linked, query]);
  // The last option creates a contact from what was typed.
  const createIndex = results.length;
  const showList = open && (results.length > 0 || !!query.trim());

  const add = (c: Contact) => {
    onChange([...value, fromContact(c, value.length === 0)]);
    setQuery("");
    setActive(0);
    inputRef.current?.focus();
  };
  const setMain = (cid: number) => onChange(value.map((r) => ({ ...r, is_main: r.contact_id === cid })));
  const remove = (cid: number) => {
    const next = value.filter((r) => r.contact_id !== cid);
    if (next.length && !next.some((r) => r.is_main)) next[0] = { ...next[0], is_main: true };
    onChange(next);
  };
  const startCreate = () => {
    setCreating({ name: query.trim(), phone: "", email: "", is_external: false });
    setOpen(false);
    setError("");
  };

  const create = useMutation({
    mutationFn: () => contactsAPI.create({ ...creating!, name: creating!.name.trim(), email: creating!.email.trim() }),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: ["contacts"] });
      add(c);
      setCreating(null);
    },
    onError: (e) => {
      // Already there: link the existing one instead of failing.
      const existing = contacts.find((c) => c.name === creating!.name.trim() && c.phone === creating!.phone);
      if (existing && !linked.has(existing.id)) { add(existing); setCreating(null); return; }
      setError(contactErrorMessage(e, t));
    },
  });
  const createProblem = !creating?.name.trim() ? t("form.required") : phoneProblem(creating.phone) ? t(phoneProblem(creating.phone)) : "";

  return (
    <FormField label={t("responsavel.label")} htmlFor={id} hint={t("responsavel.pickerHint")}>
      {value.length > 0 && (
        <ul className="mb-2 divide-y divide-[var(--border-subtle)] rounded-[var(--radius-md)] border border-[var(--border-subtle)]">
          {value.map((r) => (
            <li key={r.contact_id} className="flex items-center gap-2 px-3 py-2">
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-1.5 text-sm text-[var(--text-primary)]">
                  <span className="truncate font-medium">{r.name}</span>
                  {r.is_external && <Badge color="warning">{t("responsavel.external")}</Badge>}
                </span>
                {metaOf(r) && <span className="block text-xs text-[var(--text-muted)] truncate">{metaOf(r)}</span>}
              </span>
              <button type="button" role="radio" aria-checked={!!r.is_main} onClick={() => setMain(r.contact_id)}
                className={`inline-flex items-center gap-1 h-7 px-2 rounded-full border text-xs shrink-0 transition-colors ${r.is_main
                  ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/30"
                  : "text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"}`}>
                <Icon path={ICON_PATHS.star} className="w-3 h-3" /> {t("responsavel.main")}
              </button>
              <IconButton label={t("common.removeItem", { label: r.name })} onClick={() => remove(r.contact_id)}>
                <Icon path={ICON_PATHS.close} />
              </IconButton>
            </li>
          ))}
        </ul>
      )}

      {creating ? (
        // Enter creates here; it must not submit the asset form around it.
        <div className="rounded-[var(--radius-md)] border border-[var(--border-default)] p-3 space-y-3"
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); if (!createProblem) create.mutate(); } }}>
          <p className="text-sm font-medium text-[var(--text-primary)]">{t("responsavel.newContact")}</p>
          <FormError message={error} />
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Input label={t("responsavel.name")} required value={creating.name} autoFocus
              onChange={(e) => setCreating({ ...creating, name: e.target.value })} />
            <Input label={t("responsavel.phone")} type="tel" value={formatPhone(creating.phone)} placeholder="(86) 9 9999-9999"
              onChange={(e) => setCreating({ ...creating, phone: phoneDigits(e.target.value) })} />
            <Input label={t("contact.email")} type="email" value={creating.email}
              onChange={(e) => setCreating({ ...creating, email: e.target.value })} />
            <div className="flex items-end pb-2">
              <Checkbox label={t("contact.externalLabel")} checked={creating.is_external} onChange={(v) => setCreating({ ...creating, is_external: v })} />
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" size="sm" variant="secondary" onClick={() => setCreating(null)}>{t("common.cancel")}</Button>
            <Button type="button" size="sm" loading={create.isPending} disabled={!!createProblem} title={createProblem || undefined}
              onClick={() => create.mutate()}>{t("responsavel.createAndLink")}</Button>
          </div>
        </div>
      ) : (
        <div className="relative">
          <Icon path={ICON_PATHS.search} className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-muted)] pointer-events-none" />
          <input
            ref={inputRef}
            id={id}
            role="combobox"
            aria-expanded={showList}
            aria-controls={`${id}-list`}
            aria-autocomplete="list"
            value={query}
            placeholder={t("responsavel.searchPlaceholder")}
            onChange={(e) => { setQuery(e.target.value); setOpen(true); setActive(0); }}
            onFocus={() => setOpen(true)}
            onBlur={() => setTimeout(() => setOpen(false), 120)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") { e.preventDefault(); setOpen(true); setActive((a) => Math.min(a + 1, createIndex)); }
              else if (e.key === "ArrowUp") { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)); }
              else if (e.key === "Enter") {
                e.preventDefault();
                if (!showList) return;
                if (active < results.length) add(results[active]); else if (query.trim()) startCreate();
              } else if (e.key === "Escape" && open) { e.stopPropagation(); setOpen(false); }
            }}
            className={`${INPUT_CLASS} pl-9`}
          />
          {showList && (
            <ul id={`${id}-list`} role="listbox"
              className="absolute z-20 mt-1 w-full max-h-72 overflow-y-auto rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-surface)] py-1 shadow-lg">
              {results.map((c, i) => (
                <li key={c.id} role="option" aria-selected={i === active}
                  onMouseDown={(e) => e.preventDefault()} onMouseEnter={() => setActive(i)} onClick={() => add(c)}
                  className={`px-3 py-1.5 cursor-pointer ${i === active ? "bg-[var(--bg-elevated)]" : ""}`}>
                  <span className="flex items-center gap-1.5 text-sm text-[var(--text-primary)]">
                    <span className="truncate">{c.name}</span>
                    {c.is_external && <Badge color="warning">{t("responsavel.external")}</Badge>}
                  </span>
                  {metaOf(c) && <span className="block text-xs text-[var(--text-muted)] truncate">{metaOf(c)}</span>}
                </li>
              ))}
              {query.trim() && (
                <li role="option" aria-selected={active === createIndex}
                  onMouseDown={(e) => e.preventDefault()} onMouseEnter={() => setActive(createIndex)} onClick={startCreate}
                  className={`flex items-center gap-2 px-3 py-2 text-sm cursor-pointer text-[var(--accent)] border-t border-[var(--border-subtle)] ${active === createIndex ? "bg-[var(--bg-elevated)]" : ""}`}>
                  <Icon path={ICON_PATHS.plus} className="w-3.5 h-3.5" /> {t("responsavel.createNamed", { name: query.trim() })}
                </li>
              )}
            </ul>
          )}
        </div>
      )}
    </FormField>
  );
}
