"use client";

import Link from "next/link";
import SectionCard from "@/components/ui/SectionCard";
import SectionHeading from "@/components/ui/SectionHeading";
import Field from "@/components/ui/Field";
import Tag from "@/components/ui/Tag";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import { useLocale } from "@/contexts/LocaleContext";
import { originKey } from "@/lib/serviceDisplay";
import type { EntityResponsavel, Service } from "@/lib/types";

/**
 * The left column of the service "Visão geral": what was declared — category,
 * origin, stack, where it's deployed, ownership and links — then tags and
 * responsáveis. Every field renders; an empty one is a muted "–".
 */
export default function ServiceProfile({ service, tags, responsaveis, projectName }: {
  service: Service;
  tags: string[];
  responsaveis: EntityResponsavel[];
  projectName?: string;
}) {
  const { t } = useLocale();
  const block = "pt-4 mt-4 border-t border-[var(--border-subtle)]";
  return (
    <>
      <SectionCard title={t("service.profileTitle")}>
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("service.category")} value={service.service_kind ? t(`service.kind.${service.service_kind}`) : ""} />
          <Field label={t("service.originTitle")} value={t(originKey(service))} />
          <Field label={t("service.technologyStack")} value={service.technology_stack} />
          <Field label={t("service.environment")} value={service.environment} />
          <Field label={t("service.port")} value={service.port} mono />
          <Field label={t("service.version")} value={service.version} mono />
          <Field label={t("service.deployApproach")} value={[service.deploy_approach, service.orchestrator_tool].filter(Boolean).join(" / ")} />
          <Field label={t("service.developedBy")} value={service.developed_by === "internal" ? t("service.internal") : service.developed_by === "external" ? t("service.external") : ""} />
          <div className="col-span-2">
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">{t("service.projectLabel")}</span>
            {service.project_id && projectName
              ? <Link href={`/projects/${service.project_id}`} className="text-sm text-[var(--accent)] hover:underline">{projectName}</Link>
              : <p className="text-sm text-[var(--text-muted)]">–</p>}
          </div>
        </div>

        {service.is_external_dependency && (
          <div className={block}>
            <SectionHeading as="h3" className="!mb-2">{t("service.isExternalDependency")}</SectionHeading>
            <div className="grid grid-cols-2 gap-4">
              <Field label={t("service.externalProvider")} value={service.external_provider} />
              <Field label={t("service.externalContact")} value={service.external_contact} />
              <Field className="col-span-2" label={t("service.externalUrl")} value={service.external_url} link />
            </div>
          </div>
        )}

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("service.links")}</SectionHeading>
          <div className="grid grid-cols-1 gap-4">
            <Field label={t("project.gitlabUrl")} value={service.repository_url || service.gitlab_url} link />
            <Field label={t("project.documentationUrl")} value={service.documentation_url} link />
          </div>
        </div>

        <div className={block}>
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
