"use client";

import Link from "next/link";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Field from "@/components/ui/Field";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import type { DNSRecord, Host, EntityResponsavel } from "@/lib/types";

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
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
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
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                  </svg>
                  {h.nickname || h.oficial_slug}
                </Link>
              ))}
            </div>
          </Card>
        </>
      )}

      {/* Timestamps */}
      <div className="flex flex-wrap gap-4 text-[11px] text-[var(--text-faint)]">
        {dns.created_at && <span>Created: {new Date(dns.created_at).toLocaleString()}</span>}
        {dns.updated_at && <span>Updated: {new Date(dns.updated_at).toLocaleString()}</span>}
      </div>
    </div>
  );
}
