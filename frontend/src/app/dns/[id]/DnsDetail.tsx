"use client";

import { useState, useMemo, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { dnsAPI, hostsAPI, servicesAPI, projectsAPI, graphAPI, globalIssuesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFilteredGraph } from "@/hooks/useFilteredGraph";
import PageShell from "@/components/layout/PageShell";
import Badge from "@/components/ui/Badge";
import TabBar from "@/components/ui/TabBar";
import Drawer from "@/components/ui/Drawer";
import DetailHeader from "@/components/ui/DetailHeader";
import DetailActions from "@/components/ui/DetailActions";
import FloatingActionButton, { type FABAction } from "@/components/ui/FloatingActionButton";
import { Skeleton } from "@/components/ui/Skeleton";
import DnsForm from "../DnsForm";
import OverviewTab from "./_components/OverviewTab";
import TopologyTab from "./_components/TopologyTab";
import IssuesTab from "./_components/IssuesTab";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { certState } from "@/lib/dnsCert";
import CertBadge from "../_components/CertBadge";

type TabKey = "overview" | "topology" | "issues";

export default function DnsDetail({ id }: { id: number }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const router = useRouter();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";

  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [showEditDrawer, setShowEditDrawer] = useState(false);
  const [formFooter, setFormFooter] = useState<ReactNode>(null);
  const [formSubHeader, setFormSubHeader] = useState<ReactNode>(null);

  // ── Data queries ──
  const { data, isLoading } = useQuery({ queryKey: ["dns", id], queryFn: () => dnsAPI.get(id) });
  const { data: allHosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list });
  const { data: allProjects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });
  const { data: graphData } = useQuery({ queryKey: ["graph"], queryFn: graphAPI.get, enabled: activeTab === "topology" });
  const { data: dnsIssues = [] } = useQuery({
    queryKey: ["issues", "dns", id],
    queryFn: () => globalIssuesAPI.list({ entity_type: "dns", entity_id: String(id) }),
    enabled: !!data,
  });

  const dns = data?.dns_record;
  const responsaveis = data?.responsaveis || [];
  const entityNodeId = data ? `dns-${id}` : undefined;
  const filteredGraph = useFilteredGraph(entityNodeId, graphData, activeTab === "topology");

  const linkedHosts = useMemo(() => {
    if (!data?.host_ids || !allHosts.length) return [];
    return allHosts.filter((h) => data.host_ids.includes(h.id));
  }, [data, allHosts]);

  const linkedServices = useMemo(() => {
    const ids = data?.service_ids ?? [];
    return allServices.filter((s) => ids.includes(s.id));
  }, [data, allServices]);

  const linkedProjects = useMemo(() => {
    const ids = data?.project_ids ?? [];
    return allProjects.filter((p) => ids.includes(p.id));
  }, [data, allProjects]);

  // ── Mutations ──
  const deleteMutation = useMutation({
    mutationFn: () => dnsAPI.delete(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["dns"] }); router.push("/dns"); },
  });

  const tabs: { key: TabKey; label: string; icon?: string; badge?: number }[] = [
    {
      key: "overview",
      label: t("host.tabOverview"),
      icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
    },
    {
      key: "issues",
      label: t("host.acontecimentos"),
      icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
      badge: dnsIssues.length || undefined,
    },
    {
      key: "topology",
      label: t("host.tabTopology"),
      icon: "M13 10V3L4 14h7v7l9-11h-7z",
    },
  ];

  const fabActions: FABAction[] = [];
  if (canEdit) {
    fabActions.push({ label: t("common.edit"), icon: "M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z", onClick: () => setShowEditDrawer(true) });
  }
  if (isAdmin) {
    fabActions.push({ label: t("common.delete"), icon: "M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16", onClick: () => deleteMutation.mutate(), color: "#ef4444" });
  }

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
      ) : dns ? (
        <div className="space-y-5">
          <DetailHeader
            backHref="/dns"
            backLabel={t("common.back")}
            title={dns.domain}
            titleFont="mono"
            titleColor="var(--accent)"
            subtitle={t("dns.record")}
            badges={
              <>
                <Badge variant="situacao" situacao={dns.situacao} dot>{dns.situacao}</Badge>
                {dns.has_https && (
                  <Badge color="emerald">
                    <Icon path={ICON_PATHS.lock} className="w-3 h-3 mr-1" />
                    {t("topology.https")}
                  </Badge>
                )}
                {certState(dns) !== "none" && <CertBadge dns={dns} />}
              </>
            }
            counters={
              dnsIssues.length > 0 ? (
                <span className="inline-flex items-center gap-1 text-xs font-medium text-[var(--accent)]">
                  <Icon path={ICON_PATHS.alert} className="w-3.5 h-3.5" />
                  {dnsIssues.length}
                </span>
              ) : undefined
            }
          >
            <DetailActions
              canEdit={canEdit}
              isAdmin={isAdmin}
              onEdit={() => setShowEditDrawer(true)}
              onDelete={() => deleteMutation.mutate()}
              deleteConfirmMessage={t("dns.deleteConfirm", { name: dns.domain })}
            />
          </DetailHeader>

          <TabBar
            tabs={tabs}
            activeTab={activeTab}
            onChange={(key) => setActiveTab(key as TabKey)}
          />

          {activeTab === "overview" && (
            <OverviewTab dns={dns} tags={data.tags || []} responsaveis={responsaveis} linkedHosts={linkedHosts} linkedServices={linkedServices} linkedProjects={linkedProjects} canEdit={canEdit} t={t} />
          )}

          {activeTab === "issues" && (
            <IssuesTab issues={dnsIssues} entityType="dns" entityId={id} t={t} canEdit={canEdit} />
          )}

          {activeTab === "topology" && (
            <TopologyTab filteredGraph={filteredGraph} linkedHosts={linkedHosts} linkedServices={linkedServices} />
          )}

          {/* Edit Drawer — now uses DnsForm component */}
          <Drawer
            open={showEditDrawer}
            onClose={() => setShowEditDrawer(false)}
            title={t("common.edit")}
            subHeader={formSubHeader}
            footer={formFooter}
          >
            <DnsForm
              initial={dns}
              initialTags={data.tags}
              initialHostIds={data.host_ids}
              initialServiceIds={data.service_ids}
              initialProjectIds={data.project_ids}
              initialResponsaveis={responsaveis}
              initialGrants={data.entidades}
              onSuccess={() => {
                queryClient.invalidateQueries({ queryKey: ["dns", id] });
                queryClient.invalidateQueries({ queryKey: ["dns"] });
                setShowEditDrawer(false);
              }}
              onFooterChange={setFormFooter}
              onSubHeaderChange={setFormSubHeader}
            />
          </Drawer>

          {fabActions.length > 0 && <FloatingActionButton actions={fabActions} />}
        </div>
      ) : null}
    </PageShell>
  );
}
