"use client";

import { useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { projectsAPI, issuesAPI, integrationsAPI, hostsAPI, dnsAPI, projectEmbedsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useEntityGraph } from "@/hooks/useEntityGraph";
import PageShell from "@/components/layout/PageShell";
import SituacaoText from "@/components/ui/SituacaoText";
import CardIndicator from "@/components/inventory/CardIndicator";
import Drawer from "@/components/ui/Drawer";
import PageHeader from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import DetailSplit from "@/components/detail/DetailSplit";
import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { hostsGroup, dnsGroup, servicesGroup } from "@/components/detail/RelationsCard";
import IssuesBoard from "@/components/issues/IssuesBoard";
import { useFlag } from "@/contexts/FlagContext";
import ProjectForm from "../ProjectForm";
import ProjectProfile from "./_components/ProjectProfile";
import ProjectServices from "./_components/ProjectServices";
import ProjectAiAnalysis from "./_components/ProjectAiAnalysis";
import CommitsTab from "./_components/CommitsTab";
import WikiTab from "./_components/WikiTab";
import ChamadosTab from "./_components/ChamadosTab";
import ProjectReleases from "./_components/ProjectReleases";
import ProjectEmbeds, { projectEmbedsKey } from "./_components/ProjectEmbeds";
import ProjectSecretsTab from "./_components/ProjectSecretsTab";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";

type TabKey = "overview" | "topology" | "issues" | "releases" | "commits" | "wiki" | "chamados" | "observability" | "secrets";

export default function ProjectDetail({ id }: { id: number }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const flag = useFlag();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";

  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [showEditDrawer, setShowEditDrawer] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);

  // -- Data queries --
  // No retry: a 404 (missing or not visible) should say so at once.
  const { data, isLoading } = useQuery({ queryKey: ["project", id], queryFn: () => projectsAPI.get(id), retry: false });
  // The project_id list: entity issues plus older ones tied only by project_id.
  // Same key IssuesBoard uses, so the badge and the board share one fetch.
  const fetchIssues = () => issuesAPI.listByProject(id);
  const { data: issues = [] } = useQuery({ queryKey: ["issues", "project", id], queryFn: fetchIssues, enabled: !!data });
  const { graph, loading: graphLoading } = useEntityGraph(data ? `project-${id}` : undefined, activeTab === "topology");
  // Names for the relation rows (cached from the inventory pages).
  const { data: allHosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list(), enabled: activeTab === "topology" });
  const { data: allDns = [] } = useQuery({ queryKey: ["dns"], queryFn: dnsAPI.list, enabled: activeTab === "topology" });
  // Viewers only get the Observabilidade tab when there's something in it.
  const { data: embeds = [] } = useQuery({ queryKey: projectEmbedsKey(id), queryFn: () => projectEmbedsAPI.list(id), enabled: !!data });

  // -- Mutations --
  const deleteMutation = useMutation({
    mutationFn: () => projectsAPI.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] });
      router.push("/projects");
    },
    onError: (err) => flag({ appearance: "error", title: t("common.delete"), description: err instanceof Error ? err.message : t("form.saveFailed") }),
  });

  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const outlineEnabled = integrations?.outline?.outline_enabled === "true";
  const glpiEnabled = integrations?.glpi?.glpi_enabled === "true";
  const projectGlpiProfileID = data?.project?.glpi_token_id ?? null;

  const projectIssues = Array.isArray(issues) ? issues : [];
  const openIssues = projectIssues.filter((i) => !i.archived && i.status !== "done").length;
  const tabs: { key: TabKey; label: string; icon: string; badge?: number }[] = [
    { key: "overview", label: t("host.tabOverview"), icon: ICON_PATHS.home },
    { key: "topology", label: t("host.tabTopology"), icon: ICON_PATHS.bolt },
    { key: "issues", label: t("host.tabTracking"), icon: ICON_PATHS.alert, badge: openIssues || undefined },
    { key: "releases", label: t("release.title"), icon: NAV_ICONS.Rocket },
    { key: "commits", label: t("project.tab.commits"), icon: ICON_PATHS.code },
    ...(outlineEnabled ? [{ key: "wiki" as TabKey, label: t("project.tab.wiki"), icon: ICON_PATHS.document }] : []),
    ...(glpiEnabled ? [{ key: "chamados" as TabKey, label: t("nav.chamados"), icon: ICON_PATHS.clipboard }] : []),
    ...(canEdit || embeds.length > 0 ? [{ key: "observability" as TabKey, label: t("embed.title"), icon: ICON_PATHS.layoutGrid }] : []),
    { key: "secrets", label: t("project.tab.secrets"), icon: ICON_PATHS.lock },
  ];


  return (
    <PageShell>
      {isLoading ? (
        <div className="space-y-6">
          <Skeleton className="h-4 w-20" />
          <div className="flex justify-between">
            <div className="space-y-2"><Skeleton className="h-7 w-64" /><Skeleton className="h-4 w-24" /></div>
            <Skeleton className="h-6 w-20 rounded-full" />
          </div>
          <div className="h-40 skeleton rounded-[var(--radius-lg)]" />
        </div>
      ) : data ? (
        <div className="space-y-5">
          <PageHeader
            showEmptyDescription
            title={data.project.name}
            description={data.project.description || undefined}
            status={<SituacaoText situacao={data.project.situacao} />}
            indicators={<>
              <CardIndicator icon={ICON_PATHS.gear} count={data.services?.length ?? 0} color="warning" title={`${data.services?.length ?? 0} ${t("host.services").toLowerCase()}`} />
              <CardIndicator icon={ICON_PATHS.server} count={data.host_ids?.length ?? 0} color="info" title={`${data.host_ids?.length ?? 0} hosts`} />
              <CardIndicator icon={ICON_PATHS.globe} count={data.dns_ids?.length ?? 0} color="success" title={`${data.dns_ids?.length ?? 0} DNS`} />
              <CardIndicator icon={ICON_PATHS.clipboard} count={openIssues} color="accent" title={t("project.openIssues", { count: String(openIssues) })} />
            </>}
            onEdit={canEdit ? () => setShowEditDrawer(true) : undefined}
            onDelete={isAdmin ? () => deleteMutation.mutate() : undefined}
            deleteConfirmMessage={`${t("confirm.deleteTitle", { name: `"${data.project.name}"` })} ${t("confirm.cannotUndo")}`}
            tabs={{
              idBase: "project",
              label: data.project.name,
              active: activeTab,
              onChange: (key) => setActiveTab(key as TabKey),
              panelClassName: "animate-fade-in",
              items: tabs,
            }}
          >
            {activeTab === "overview" && (
              <DetailSplit profile={<ProjectProfile project={data.project} tags={data.tags || []} responsaveis={data.responsaveis || []} onEditResponsaveis={canEdit ? () => setShowEditDrawer(true) : undefined} />}>
                <ProjectAiAnalysis projectId={id} />
                <ProjectServices services={data.services || []} />
              </DetailSplit>
            )}
            {activeTab === "topology" && (() => {
              const groups = [
                servicesGroup(data.services || [], t),
                hostsGroup(allHosts.filter((h) => data.host_ids?.includes(h.id)), t),
                dnsGroup(allDns.filter((d) => data.dns_ids?.includes(d.id)), t),
              ];
              return (
                <TopologyPane graph={graph} loading={graphLoading} t={t}
                  hasRelations={groups.some((g) => g.rows.length > 0)}
                  relations={<RelationsCard groups={groups} t={t} />} />
              );
            })()}
            {activeTab === "issues" && (
              <IssuesBoard entityType="project" entityId={id} canEdit={canEdit} fetcher={fetchIssues} />
            )}
            {activeTab === "releases" && <ProjectReleases projectId={id} canEdit={canEdit} canDelete={isAdmin} />}
            {activeTab === "commits" && <CommitsTab projectId={id} />}
            {activeTab === "wiki" && outlineEnabled && <WikiTab projectId={id} canEdit={canEdit} />}
            {activeTab === "chamados" && glpiEnabled && data && (
              <ChamadosTab
                projectId={id}
                projectName={data.project.name}
                profileID={projectGlpiProfileID}
                canEdit={canEdit}
              />
            )}
            {activeTab === "observability" && <ProjectEmbeds projectId={id} canEdit={canEdit} />}
            {activeTab === "secrets" && <ProjectSecretsTab projectId={id} services={data.services || []} canEdit={canEdit} />}
          </PageHeader>

          <Drawer
            open={showEditDrawer}
            onClose={() => setShowEditDrawer(false)}
            title={t("form.editTitle", { name: data.project.name })}
            subHeader={formSubHeader}
            footer={formFooter}
          >
            <ProjectForm
              initial={data.project}
              initialGrants={data.entidades}
              initialTags={data.tags}
              initialResponsaveis={data.responsaveis}
              initialServiceIds={(data.services ?? []).map((s) => s.id)}
              initialHostIds={data.direct_host_ids}
              initialDnsIds={data.direct_dns_ids}
              onClose={() => setShowEditDrawer(false)}
              onSubHeaderChange={setFormSubHeader}
              onFooterChange={setFormFooter}
              onSuccess={() => {
                setShowEditDrawer(false);
                queryClient.invalidateQueries({ queryKey: ["project", id] });
                queryClient.invalidateQueries({ queryKey: ["projects"] });
                // Moving a service here changes it and the project it left.
                queryClient.invalidateQueries({ queryKey: ["services"] });
                queryClient.invalidateQueries({ queryKey: ["graph"] });
              }}
            />
          </Drawer>
        </div>
      ) : (
        <EmptyState icon="folder" title={t("project.notFound")} description={t("dns.notFoundDesc")}
          action={<Button size="sm" variant="secondary" onClick={() => router.push("/projects")}>{t("project.backToList")}</Button>} />
      )}
    </PageShell>
  );
}
