"use client";

import { useState, useMemo, type ReactNode } from "react";
import { useFilteredGraph } from "@/hooks/useFilteredGraph";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { hostsAPI, sshAPI, graphAPI, globalIssuesAPI, integrationsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import IconButton from "@/components/ui/IconButton";
import FloatingActionButton, { type FABAction } from "@/components/ui/FloatingActionButton";
import { Skeleton } from "@/components/ui/Skeleton";
import PageHeader from "@/components/ui/PageHeader";
import SituacaoText from "@/components/ui/SituacaoText";
import { CardIndicator } from "@/components/inventory";
import Drawer from "@/components/ui/Drawer";
import HostForm from "../HostForm";
import SSHOperations from "./_components/SSHOperations";
import TopologyTab from "./_components/TopologyTab";
import HostProfile from "./_components/HostProfile";
import ScanPane from "./_components/ScanPane";
import MetricsTab from "./_components/MetricsTab";
import IssuesTab from "./IssuesTab";
import SSHConfigDrawer from "./_components/SSHConfigDrawer";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

type TabKey = "overview" | "operations" | "alerts" | "topology" | "metrics";

export default function HostDetail({ slug }: { slug: string }) {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const router = useRouter();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";

  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [showEditDrawer, setShowEditDrawer] = useState(false);
  const [formFooter, setFormFooter] = useState<ReactNode>(null);
  const [formSubHeader, setFormSubHeader] = useState<ReactNode>(null);
  const [openAlertCreate, setOpenAlertCreate] = useState(false);
  const [openIssueCreate, setOpenIssueCreate] = useState(false);
  const [openChamadoCreate, setOpenChamadoCreate] = useState(false);
  const [showSSHConfigDrawer, setShowSSHConfigDrawer] = useState(false);

  /* ─── Queries ─── */

  const { data, isLoading } = useQuery({
    queryKey: ["host", slug],
    queryFn: () => hostsAPI.get(slug),
  });

  const { data: serverInfo } = useQuery({
    queryKey: ["ssh-server-info"],
    queryFn: sshAPI.serverInfo,
  });

  const { data: hostsList = [] } = useQuery({
    queryKey: ["hosts"],
    queryFn: () => hostsAPI.list(),
  });
  const hostAlerts = useMemo(() => {
    if (!data) return [];
    const match = hostsList.find((h) => h.oficial_slug === slug);
    return match?.alerts || data.host.alerts || [];
  }, [hostsList, data, slug]);

  const { data: graphData } = useQuery({ queryKey: ["graph"], queryFn: graphAPI.get, enabled: activeTab === "topology" });
  const { data: hostIssues = [] } = useQuery({
    queryKey: ["issues", "host", data?.host?.id],
    queryFn: () => globalIssuesAPI.list({ entity_type: "host", entity_id: String(data!.host.id) }),
    enabled: !!data?.host?.id,
  });

  const entityNodeId = data ? `host-${data.host.id}` : undefined;
  const filteredGraph = useFilteredGraph(entityNodeId, graphData, activeTab === "topology");

  /* ─── Mutations ─── */

  const deleteMutation = useMutation({
    mutationFn: () => hostsAPI.delete(slug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
      router.push("/hosts");
    },
  });

  /* ─── Computed values ─── */

  const alertCount = hostAlerts.length;
  const openIssuesCount = hostIssues.filter((i) => i.status !== "done").length;
  const issuesTabBadge = alertCount + openIssuesCount;

  const tabIcons: Record<string, string> = {
    overview: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0h4",
    operations: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z",
    alerts: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
    topology: "M13 10V3L4 14h7v7l9-11h-7z",
  };

  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const grafanaEnabled = integrations?.grafana?.grafana_enabled === "true";

  const tabs: { key: TabKey; label: string; icon?: string; badge?: number }[] = [
    { key: "overview", label: t("host.tabOverview"), icon: tabIcons.overview },
    ...(canEdit ? [{ key: "operations" as TabKey, label: t("host.tabOperations"), icon: tabIcons.operations }] : []),
    { key: "alerts" as TabKey, label: t("host.tabTracking"), icon: tabIcons.alerts, badge: issuesTabBadge || undefined },
    { key: "topology", label: t("host.tabTopology"), icon: tabIcons.topology },
    ...(grafanaEnabled ? [{ key: "metrics" as TabKey, label: "Metrics", icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" }] : []),
  ];

  /* ─── Render ─── */

  return (
    <PageShell>
      {isLoading ? (
        <div className="space-y-6">
          <div className="flex justify-between">
            <div className="space-y-2"><Skeleton className="h-7 w-48" /><Skeleton className="h-4 w-32" /></div>
            <Skeleton className="h-6 w-20 rounded-full" />
          </div>
          <div className="h-40 skeleton rounded-[var(--radius-lg)]" />
        </div>
      ) : data ? (
        <div className="space-y-5">
          <PageHeader
            showEmptyDescription
            title={data.host.nickname}
            subtitle={data.host.oficial_slug}
            subtitleFont="mono"
            status={<SituacaoText situacao={data.host.situacao} />}
            indicators={
              <>
                <CardIndicator icon={ICON_PATHS.alert} count={alertCount} color="warning" title={t("host.alertsTitle", { count: String(alertCount) })} />
                <CardIndicator icon={ICON_PATHS.clipboard} count={openIssuesCount} color="accent" title={t("host.openIssuesTitle", { count: String(openIssuesCount) })} />
                <CardIndicator icon={ICON_PATHS.document} count={data.host.chamados_count || 0} color="warning" title={t("host.chamadosTitle", { count: String(data.host.chamados_count || 0) })} />
                <CardIndicator
                  icon={ICON_PATHS.moon}
                  count={data.host.idle ? 1 : 0}
                  color="info"
                  hideCount
                  disabled={!data.host.has_scan}
                  title={data.host.idle ? `${t("host.idle")}: ${(data.host.idle_reasons ?? []).join("; ") || t("host.idleNoWorkloads")}` : `${t("host.idle")}: ${t("host.idleNotFlaggedShort")}`}
                />
              </>
            }
            description={data.host.description || undefined}
            actions={
              <IconButton variant="outline" onClick={() => setShowSSHConfigDrawer(true)} label={t("host.sshConfig")}>
                <Icon path={ICON_PATHS.code} />
              </IconButton>
            }
            onEdit={canEdit ? () => setShowEditDrawer(true) : undefined}
            onDelete={isAdmin ? () => deleteMutation.mutate() : undefined}
            deleteConfirmMessage={`${t("confirm.deleteTitle", { name: `"${data.host.nickname}"` })} ${t("confirm.cannotUndo")}`}
            tabs={{
              idBase: "host",
              label: data.host.nickname,
              active: activeTab,
              onChange: (k) => setActiveTab(k as TabKey),
              items: tabs,
              panelClassName: "space-y-5",
            }}
          >


          {/* ═══ OVERVIEW ═══ */}
          {/* Declared (left, stays in view) beside observed (right, scrolls). */}
          {activeTab === "overview" && (
            <div className="lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] gap-6 max-lg:space-y-6 animate-fade-in">
              <aside className="lg:sticky lg:top-0 lg:self-start lg:max-h-[calc(100vh-5.5rem)] lg:overflow-y-auto">
                <HostProfile
                  host={data.host}
                  tags={data.tags}
                  responsaveis={data.responsaveis ?? []}
                  canEdit={canEdit}
                  isAdmin={isAdmin}
                  slug={slug}
                  t={t}
                />
              </aside>
              <ScanPane slug={slug} host={data.host} lastScan={data.last_scan} canEdit={canEdit} />
            </div>
          )}

          {/* ═══ OPERATIONS TAB ═══ */}
          {activeTab === "operations" && canEdit && (
            <div className="animate-fade-in">
              <SSHOperations
                slug={data.host.oficial_slug}
                hasPassword={data.host.has_password}
                hasKey={data.host.has_key}
                preferredAuth={data.host.preferred_auth}
                passwordTestStatus={data.host.password_test_status}
                keyTestStatus={data.host.key_test_status}
                dockerGroupStatus={data.host.docker_group_status}
                coolifyServerUUID={data.host.coolify_server_uuid}
                serverInfo={serverInfo}
                lastScan={data.last_scan ?? undefined}
                t={t}
                locale={locale}
                isAdmin={isAdmin}
              />
            </div>
          )}

          {/* ═══ ISSUES TAB ═══ */}
          {activeTab === "alerts" && (
            <IssuesTab
              hostAlerts={hostAlerts}
              chamados={data.chamados ?? []}
              hostId={data.host.id}
              slug={slug}
              canEdit={canEdit}
              openAlertCreate={openAlertCreate}
              onAlertCreateDone={() => setOpenAlertCreate(false)}
              openIssueCreate={openIssueCreate}
              onIssueCreateDone={() => setOpenIssueCreate(false)}
              openChamadoCreate={openChamadoCreate}
              onChamadoCreateDone={() => setOpenChamadoCreate(false)}
            />
          )}

          {/* ═══ TOPOLOGY TAB ═══ */}
          {activeTab === "topology" && (
            <TopologyTab data={data} filteredGraph={filteredGraph} t={t} />
          )}

          {/* ═══ METRICS TAB ═══ */}
          {activeTab === "metrics" && grafanaEnabled && <MetricsTab slug={slug} />}
          </PageHeader>
        </div>
      ) : null}

      {/* Edit Drawer */}
      {data && (
        <Drawer
          open={showEditDrawer}
          onClose={() => setShowEditDrawer(false)}
          title={t("common.edit") + " " + data.host.nickname}
          subHeader={formSubHeader}
          footer={formFooter}
        >
          <HostForm
            host={data.host}
            tags={data.tags}
            responsaveis={data.responsaveis ?? []}
            chamados={data.chamados ?? []}
            entidades={data.entidades}
            dnsRecords={data.dns_records ?? []}
            services={data.services ?? []}
            projects={data.projects ?? []}
            onClose={() => setShowEditDrawer(false)}
            onFooterChange={setFormFooter}
            onSubHeaderChange={setFormSubHeader}
            onSuccess={() => {
              setShowEditDrawer(false);
              queryClient.invalidateQueries({ queryKey: ["host", slug] });
              queryClient.invalidateQueries({ queryKey: ["hosts"] });
            }}
          />
        </Drawer>
      )}

      {data && (
        <SSHConfigDrawer
          open={showSSHConfigDrawer}
          onClose={() => setShowSSHConfigDrawer(false)}
          slug={slug}
          host={data.host}
        />
      )}

      {/* Mobile FAB: the tab's "add" actions (edit/delete are in the header on phones too). */}
      {data && canEdit && activeTab === "alerts" && (
        <FloatingActionButton
          actions={[
            ...(canEdit && activeTab === "alerts" ? [
              {
                label: t("host.addChamado"),
                icon: "M15 5v2m0 4v2m0 4v2M5 5a2 2 0 00-2 2v3a2 2 0 110 4v3a2 2 0 002 2h14a2 2 0 002-2v-3a2 2 0 110-4V7a2 2 0 00-2-2H5z",
                onClick: () => setOpenChamadoCreate(true),
              },
              {
                label: t("host.addAlert"),
                icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
                onClick: () => setOpenAlertCreate(true),
              },
              {
                label: t("host.addIssue"),
                icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
                onClick: () => setOpenIssueCreate(true),
              },
            ] : []),
          ] satisfies FABAction[]}
        />
      )}
    </PageShell>
  );
}
