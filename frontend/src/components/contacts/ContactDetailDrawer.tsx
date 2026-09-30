"use client";

import { useQuery } from "@tanstack/react-query";
import { contactsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import Field from "@/components/ui/Field";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import SectionHeading from "@/components/ui/SectionHeading";
import { RowGroup, ListRow, RowText } from "@/components/ui/RowList";
import { Skeleton } from "@/components/ui/Skeleton";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";
import { formatPhone, whatsappNumber } from "@/lib/phone";
import type { Contact } from "@/lib/types";

type UseType = "host" | "dns" | "service" | "project" | "api_catalog";
export const USE_TYPES: UseType[] = ["host", "service", "dns", "project", "api_catalog"];
const USE_ICON: Record<UseType, string> = { host: NAV_ICONS.Server, dns: NAV_ICONS.Globe, service: NAV_ICONS.Boxes, project: NAV_ICONS.FolderKanban, api_catalog: NAV_ICONS.Plug };

/** "3 hosts · 1 serviço" — what a contact is responsável for, in words. */
export function usageSummary(usage: Contact["usage"], t: (k: string, v?: Record<string, string>) => string): string {
  return USE_TYPES.filter((k) => usage?.[k]).map((k) => t(`contact.use.${k}`, { count: String(usage![k]) })).join(" · ");
}

const HREF: Record<UseType, (u: { id: number; slug?: string }) => string> = {
  host: (u) => `/hosts/${u.slug}`,
  dns: (u) => `/dns/${u.id}`,
  service: (u) => `/services/${u.id}`,
  project: (u) => `/projects/${u.id}`,
  api_catalog: (u) => `/atlas/apis/${u.id}`,
};
const hrefOf = (u: { type: UseType; id: number; slug?: string }) => HREF[u.type](u);

/**
 * A contact at a glance: how to reach them and everything they're
 * responsável for, each asset one click away.
 */
export default function ContactDetailDrawer({ contact, onClose, onEdit, onDelete }: {
  contact: Contact | null;
  onClose: () => void;
  onEdit?: (c: Contact) => void;
  onDelete?: (c: Contact) => void;
}) {
  const { t } = useLocale();
  const { data: uses = [], isLoading, isError } = useQuery({
    queryKey: ["contact-usage", contact?.id],
    queryFn: () => contactsAPI.usage(contact!.id),
    enabled: !!contact,
    retry: false,
  });

  const footer = contact && (onEdit || onDelete) ? (
    <div className="flex items-center gap-2">
      {onDelete && <Button variant="ghost" size="sm" onClick={() => onDelete(contact)}>{t("common.delete")}</Button>}
      {onEdit && <Button className="ml-auto" onClick={() => onEdit(contact)}>{t("common.edit")}</Button>}
    </div>
  ) : undefined;

  return (
    <Drawer open={!!contact} onClose={onClose} title={contact?.name ?? ""} footer={footer}>
      {contact && (
        <div className="space-y-6">
          <div className="flex flex-wrap items-center gap-2">
            {contact.role && <span className="text-sm text-[var(--text-secondary)]">{contact.role}</span>}
            {contact.is_external && <Badge color="warning">{t("responsavel.external")}</Badge>}
          </div>
          <div className="grid grid-cols-2 gap-4">
            <Field label={t("responsavel.phone")} value={contact.phone ? formatPhone(contact.phone) : ""} mono />
            <Field label={t("contact.email")} value={contact.email ?? ""} />
            <Field label={t("contact.area")} value={contact.entity} />
            {contact.notes && <Field className="col-span-2" label={t("responsavel.notes")} value={contact.notes} />}
          </div>
          {(contact.phone || contact.email) && (
            <div className="flex flex-wrap gap-2">
              {contact.phone && (
                <a className="inline-flex items-center gap-1.5 h-8 px-3 text-sm rounded-[var(--radius-md)] border border-[var(--success)]/30 bg-[var(--success)]/10 text-[var(--success)] hover:bg-[var(--success)]/20"
                  href={`https://wa.me/${whatsappNumber(contact.phone)}`} target="_blank" rel="noopener noreferrer">
                  <Icon path={ICON_PATHS.whatsapp} fill="currentColor" stroke="none" className="w-4 h-4" /> WhatsApp
                </a>
              )}
              {contact.email && (
                <a className="inline-flex items-center gap-1.5 h-8 px-3 text-sm rounded-[var(--radius-md)] border border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
                  href={`mailto:${contact.email}`}>
                  <Icon path={ICON_PATHS.send} className="w-4 h-4" /> {t("contact.sendEmail")}
                </a>
              )}
            </div>
          )}

          <section className="space-y-2">
            <SectionHeading as="h3" className="!mb-0" count={uses.length}>{t("contact.responsibleFor")}</SectionHeading>
            {isLoading && !isError ? <Skeleton className="h-24 w-full" /> : uses.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">{t("contact.noUse")}</p>
            ) : (
              <div className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
                {USE_TYPES.map((type) => {
                  const rows = uses.filter((u) => u.type === type);
                  return rows.length > 0 && (
                    <RowGroup key={type} title={t(`contact.useGroup.${type}`)}>
                      {rows.map((u) => (
                        <ListRow key={`${type}-${u.id}`} href={hrefOf(u)}>
                          <Icon path={USE_ICON[type]} className="w-3.5 h-3.5 shrink-0 text-[var(--text-muted)]" />
                          <RowText title={u.name} />
                          {u.is_main && <span className="text-xs text-[var(--text-muted)]">{t("responsavel.main")}</span>}
                        </ListRow>
                      ))}
                    </RowGroup>
                  );
                })}
              </div>
            )}
          </section>
        </div>
      )}
    </Drawer>
  );
}
