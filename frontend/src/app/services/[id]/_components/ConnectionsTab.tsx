"use client";

import Link from "next/link";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import type { Service, Host, DNSRecord } from "@/lib/types";

interface ConnectionsTabProps {
  dependsOnServices: Service[];
  dependentServices: Service[];
  linkedHosts: Host[];
  linkedDns: DNSRecord[];
  t: (key: string) => string;
}

export default function ConnectionsTab({ dependsOnServices, dependentServices, linkedHosts, linkedDns, t }: ConnectionsTabProps) {
  return (
    <div className="space-y-5 animate-fade-in">
      {/* Dependencies */}
      <Card hover={false}>
        <h2
          className="text-sm font-semibold text-[var(--text-secondary)] mb-3"
          style={{ fontFamily: "var(--font-display)" }}
        >
          {t("service.dependencies")}
        </h2>
        {dependsOnServices.length > 0 ? (
          <div className="space-y-1">
            {dependsOnServices.map((dep) => (
              <Link
                key={dep.id}
                href={`/services/${dep.id}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <svg className="w-4 h-4 text-[var(--text-faint)] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
                </svg>
                <span className="flex-1 truncate">{dep.nickname}</span>
                {dep.is_external_dependency && (
                  <Badge color="amber" className="text-[10px]">{t("service.isExternalDependency")}</Badge>
                )}
                {dep.technology_stack && <Badge>{dep.technology_stack}</Badge>}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">No dependencies</p>
        )}
      </Card>

      {/* Dependents */}
      <Card hover={false}>
        <h2
          className="text-sm font-semibold text-[var(--text-secondary)] mb-3"
          style={{ fontFamily: "var(--font-display)" }}
        >
          {t("service.dependents")}
        </h2>
        {dependentServices.length > 0 ? (
          <div className="space-y-1">
            {dependentServices.map((dep) => (
              <Link
                key={dep.id}
                href={`/services/${dep.id}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <svg className="w-4 h-4 text-[var(--text-faint)] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
                </svg>
                <span className="flex-1 truncate">{dep.nickname}</span>
                {dep.technology_stack && <Badge>{dep.technology_stack}</Badge>}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">No dependents</p>
        )}
      </Card>

      {/* Linked Hosts */}
      <Card hover={false}>
        <h2
          className="text-sm font-semibold text-[var(--text-secondary)] mb-3"
          style={{ fontFamily: "var(--font-display)" }}
        >
          Linked Hosts
        </h2>
        {linkedHosts.length > 0 ? (
          <div className="space-y-1">
            {linkedHosts.map((host) => (
              <Link
                key={host.id}
                href={`/hosts/${host.oficial_slug}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <svg className="w-4 h-4 text-[var(--text-faint)] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
                </svg>
                <span className="flex-1 truncate" style={{ fontFamily: "var(--font-mono)" }}>{host.oficial_slug}</span>
                {host.nickname && host.nickname !== host.oficial_slug && (
                  <span className="text-[var(--text-faint)] text-xs truncate">{host.nickname}</span>
                )}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">No linked hosts</p>
        )}
      </Card>

      {/* Linked DNS */}
      <Card hover={false}>
        <h2
          className="text-sm font-semibold text-[var(--text-secondary)] mb-3"
          style={{ fontFamily: "var(--font-display)" }}
        >
          Linked DNS
        </h2>
        {linkedDns.length > 0 ? (
          <div className="space-y-1">
            {linkedDns.map((dns) => (
              <Link
                key={dns.id}
                href={`/dns/${dns.id}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <svg className="w-4 h-4 text-[var(--text-faint)] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
                </svg>
                <span className="flex-1 truncate" style={{ fontFamily: "var(--font-mono)" }}>{dns.domain}</span>
                {dns.has_https && (
                  <Badge color="emerald">
                    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                    </svg>
                  </Badge>
                )}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">No linked DNS records</p>
        )}
      </Card>
    </div>
  );
}
