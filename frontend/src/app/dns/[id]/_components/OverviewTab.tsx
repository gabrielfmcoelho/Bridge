"use client";

import Link from "next/link";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Field from "@/components/ui/Field";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import type { DNSRecord, Host, EntityResponsavel } from "@/lib/types";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface OverviewTabProps {
  dns: DNSRecord;
  tags: string[];
  responsaveis: EntityResponsavel[];
  linkedHosts: Host[];
  t: (key: string) => string;
}

export default function OverviewTab({ dns, tags, responsaveis, linkedHosts, t }: OverviewTabProps) {
  return (
    <div className="space-y-5 animate-fade-in">
      <SectionHeading>DNS Info</SectionHeading>
      <Card hover={false}>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4 text-sm">
          <Field label="Domain" value={dns.domain} mono />
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">{t("host.situacao")}</span>
            <Badge variant="situacao" situacao={dns.situacao} dot>{dns.situacao}</Badge>
          </div>
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">HTTPS</span>
            {dns.has_https ? (
              <Badge color="emerald">
                <Icon path={ICON_PATHS.lock} className="w-3 h-3 mr-1" />
                HTTPS enabled
              </Badge>
            ) : (
              <span className="text-[var(--text-faint)] text-sm">-</span>
            )}
          </div>
        </div>
        {tags.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mt-4 pt-4 border-t border-[var(--border-subtle)]">
            {tags.map((tag) => <Badge key={tag}>{tag}</Badge>)}
          </div>
        )}
      </Card>

      {/* Observations */}
      {dns.observacoes && (
        <>
          <SectionHeading>{t("common.observacoes")}</SectionHeading>
          <Card hover={false}>
            <p className="text-sm text-[var(--text-secondary)] whitespace-pre-wrap">{dns.observacoes}</p>
          </Card>
        </>
      )}

      {/* Responsaveis — shared component with cards/table view toggle */}
      <ResponsaveisSection responsaveis={responsaveis} t={t} />

      {/* Linked Hosts */}
      {linkedHosts.length > 0 && (
        <>
          <SectionHeading>Linked Hosts</SectionHeading>
          <Card hover={false}>
            <div className="flex flex-wrap gap-2">
              {linkedHosts.map((h) => (
                <Link
                  key={h.id}
                  href={`/hosts/${h.oficial_slug}`}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-[var(--text-secondary)] hover:text-[var(--accent)] hover:border-[var(--accent)]/30 transition-colors"
                >
                  <Icon path={ICON_PATHS.server} className="w-3.5 h-3.5" />
                  {h.nickname || h.oficial_slug}
                </Link>
              ))}
            </div>
          </Card>
        </>
      )}

      {/* Timestamps */}
      <div className="flex flex-wrap gap-4 text-xs text-[var(--text-faint)]">
        {dns.created_at && <span>Created: {new Date(dns.created_at).toLocaleString()}</span>}
        {dns.updated_at && <span>Updated: {new Date(dns.updated_at).toLocaleString()}</span>}
      </div>
    </div>
  );
}
