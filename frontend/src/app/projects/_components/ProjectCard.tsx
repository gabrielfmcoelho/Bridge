"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { situacaoAccent } from "@/lib/constants";
import { useSituacao } from "@/hooks/useSituacao";
import SituacaoText from "@/components/ui/SituacaoText";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorGrid, CardMeter } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Project } from "@/lib/types";

export default function ProjectCard({ project }: { project: Project }) {
  const { t } = useLocale();
  const { roleOf, colorOf } = useSituacao();
  const total = project.issues_total ?? 0;
  const open = project.issues_count ?? 0;
  const done = total - open;

  return (
    <Link href={`/projects/${project.id}`} className="block h-full">
      <Card accent={situacaoAccent(roleOf(project.situacao), colorOf(project.situacao))} className="h-full flex flex-col overflow-hidden">
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
            { label: t("dns.responsavel"), value: project.main_responsavel_name || "" },
            { label: t("host.entity"), value: project.main_entidade || "" },
            { label: t("project.externalCompany"), value: project.tem_empresa_externa_responsavel ? project.contato_empresa_responsavel || t("common.yes") : "" },
            { label: t("project.managed"), value: project.is_directly_managed ? t("common.yes") : t("common.no") },
          ]}
        />

        <CardTagsSection tags={project.tags} />

        {/* The project's own block: how much of its tracked work is done. */}
        <div className="mt-3 pt-3 pb-1 border-t border-[var(--border-subtle)]">
          <CardMeter
            label={t("project.issuesProgress")}
            pct={total ? Math.round((done / total) * 100) : null}
            reading={total ? `${done}/${total}` : undefined}
            tone={open === 0 ? "success" : done / Math.max(total, 1) >= 0.5 ? "success" : "warning"}
            caption={total ? t("project.openIssues", { count: String(open) }) : undefined}
          />
        </div>

        <CardIndicatorGrid>
          <CardIndicator icon={ICON_PATHS.gear} count={project.services_count ?? 0} color="warning" title={`${project.services_count ?? 0} ${t("host.services").toLowerCase()}`} />
          <CardIndicator icon={ICON_PATHS.server} count={project.hosts_count ?? 0} color="info" title={`${project.hosts_count ?? 0} hosts`} />
          <CardIndicator icon={ICON_PATHS.globe} count={project.dns_count ?? 0} color="success" title={`${project.dns_count ?? 0} DNS`} />
          <CardIndicator icon={ICON_PATHS.clipboard} count={open} color="accent" title={t("project.openIssues", { count: String(open) })} />
          <CardIndicator icon={ICON_PATHS.code} count={project.repos_count ?? 0} color="success" title={t("project.reposCount", { count: String(project.repos_count ?? 0) })} />
          <CardIndicator icon={ICON_PATHS.document} count={project.documentation_url ? 1 : 0} color="info" hideCount title={project.documentation_url ? t("project.hasDocumentationTitle") : t("project.noDocumentationTitle")} />
        </CardIndicatorGrid>
      </Card>
    </Link>
  );
}
