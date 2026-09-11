"use client";

import Link from "next/link";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import type { Service, Host, DNSRecord } from "@/lib/types";
import SectionHeading from "@/components/ui/SectionHeading";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

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
        <SectionHeading variant="section">
          {t("service.dependencies")}
        </SectionHeading>
        {dependsOnServices.length > 0 ? (
          <div className="space-y-1">
            {dependsOnServices.map((dep) => (
              <Link
                key={dep.id}
                href={`/services/${dep.id}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <Icon path={ICON_PATHS.arrowRight} className="w-4 h-4 text-[var(--text-faint)] shrink-0" strokeWidth={1.5} />
                <span className="flex-1 truncate">{dep.nickname}</span>
                {dep.is_external_dependency && (
                  <Badge color="amber" className="text-2xs">{t("service.isExternalDependency")}</Badge>
                )}
                {dep.technology_stack && <Badge>{dep.technology_stack}</Badge>}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">{t("service.noDependencies")}</p>
        )}
      </Card>

      {/* Dependents */}
      <Card hover={false}>
        <SectionHeading variant="section">
          {t("service.dependents")}
        </SectionHeading>
        {dependentServices.length > 0 ? (
          <div className="space-y-1">
            {dependentServices.map((dep) => (
              <Link
                key={dep.id}
                href={`/services/${dep.id}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <Icon path={ICON_PATHS.arrowLeft} className="w-4 h-4 text-[var(--text-faint)] shrink-0" strokeWidth={1.5} />
                <span className="flex-1 truncate">{dep.nickname}</span>
                {dep.technology_stack && <Badge>{dep.technology_stack}</Badge>}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">{t("service.noDependents")}</p>
        )}
      </Card>

      {/* Linked Hosts */}
      <Card hover={false}>
        <SectionHeading variant="section">
          {t("service.linkedHosts")}
        </SectionHeading>
        {linkedHosts.length > 0 ? (
          <div className="space-y-1">
            {linkedHosts.map((host) => (
              <Link
                key={host.id}
                href={`/hosts/${host.oficial_slug}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <Icon path={ICON_PATHS.servers} className="w-4 h-4 text-[var(--text-faint)] shrink-0" strokeWidth={1.5} />
                <span className="flex-1 truncate font-mono">{host.oficial_slug}</span>
                {host.nickname && host.nickname !== host.oficial_slug && (
                  <span className="text-[var(--text-faint)] text-xs truncate">{host.nickname}</span>
                )}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">{t("service.noLinkedHosts")}</p>
        )}
      </Card>

      {/* Linked DNS */}
      <Card hover={false}>
        <SectionHeading variant="section">
          {t("service.linkedDns")}
        </SectionHeading>
        {linkedDns.length > 0 ? (
          <div className="space-y-1">
            {linkedDns.map((dns) => (
              <Link
                key={dns.id}
                href={`/dns/${dns.id}`}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]"
              >
                <Icon path={ICON_PATHS.globeAlt} className="w-4 h-4 text-[var(--text-faint)] shrink-0" strokeWidth={1.5} />
                <span className="flex-1 truncate font-mono">{dns.domain}</span>
                {dns.has_https && (
                  <Badge color="emerald">
                    <Icon path={ICON_PATHS.lock} className="w-3 h-3" />
                  </Badge>
                )}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">{t("service.noLinkedDns")}</p>
        )}
      </Card>
    </div>
  );
}
