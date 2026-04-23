"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { hostsAPI, dnsAPI } from "@/lib/api";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import ProjectAiAnalysis from "./ProjectAiAnalysis";
import type { Project, ProjectResponsavel, Service } from "@/lib/types";

interface OverviewTabProps {
  project: Project;
  responsaveis: ProjectResponsavel[];
  services: Service[];
  hostIds: number[];
  dnsIds: number[];
  t: (key: string) => string;
}

export default function OverviewTab({ project, responsaveis, services, hostIds, dnsIds, t }: OverviewTabProps) {
  const { data: allHosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: allDns = [] } = useQuery({ queryKey: ["dns"], queryFn: dnsAPI.list });

  const linkedHosts = allHosts.filter((h) => hostIds.includes(h.id));
  const linkedDns = allDns.filter((d) => dnsIds.includes(d.id));

  return (
    <div className="space-y-5">
      {/* Project info */}
      <Card accent="amber" hover={false} className="animate-slide-up stagger-1" style={{ animationFillMode: "both" } as React.CSSProperties}>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium">{t("project.setorResponsavel")}</span>
            <p className="text-[var(--text-primary)]">{project.setor_responsavel || "-"}</p>
          </div>
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium">{t("project.responsaveis")}</span>
            <p className="text-[var(--text-primary)]">{project.responsavel || "-"}</p>
          </div>
        </div>
        {/* Links as icons */}
        <div className="flex gap-3 mt-4 pt-3 border-t border-[var(--border-subtle)]">
          {project.gitlab_url && (
            <a href={project.gitlab_url} target="_blank" rel="noopener noreferrer"
              className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors"
              title={t("project.gitlabUrl")}>
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
              </svg>
              GitLab
            </a>
          )}
          {project.documentation_url && (
            <a href={project.documentation_url} target="_blank" rel="noopener noreferrer"
              className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors"
              title={t("project.documentationUrl")}>
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
              </svg>
              Docs
            </a>
          )}
          {!project.gitlab_url && !project.documentation_url && (
            <span className="text-xs text-[var(--text-faint)]">No links</span>
          )}
        </div>
      </Card>

      {/* AI analysis of recent work (based on commits from linked GitLab repos) */}
      <ProjectAiAnalysis projectId={project.id} />

      {/* Responsaveis — shared cards/table view */}
      <ResponsaveisSection responsaveis={responsaveis} t={t} />

      {/* Services */}
      {services && services.length > 0 && (
        <Card hover={false} className="animate-slide-up stagger-3" style={{ animationFillMode: "both" } as React.CSSProperties}>
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-3" style={{ fontFamily: "var(--font-display)" }}>
            {t("service.title")}
          </h2>
          <div className="space-y-1">
            {services.map((svc) => (
              <Link key={svc.id} href={`/services/${svc.id}`} className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]">
                <span>{svc.nickname}</span>
                {svc.technology_stack && <Badge>{svc.technology_stack}</Badge>}
                {svc.is_external_dependency && (
                  <Badge color="amber" className="text-[10px]">
                    {svc.external_provider || "External"}
                  </Badge>
                )}
              </Link>
            ))}
          </div>
        </Card>
      )}

      {/* Linked DNS records */}
      {linkedDns.length > 0 && (
        <Card hover={false} className="animate-slide-up stagger-4" style={{ animationFillMode: "both" } as React.CSSProperties}>
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-3" style={{ fontFamily: "var(--font-display)" }}>
            DNS Records
          </h2>
          <div className="space-y-1">
            {linkedDns.map((dns) => (
              <div key={dns.id} className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors">
                <span className="text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{dns.domain}</span>
                {dns.has_https && (
                  <Badge color="emerald">
                    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                    </svg>
                  </Badge>
                )}
                <Badge variant="situacao" situacao={dns.situacao} dot>{dns.situacao}</Badge>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Linked Hosts */}
      {linkedHosts.length > 0 && (
        <Card hover={false} className="animate-slide-up stagger-5" style={{ animationFillMode: "both" } as React.CSSProperties}>
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-3" style={{ fontFamily: "var(--font-display)" }}>
            Hosts
          </h2>
          <div className="space-y-1">
            {linkedHosts.map((host) => (
              <Link key={host.id} href={`/hosts/${host.oficial_slug}`} className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors text-[var(--text-primary)] hover:text-[var(--accent)]">
                <span style={{ fontFamily: "var(--font-mono)" }}>{host.nickname}</span>
                <span className="text-[var(--text-faint)] text-xs">{host.hostname}</span>
                <Badge variant="situacao" situacao={host.situacao} dot>{host.situacao}</Badge>
              </Link>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}
