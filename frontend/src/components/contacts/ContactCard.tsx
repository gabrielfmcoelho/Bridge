"use client";

import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import RowActions, { type RowAction } from "@/components/ui/RowActions";
import { CardHeader, CardMetadataGrid, CardIndicator, CardIndicatorGrid } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import { formatPhone } from "@/lib/phone";
import { usageSummary } from "./ContactDetailDrawer";
import type { Contact } from "@/lib/types";

/**
 * A contact on the inventory card anatomy (header · metadata grid ·
 * indicators): who, how to reach, and — the part only a contact has — what
 * they're responsável for, one indicator per asset type. External contacts
 * carry the warning accent.
 */
export default function ContactCard({ contact: c, onOpen, actions }: { contact: Contact; onOpen: () => void; actions: RowAction[] }) {
  const { t } = useLocale();
  const n = (k: "host" | "dns" | "service" | "project") => c.usage?.[k] ?? 0;
  return (
    <Card accent={c.is_external ? "warning" : "accent"} hover onClick={onOpen} className="h-full flex flex-col overflow-hidden cursor-pointer">
      <CardHeader
        titleFont="display"
        title={c.name}
        subtitle={c.role || undefined}
        subtitleFont="display"
        status={c.is_external ? <Badge color="warning">{t("responsavel.external")}</Badge> : <span className="text-xs text-[var(--text-secondary)]">{t("contact.internal")}</span>}
        description={c.notes}
        corner={<span onClick={(e) => e.stopPropagation()}><RowActions name={c.name} actions={actions} /></span>}
      />
      <CardMetadataGrid
        items={[
          { label: t("responsavel.phone"), value: c.phone ? formatPhone(c.phone) : "", mono: true },
          { label: t("contact.email"), value: c.email ?? "" },
          { label: t("contact.area"), value: c.entity },
          { label: t("contact.responsibleFor"), value: usageSummary(c.usage, t) },
        ]}
      />
      <CardIndicatorGrid>
        <CardIndicator icon={ICON_PATHS.server} count={n("host")} color="info" title={t("contact.use.host", { count: String(n("host")) })} />
        <CardIndicator icon={ICON_PATHS.gear} count={n("service")} color="warning" title={t("contact.use.service", { count: String(n("service")) })} />
        <CardIndicator icon={ICON_PATHS.globe} count={n("dns")} color="success" title={t("contact.use.dns", { count: String(n("dns")) })} />
        <CardIndicator icon={ICON_PATHS.folder} count={n("project")} color="purple" title={t("contact.use.project", { count: String(n("project")) })} />
      </CardIndicatorGrid>
    </Card>
  );
}
