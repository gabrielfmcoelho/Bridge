"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
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
  const fallbackColor = project.situacao === "active" ? "#10b981" : project.situacao === "maintenance" ? "#f59e0b" : "#6b7280";

  return (
    <Link href={`/projects/${project.id}`}>
      <Card className="h-full border-l-[3px] flex flex-col overflow-hidden" style={{ borderLeftColor: situacaoColor || fallbackColor }} clickIndicator="link">
        <CardHeader
          title={project.name}
          subtitle={project.setor_responsavel || undefined}
          description={project.description || t("common.noDescription") || "-"}
          badge={
            <Badge variant="situacao" situacao={project.situacao} compact>
              {project.situacao}
            </Badge>
          }
        />

        <CardMetadataGrid
          items={[
            { label: t("project.responsavel") || "Responsavel", value: project.responsavel || "-" },
            { label: t("project.externalCompany") || "Empresa Externa", value: project.contato_empresa_responsavel || "-" },
            { label: t("project.managed") || "Gerenciado", value: project.is_directly_managed ? t("common.yes") || "Yes" : t("common.no") || "No" },
            { label: t("common.status"), value: project.situacao || "-" },
          ]}
        />

        <CardTagsSection tags={project.tags} />

        {/* Bottom indicators */}
        <div className="flex items-center gap-3 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4">
          <CardIndicator icon={ICON_PATHS.building} count={project.tem_empresa_externa_responsavel ? 1 : 0} color="amber" title={project.tem_empresa_externa_responsavel ? "External company" : "No external company"} />
          <CardIndicator icon={ICON_PATHS.checkCircle} count={project.is_directly_managed ? 1 : 0} color="emerald" title={project.is_directly_managed ? "Directly managed" : "Not directly managed"} />
          <CardIndicator icon={ICON_PATHS.user} count={project.is_responsible ? 1 : 0} color="cyan" title={project.is_responsible ? "Is responsible" : "Not responsible"} />
          <CardIndicatorSeparator />
          {/* Entity link counts — icons visible; counts available when backend adds _count fields */}
          <CardIndicator icon={ICON_PATHS.server} count={(project as unknown as { hosts_count?: number }).hosts_count || 0} color="cyan" title="Linked hosts" />
          <CardIndicator icon={ICON_PATHS.cube} count={(project as unknown as { services_count?: number }).services_count || 0} color="amber" title="Linked services" />
          <CardIndicator icon={ICON_PATHS.globe} count={(project as unknown as { dns_count?: number }).dns_count || 0} color="emerald" title="Linked DNS" />
          <CardIndicatorSeparator />
          <CardIndicator icon={ICON_PATHS.code} count={project.gitlab_url ? 1 : 0} color="emerald" title={project.gitlab_url ? "GitLab" : "No GitLab"} />
          <CardIndicator icon={ICON_PATHS.document} count={project.documentation_url ? 1 : 0} color="sky" title={project.documentation_url ? "Documentation" : "No documentation"} />
        </div>
      </Card>
    </Link>
  );
}
