"use client";

import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Field from "@/components/ui/Field";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import type { Service, EntityResponsavel } from "@/lib/types";

interface OverviewTabProps {
  service: Service;
  tags: string[];
  responsaveis: EntityResponsavel[];
  t: (key: string) => string;
}

export default function OverviewTab({ service, tags, responsaveis, t }: OverviewTabProps) {
  return (
    <div className="space-y-5 animate-fade-in">
      {/* Tags */}
      {tags && tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {tags.map((tag) => (
            <Badge key={tag}>{tag}</Badge>
          ))}
        </div>
      )}

      {/* Main info grid */}
      <Card accent="purple" hover={false}>
        <h2
          className="text-sm font-semibold text-[var(--text-secondary)] mb-4"
          style={{ fontFamily: "var(--font-display)" }}
        >
          Service Info
        </h2>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
          <Field label={t("service.serviceType")} value={[service.service_type, service.service_subtype].filter(Boolean).join(" / ")} />
          <Field label={t("service.technologyStack")} value={service.technology_stack} />
          <Field label={t("service.deployApproach")} value={[service.deploy_approach, service.orchestrator_tool].filter(Boolean).join(" / ")} />
          <Field label={t("service.environment")} value={service.environment} />
          <Field label={t("service.port")} value={service.port} mono />
          <Field label={t("service.version")} value={service.version} />
          <div>
            <span className="text-[var(--text-faint)] text-[11px]">{t("service.developedBy")}</span>
            {service.developed_by ? (
              <Badge className="mt-1">{service.developed_by}</Badge>
            ) : (
              <p className="text-sm text-[var(--text-secondary)]">-</p>
            )}
          </div>
          <Field
            label={t("project.gitlabUrl")}
            value={service.repository_url || service.gitlab_url}
            link
          />
          <Field
            label={t("project.documentationUrl")}
            value={service.documentation_url}
            link
          />
        </div>
      </Card>

      {/* External dependency section */}
      {service.is_external_dependency && (
        <Card accent="amber" hover={false}>
          <h2
            className="text-sm font-semibold text-[var(--text-secondary)] mb-4"
            style={{ fontFamily: "var(--font-display)" }}
          >
            External Dependency
          </h2>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <Field label={t("service.externalProvider")} value={service.external_provider} />
            <Field label={t("service.externalUrl")} value={service.external_url} link />
            <Field label={t("service.externalContact")} value={service.external_contact} />
          </div>
        </Card>
      )}

      {/* Responsaveis */}
      <ResponsaveisSection responsaveis={responsaveis} t={t} />
    </div>
  );
}
