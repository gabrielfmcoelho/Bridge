"use client";

import { useState, useMemo, type ReactNode } from "react";
import { useFilteredGraph } from "@/hooks/useFilteredGraph";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { hostsAPI, sshAPI, graphAPI, globalIssuesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Badge from "@/components/ui/Badge";
import IconButton from "@/components/ui/IconButton";
import FloatingActionButton, { type FABAction } from "@/components/ui/FloatingActionButton";
import { Skeleton } from "@/components/ui/Skeleton";
import TabBar from "@/components/ui/TabBar";
import Drawer from "@/components/ui/Drawer";
import HostForm from "../HostForm";
import SSHOperations from "./_components/SSHOperations";
import TopologyTab from "./_components/TopologyTab";
import OverviewTab from "./_components/OverviewTab";
import ScansTab from "./_components/ScansTab";
import IssuesTab from "./IssuesTab";
import SSHConfigDrawer from "./_components/SSHConfigDrawer";

type TabKey = "overview" | "scans" | "operations" | "alerts" | "topology";

export default function HostDetail({ slug }: { slug: string }) {
  const { t, formatDateTime, locale } = useLocale();
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
    scans: "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z",
    operations: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z",
    alerts: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
    topology: "M13 10V3L4 14h7v7l9-11h-7z",
  };

  const tabs: { key: TabKey; label: string; icon?: string; badge?: number }[] = [
    { key: "overview", label: t("host.tabOverview"), icon: tabIcons.overview },
    { key: "scans", label: t("host.tabScans"), icon: tabIcons.scans },
    ...(canEdit ? [{ key: "operations" as TabKey, label: t("host.tabOperations"), icon: tabIcons.operations }] : []),
    { key: "alerts" as TabKey, label: t("host.tabTracking"), icon: tabIcons.alerts, badge: issuesTabBadge || undefined },
    { key: "topology", label: t("host.tabTopology"), icon: tabIcons.topology },
  ];

  /* ─── Render ─── */

  return (
    <PageShell>
      {/* Back link */}
      <div className="mb-5">
        <Link href="/hosts" className="inline-flex items-center gap-1 text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider hover:text-[var(--accent)] transition-colors">
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          {t("common.back")}
        </Link>
      </div>

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
          {/* ─── Header ─── */}
          <div className="flex-1 min-w-0">
            <h1 className="text-2xl font-bold text-[var(--accent)]" style={{ fontFamily: "var(--font-display)" }}>
              {data.host.nickname}
            </h1>
            <p className="text-[var(--text-muted)] text-sm" style={{ fontFamily: "var(--font-mono)" }}>
              {data.host.oficial_slug}
            </p>
            {data.host.description && (
              <p className="text-[var(--text-secondary)] mt-1.5">{data.host.description}</p>
            )}
            <div className="flex items-center gap-2 mt-2">
              <Badge variant="situacao" situacao={data.host.situacao} compact className="[&>span:first-child]:w-3 [&>span:first-child]:h-3">{data.host.situacao}</Badge>
              {/* Alerts */}
              <span className={`inline-flex items-center gap-1 text-xs ${alertCount > 0 ? "text-amber-400" : "text-[var(--text-faint)]"}`} title={`${alertCount} alerts`}>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                {alertCount > 0 && <span style={{ fontFamily: "var(--font-mono)" }}>{alertCount}</span>}
              </span>
              {/* Issues */}
              <span className={`inline-flex items-center gap-1 text-xs ${openIssuesCount > 0 ? "text-purple-400" : "text-[var(--text-faint)]"}`} title={`${openIssuesCount} open issues`}>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                </svg>
                {openIssuesCount > 0 && <span style={{ fontFamily: "var(--font-mono)" }}>{openIssuesCount}</span>}
              </span>
              {/* Chamados */}
              <span className={`inline-flex items-center gap-1 text-xs ${(data.host.chamados_count || 0) > 0 ? "text-orange-400" : "text-[var(--text-faint)]"}`} title={`${data.host.chamados_count || 0} chamados`}>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                {(data.host.chamados_count || 0) > 0 && <span style={{ fontFamily: "var(--font-mono)" }}>{data.host.chamados_count}</span>}
              </span>
            </div>
          </div>

          {/* ─── Tab bar + actions ─── */}
          <div className="flex items-center justify-between gap-3">
            <TabBar tabs={tabs} activeTab={activeTab} onChange={(k) => setActiveTab(k as TabKey)} />
            <div className="hidden md:flex items-center gap-1.5 shrink-0">
              <IconButton onClick={() => setShowSSHConfigDrawer(true)} title={t("host.sshConfig")}>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                </svg>
              </IconButton>
              {canEdit && (
                <IconButton onClick={() => setShowEditDrawer(true)} title={t("common.edit")}>
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                  </svg>
                </IconButton>
              )}
              {isAdmin && (
                <IconButton variant="danger" onClick={() => { if (confirm(`Delete "${data.host.nickname}"?`)) deleteMutation.mutate(); }} title={t("common.delete")}>
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </IconButton>
              )}
            </div>
          </div>

          {/* ═══ OVERVIEW ═══ */}
          {activeTab === "overview" && (
            <OverviewTab
              host={data.host}
              tags={data.tags}
              responsaveis={data.responsaveis ?? []}
              chamados={data.chamados ?? []}
              canEdit={canEdit}
              isAdmin={isAdmin}
              slug={slug}
              t={t}
            />
          )}

          {/* ═══ SCANS TAB ═══ */}
          {activeTab === "scans" && (
            <ScansTab lastScan={data.last_scan ?? undefined} formatDateTime={formatDateTime} locale={locale} t={t} />
          )}

          {/* ═══ OPERATIONS TAB ═══ */}
          {activeTab === "operations" && canEdit && (
            <div className="animate-fade-in">
              <SSHOperations
                slug={data.host.oficial_slug}
                hasPassword={data.host.has_password}
                hasKey={!!data.host.key_path}
                preferredAuth={data.host.preferred_auth}
                passwordTestStatus={data.host.password_test_status}
                keyTestStatus={data.host.key_test_status}
                dockerGroupStatus={data.host.docker_group_status}
                coolifyServerUUID={data.host.coolify_server_uuid}
                serverInfo={serverInfo}
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

      {/* Mobile FAB */}
      {data && (
        <FloatingActionButton
          actions={[
            ...(canEdit ? [{
              label: t("common.edit") + " Host",
              icon: "M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z",
              onClick: () => setShowEditDrawer(true),
            }] : []),
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
            ...(isAdmin ? [{
              label: t("common.delete") + " Host",
              icon: "M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16",
              onClick: () => { if (confirm(`Delete "${data.host.nickname}"?`)) deleteMutation.mutate(); },
              color: "#ef4444",
            }] : []),
          ] satisfies FABAction[]}
        />
      )}
    </PageShell>
  );
}
