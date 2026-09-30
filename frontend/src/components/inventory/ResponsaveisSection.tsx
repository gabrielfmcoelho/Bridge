"use client";

import { useState } from "react";
import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import ContactDetailDrawer from "@/components/contacts/ContactDetailDrawer";
import { ICON_PATHS } from "@/lib/icon-paths";
import { whatsappNumber } from "@/lib/phone";
import type { Contact, EntityResponsavel } from "@/lib/types";

const initials = (name: string) => name.split(" ").filter(Boolean).map((p) => p[0]).slice(0, 2).join("").toUpperCase();

/**
 * An asset's responsáveis in its detail profile: main one first, a WhatsApp
 * shortcut per phone, and each name opens the contact — how to reach them and
 * what else they're responsável for.
 */
export default function ResponsaveisSection({ responsaveis, t, emptyTitle }: {
  responsaveis: EntityResponsavel[];
  t: (key: string) => string;
  emptyTitle?: string;
  /** Kept for callers; the section is always the compact list now. */
  compact?: boolean;
}) {
  const [open, setOpen] = useState<Contact | null>(null);
  const items = [...(responsaveis ?? [])].sort((a, b) => Number(b.is_main) - Number(a.is_main));

  return (
    <>
      <SectionCard title={t("host.responsaveis")} count={items.length} body="flush"
        empty={items.length === 0 ? emptyTitle || t("host.noResponsaveis") : undefined}>
        <ul className="divide-y divide-[var(--border-subtle)]">
          {items.map((r, i) => (
            <li key={r.id ?? `${r.contact_id}-${i}`} className="flex items-center gap-3 px-5 py-3">
              <span className={`w-8 h-8 rounded-full flex items-center justify-center shrink-0 text-xs font-bold ${r.is_main ? "bg-[var(--accent)]/15 text-[var(--accent)]" : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"}`}>
                {initials(r.name)}
              </span>
              <button type="button" className="min-w-0 flex-1 text-left group"
                onClick={() => setOpen({ id: r.contact_id, name: r.name, phone: r.phone, role: r.role, entity: r.entity, notes: r.notes ?? "", is_external: r.is_external })}>
                <span className="flex items-center gap-1.5 text-sm font-medium text-[var(--text-primary)] truncate group-hover:underline">
                  {r.name}
                  {r.is_main && <Icon path={ICON_PATHS.star} className="w-3.5 h-3.5 text-[var(--warning)]" aria-label={t("responsavel.main")} />}
                  {r.is_external && <Badge color="warning">{t("responsavel.external")}</Badge>}
                </span>
                <span className="block text-xs text-[var(--text-muted)] truncate">{[r.role, r.entity].filter(Boolean).join(" · ") || "–"}</span>
              </button>
              {r.phone && (
                <a href={`https://wa.me/${whatsappNumber(r.phone)}`} target="_blank" rel="noopener noreferrer"
                  aria-label={`WhatsApp ${r.name}`} title={`WhatsApp ${r.name}`}
                  className="w-8 h-8 rounded-full flex items-center justify-center bg-[var(--success)]/15 text-[var(--success)] hover:bg-[var(--success)]/25 transition-colors">
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
