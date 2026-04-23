"use client";

import { useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { projectsAPI, issuesAPI, graphAPI, integrationsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFilteredGraph } from "@/hooks/useFilteredGraph";
import PageShell from "@/components/layout/PageShell";
import Badge from "@/components/ui/Badge";
import TabBar from "@/components/ui/TabBar";
import Drawer from "@/components/ui/Drawer";
import DetailHeader from "@/components/ui/DetailHeader";
import DetailActions from "@/components/ui/DetailActions";
import { Skeleton } from "@/components/ui/Skeleton";
import ProjectForm from "../ProjectForm";
import DetailKpiSection from "./_components/KpiSection";
import OverviewTab from "./_components/OverviewTab";
import TopologyTab from "./_components/TopologyTab";
import IssueBoard from "./_components/IssueBoard";
import CommitsTab from "./_components/CommitsTab";
import WikiTab from "./_components/WikiTab";
import ChamadosTab from "./_components/ChamadosTab";

type TabKey = "overview" | "topology" | "issues" | "commits" | "wiki" | "chamados";

export default function ProjectDetail({ id }: { id: number }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";

  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [showEditDrawer, setShowEditDrawer] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<ReactNode>(null);

  // -- Data queries --
  const { data, isLoading } = useQuery({ queryKey: ["project", id], queryFn: () => projectsAPI.get(id) });
  const { data: graphData } = useQuery({ queryKey: ["graph"], queryFn: graphAPI.get, enabled: activeTab === "topology" });
  const { data: issues = [] } = useQuery({
    queryKey: ["project-issues", id],
    queryFn: () => issuesAPI.listByProject(id),
  });

  const entityNodeId = data ? `project-${id}` : undefined;
  const filteredGraph = useFilteredGraph(entityNodeId, graphData, activeTab === "topology", 3);

  // -- Mutations --
  const deleteMutation = useMutation({
    mutationFn: () => projectsAPI.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] });
      router.push("/projects");
    },
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

  const tabs: { key: TabKey; label: string; icon?: string; badge?: number }[] = [
    { key: "overview", label: "Overview", icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" },
    { key: "topology", label: "Topology", icon: "M13 10V3L4 14h7v7l9-11h-7z" },
    { key: "issues", label: t("issue.title"), icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" },
    { key: "commits", label: "Commits", icon: "M8 7a4 4 0 118 0 4 4 0 01-8 0zM12 11v8M3 15h6m6 0h6" },
    ...(outlineEnabled ? [{ key: "wiki" as TabKey, label: "Wiki", icon: "M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" }] : []),
    ...(glpiEnabled ? [{ key: "chamados" as TabKey, label: "Chamados", icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" }] : []),
  ];

  const safeIssues = Array.isArray(issues) ? issues : [];

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
          <DetailHeader
            backHref="/projects"
            backLabel={t("common.back")}
            title={data.project.name}
            titleColor="var(--accent)"
            description={data.project.description || undefined}
            badges={
              <Badge variant="situacao" situacao={data.project.situacao} dot>{data.project.situacao}</Badge>
            }
            counters={
              safeIssues.length > 0 ? (
                <span className="inline-flex items-center gap-1 text-xs font-medium text-purple-400">
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  {safeIssues.length}
                </span>
              ) : undefined
            }
          >
            <DetailActions
              canEdit={canEdit}
              isAdmin={isAdmin}
              onEdit={() => setShowEditDrawer(true)}
              onDelete={() => deleteMutation.mutate()}
              deleteConfirmMessage={`Delete project "${data.project.name}"? This cannot be undone.`}
            />
          </DetailHeader>

          <TabBar
            tabs={tabs.map((tab) => ({
              key: tab.key,
              label: tab.label,
              icon: tab.icon,
              badge: tab.key === "issues" && safeIssues.length > 0 ? safeIssues.length : undefined,
            }))}
            activeTab={activeTab}
            onChange={(key) => setActiveTab(key as TabKey)}
          />

          <div className="animate-fade-in">
            {activeTab === "overview" && (
              <>
                <DetailKpiSection
                  servicesCount={data.services?.length || 0}
                  hostsCount={data.host_ids?.length || 0}
                  dnsCount={data.dns_ids?.length || 0}
                  issuesCount={safeIssues.length}
                  t={t}
                />
                <OverviewTab
                  project={data.project}
                  responsaveis={data.responsaveis}
                  services={data.services}
                  hostIds={data.host_ids || []}
                  dnsIds={data.dns_ids || []}
                  t={t}
                />
              </>
            )}
            {activeTab === "topology" && <TopologyTab filteredGraph={filteredGraph} />}
            {activeTab === "issues" && (
              <IssueBoard projectId={id} services={data.services || []} canEdit={canEdit} />
            )}
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
          </div>

          {/* Edit Drawer — now uses ProjectForm component */}
          <Drawer
            open={showEditDrawer}
            onClose={() => setShowEditDrawer(false)}
            title={t("common.edit")}
            subHeader={formSubHeader}
          >
            <ProjectForm
              initial={data.project}
              onSubHeaderChange={setFormSubHeader}
              onSuccess={() => {
                setShowEditDrawer(false);
                queryClient.invalidateQueries({ queryKey: ["project", id] });
                queryClient.invalidateQueries({ queryKey: ["projects"] });
              }}
            />
          </Drawer>
        </div>
      ) : null}
    </PageShell>
  );
}
