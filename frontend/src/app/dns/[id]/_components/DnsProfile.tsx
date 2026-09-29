"use client";

import SectionCard from "@/components/ui/SectionCard";
import SectionHeading from "@/components/ui/SectionHeading";
import Field from "@/components/ui/Field";
import Tag from "@/components/ui/Tag";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import { useLocale } from "@/contexts/LocaleContext";
import type { DNSRecord, EntityResponsavel } from "@/lib/types";

/**
 * The left column of the DNS "Visão geral": what was declared about the
 * domain. Domain, situação and observações are the page header's title,
 * status and description, so they aren't repeated here. Every field renders;
 * an empty one is a muted "–".
 */
export default function DnsProfile({ dns, tags, responsaveis }: { dns: DNSRecord; tags: string[]; responsaveis: EntityResponsavel[] }) {
  const { t, formatDateTime } = useLocale();
  return (
    <>
      <SectionCard title={t("dns.profileTitle")}>
        <div className="grid grid-cols-2 gap-4">
          <Field label="HTTPS" value={dns.has_https ? t("common.yes") : t("common.no")} />
          <Field label={t("host.entity")} value={dns.main_entidade || ""} />
          <Field label={t("dns.createdAt")} value={dns.created_at ? formatDateTime(dns.created_at) : ""} />
          <Field label={t("dns.updatedAt")} value={dns.updated_at ? formatDateTime(dns.updated_at) : ""} />
        </div>
        <div className="pt-4 mt-4 border-t border-[var(--border-subtle)]">
          <SectionHeading as="h3" className="!mb-2">{t("common.tags")}</SectionHeading>
          <div className="flex flex-wrap gap-1.5">
            {tags.length > 0 ? tags.map((tag) => <Tag key={tag}>{tag}</Tag>) : <span className="text-sm text-[var(--text-muted)]">–</span>}
          </div>
        </div>
      </SectionCard>
      <ResponsaveisSection responsaveis={responsaveis} t={t} compact />
    </>
  );
}
