"use client";

import { useState } from "react";
import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import ContactDetailDrawer from "@/components/contacts/ContactDetailDrawer";
import { ICON_PATHS } from "@/lib/icon-paths";
import { formatPhone, whatsappNumber } from "@/lib/phone";
import type { Contact, EntityResponsavel } from "@/lib/types";

const initials = (name: string) => name.split(" ").filter(Boolean).map((p) => p[0]).slice(0, 2).join("").toUpperCase();

/**
 * An asset's responsáveis in its detail profile: main one first (marked only
 * when there's more than one to tell apart), role · area, the phone readable
 * with a WhatsApp shortcut, and each name opens the contact — how to reach
 * them and what else they're responsável for. "+" / the empty state lead to
 * the edit form.
 */
export default function ResponsaveisSection({ responsaveis, t, emptyTitle, onEdit }: {
  responsaveis: EntityResponsavel[];
  t: (key: string) => string;
  emptyTitle?: string;
  /** Opens the asset's edit form; absent for users who can't edit. */
  onEdit?: () => void;
  /** Kept for callers; the section is always the compact list. */
  compact?: boolean;
}) {
  const [open, setOpen] = useState<Contact | null>(null);
  const items = [...(responsaveis ?? [])].sort((a, b) => Number(b.is_main) - Number(a.is_main));
  const markMain = items.length > 1;

  return (
    <>
      <SectionCard
        title={t("host.responsaveis")}
        count={items.length}
        body="flush"
        empty={items.length === 0 ? emptyTitle || t("host.noResponsaveis") : undefined}
        emptyAction={onEdit ? <Button size="sm" variant="secondary" onClick={onEdit}>{t("responsavel.addFirst")}</Button> : undefined}
        controls={onEdit && items.length > 0 ? (
          <IconButton label={t("responsavel.manage")} onClick={onEdit}><Icon path={ICON_PATHS.editPencil} /></IconButton>
        ) : undefined}
      >
        <ul className="divide-y divide-[var(--border-subtle)]">
          {items.map((r, i) => (
            <li key={r.id ?? `${r.contact_id}-${i}`} className="flex items-center gap-3 px-5 py-3">
              <span aria-hidden className={`w-9 h-9 rounded-full flex items-center justify-center shrink-0 text-xs font-bold ${r.is_main && markMain ? "bg-[var(--accent)]/15 text-[var(--accent)]" : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"}`}>
                {initials(r.name)}
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-1.5 min-w-0">
                  <button type="button" className="text-sm font-medium text-[var(--text-primary)] truncate hover:underline text-left"
                    onClick={() => setOpen({ id: r.contact_id, name: r.name, phone: r.phone, role: r.role, entity: r.entity, notes: r.notes ?? "", is_external: r.is_external })}>
                    {r.name}
                  </button>
                  {r.is_main && markMain && <Badge color="purple">{t("responsavel.main")}</Badge>}
                  {r.is_external && <Badge color="warning">{t("responsavel.external")}</Badge>}
                </span>
                <span className="block text-xs text-[var(--text-muted)] truncate">{[r.role, r.entity].filter(Boolean).join(" · ") || "–"}</span>
                {r.phone && <a href={`tel:+${whatsappNumber(r.phone)}`} className="block text-xs font-mono text-[var(--text-secondary)] hover:underline w-fit">{formatPhone(r.phone)}</a>}
              </span>
              {r.phone && (
                <a href={`https://wa.me/${whatsappNumber(r.phone)}`} target="_blank" rel="noopener noreferrer"
                  aria-label={`WhatsApp ${r.name}`} title={`WhatsApp ${r.name}`}
                  className="w-8 h-8 rounded-full flex items-center justify-center shrink-0 bg-[var(--success)]/15 text-[var(--success)] hover:bg-[var(--success)]/25 transition-colors">
                  <Icon path={ICON_PATHS.whatsapp} fill="currentColor" stroke="none" className="w-4 h-4" />
                </a>
              )}
            </li>
          ))}
        </ul>
      </SectionCard>
      <ContactDetailDrawer contact={open} onClose={() => setOpen(null)} />
    </>
  );
}
