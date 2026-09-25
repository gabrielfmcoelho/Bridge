"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { situacaoAccent } from "@/lib/constants";
import SituacaoText from "@/components/ui/SituacaoText";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Project } from "@/lib/types";

export default function ProjectCard({ project }: { project: Project }) {
  const { t } = useLocale();
  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const situacaoColor = situacoes.find((s) => s.value === project.situacao)?.color;

  return (
    <Link href={`/projects/${project.id}`} className="block h-full">
      <Card accent={situacaoAccent(project.situacao, situacaoColor)} className="h-full flex flex-col overflow-hidden">
        {/* Fixed anatomy: every slot renders, "–" when empty. */}
        <CardHeader
          titleFont="display"
          title={project.name}
          subtitle={project.setor_responsavel || undefined}
          subtitleFont="display"
          status={<SituacaoText situacao={project.situacao} />}
          description={project.description}
        />

        <CardMetadataGrid
          items={[
            { label: t("project.responsavel"), value: project.responsavel || "" },
            { label: t("project.externalCompany"), value: project.contato_empresa_responsavel || "" },
            { label: t("project.managed"), value: project.is_directly_managed ? t("common.yes") : t("common.no") },
            { label: t("host.entity"), value: project.main_entidade || "" },
          ]}
        />

        <CardTagsSection tags={project.tags} />

        {/* Bottom indicators */}
        <div className="flex items-center gap-3 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4">
          <CardIndicator icon={ICON_PATHS.building} count={project.tem_empresa_externa_responsavel ? 1 : 0} color="amber" title={project.tem_empresa_externa_responsavel ? t("project.hasExternalCompanyTitle") : t("project.noExternalCompanyTitle")} hideCount />
          <CardIndicator icon={ICON_PATHS.checkCircle} count={project.is_directly_managed ? 1 : 0} color="emerald" title={project.is_directly_managed ? t("project.directlyManagedTitle") : t("project.notDirectlyManagedTitle")} hideCount />
          <CardIndicator icon={ICON_PATHS.user} count={project.is_responsible ? 1 : 0} color="cyan" title={project.is_responsible ? t("project.isResponsibleTitle") : t("project.notResponsibleTitle")} hideCount />
          <CardIndicatorSeparator />
          {/* Entity link counts — icons visible; counts available when backend adds _count fields */}
          {/* Dimmed until the list endpoint sends the counts — never a fake 0. */}
          <CardIndicator icon={ICON_PATHS.server} count={project.hosts_count} disabled={project.hosts_count === undefined} color="cyan" title={t("project.linkedHostsTitle")} />
          <CardIndicator icon={ICON_PATHS.cube} count={project.services_count} disabled={project.services_count === undefined} color="warning" title={t("project.linkedServicesTitle")} />
          <CardIndicator icon={ICON_PATHS.globe} count={project.dns_count} disabled={project.dns_count === undefined} color="success" title={t("project.linkedDnsTitle")} />
          <CardIndicatorSeparator />
          <CardIndicator icon={ICON_PATHS.code} count={project.gitlab_url ? 1 : 0} color="emerald" title={project.gitlab_url ? "GitLab" : t("project.noGitlabTitle")} hideCount />
          <CardIndicator icon={ICON_PATHS.document} count={project.documentation_url ? 1 : 0} color="sky" title={project.documentation_url ? t("project.hasDocumentationTitle") : t("project.noDocumentationTitle")} hideCount />
        </div>
      </Card>
    </Link>
  );
}
