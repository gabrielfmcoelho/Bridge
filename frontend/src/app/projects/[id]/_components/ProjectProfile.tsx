"use client";

import SectionCard from "@/components/ui/SectionCard";
import SectionHeading from "@/components/ui/SectionHeading";
import Field from "@/components/ui/Field";
import Tag from "@/components/ui/Tag";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import { useLocale } from "@/contexts/LocaleContext";
import type { Project, ProjectResponsavel } from "@/lib/types";

/**
 * The left column of the project "Visão geral": what was declared — setor,
 * how it's run, external company, links, tags — then its responsáveis.
 * Name, situação and description are the page header's; not repeated.
 */
export default function ProjectProfile({ project, tags, responsaveis }: { project: Project; tags: string[]; responsaveis: ProjectResponsavel[] }) {
  const { t } = useLocale();
  const block = "pt-4 mt-4 border-t border-[var(--border-subtle)]";
  return (
    <>
      <SectionCard title={t("project.profileTitle")}>
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("project.setorResponsavel")} value={project.setor_responsavel} />
          <Field label={t("dns.responsavel")} value={project.responsavel} />
          <Field label={t("project.managed")} value={project.is_directly_managed ? t("common.yes") : t("common.no")} />
          <Field label={t("project.isResponsible")} value={project.is_responsible ? t("common.yes") : t("common.no")} />
          <Field className="col-span-2" label={t("project.externalCompany")}
            value={project.tem_empresa_externa_responsavel ? project.contato_empresa_responsavel || t("common.yes") : ""} />
        </div>
        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("service.links")}</SectionHeading>
          <div className="grid grid-cols-1 gap-4">
            <Field label={t("project.documentationUrl")} value={project.documentation_url} link />
            <Field label={t("project.gitlabUrl")} value={project.gitlab_url} link />
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
