"use client";

import { useState, useMemo, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { dnsAPI, hostsAPI, servicesAPI, projectsAPI, graphAPI, globalIssuesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFilteredGraph } from "@/hooks/useFilteredGraph";
import PageShell from "@/components/layout/PageShell";
import SituacaoText from "@/components/ui/SituacaoText";
import CardIndicator from "@/components/inventory/CardIndicator";
import Drawer from "@/components/ui/Drawer";
import PageHeader from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import DnsForm from "../DnsForm";
import OverviewTab from "./_components/OverviewTab";
import TopologyTab from "./_components/TopologyTab";
import IssuesTab from "./_components/IssuesTab";
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
          <PageHeader
            showEmptyDescription
            title={dns.domain}
            titleFont="mono"
            subtitle={t("dns.record")}
            description={dns.observacoes || undefined}
            status={<SituacaoText situacao={dns.situacao} />}
            indicators={<>
              <CardIndicator icon={ICON_PATHS.lock} count={dns.has_https ? 1 : 0} hideCount color="success" title={t("topology.https")} />
              <CardIndicator icon={ICON_PATHS.alert} count={dnsIssues.length} color="purple" title={t("host.acontecimentos")} />
              {certState(dns) !== "none" && <CertBadge dns={dns} />}
            </>}
            onEdit={canEdit ? () => setShowEditDrawer(true) : undefined}
            onDelete={isAdmin ? () => deleteMutation.mutate() : undefined}
            deleteConfirmMessage={t("dns.deleteConfirm", { name: dns.domain })}
            tabs={{ idBase: "dns", label: dns.domain, active: activeTab, onChange: (k) => setActiveTab(k as TabKey), items: tabs, panelClassName: "space-y-5" }}
          >

          {activeTab === "overview" && (
            <OverviewTab dns={dns} tags={data.tags || []} responsaveis={responsaveis} linkedHosts={linkedHosts} linkedServices={linkedServices} linkedProjects={linkedProjects} canEdit={canEdit} t={t} />
          )}

          {activeTab === "issues" && (
            <IssuesTab issues={dnsIssues} entityType="dns" entityId={id} t={t} canEdit={canEdit} />
          )}

          {activeTab === "topology" && (
            <TopologyTab filteredGraph={filteredGraph} linkedHosts={linkedHosts} linkedServices={linkedServices} />
          )}

          </PageHeader>

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

        </div>
      ) : null}
    </PageShell>
  );
}
