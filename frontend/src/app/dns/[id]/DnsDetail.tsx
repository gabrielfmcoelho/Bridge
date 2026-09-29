"use client";

import { useState, useMemo, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { dnsAPI, hostsAPI, servicesAPI, projectsAPI, globalIssuesAPI } from "@/lib/api";
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
import DnsForm from "../DnsForm";
import DnsProfile from "./_components/DnsProfile";
import CertificateCard from "./_components/CertificateCard";
import DetailSplit from "@/components/detail/DetailSplit";
import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { hostsGroup, servicesGroup, projectsGroup } from "@/components/detail/RelationsCard";
import IssuesBoard from "@/components/issues/IssuesBoard";
import EmptyState from "@/components/ui/EmptyState";
import { ICON_PATHS } from "@/lib/icon-paths";
import { certState, certTone } from "@/lib/dnsCert";
import { certLabel } from "../_components/CertBadge";

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
  // No retry: a 404 (missing or not visible) should say so at once, not after backoff.
  const { data, isLoading } = useQuery({ queryKey: ["dns", id], queryFn: () => dnsAPI.get(id), retry: false });
  const { data: allHosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list });
  const { data: allProjects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });
  const { data: dnsIssues = [] } = useQuery({
    queryKey: ["issues", "dns", id],
    queryFn: () => globalIssuesAPI.list({ entity_type: "dns", entity_id: String(id) }),
    enabled: !!data,
  });

  const dns = data?.dns_record;
  const responsaveis = data?.responsaveis || [];
  const { graph, loading: graphLoading } = useEntityGraph(data ? `dns-${id}` : undefined, activeTab === "topology");

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
      icon: ICON_PATHS.home,
    },
    {
      key: "issues",
      label: t("host.tabTracking"),
      icon: ICON_PATHS.alert,
      badge: dnsIssues.length || undefined,
    },
    {
      key: "topology",
      label: t("host.tabTopology"),
      icon: ICON_PATHS.bolt,
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
              <CardIndicator icon={ICON_PATHS.lock} count={dns.has_https ? 1 : 0} hideCount color={certTone(certState(dns)) === "default" ? "success" : certTone(certState(dns))} title={certLabel(dns, t) || t("topology.noHttps")} />
              <CardIndicator icon={ICON_PATHS.server} count={linkedHosts.length} color="info" title={t("dns.hostCount", { count: String(linkedHosts.length) })} />
              <CardIndicator icon={ICON_PATHS.gear} count={linkedServices.length} color="warning" title={`${linkedServices.length} ${t("host.services").toLowerCase()}`} />
              <CardIndicator icon={ICON_PATHS.clipboard} count={dnsIssues.length} color="accent" title={`${dnsIssues.length} ${t("nav.issues").toLowerCase()}`} />
            </>}
            onEdit={canEdit ? () => setShowEditDrawer(true) : undefined}
            onDelete={isAdmin ? () => deleteMutation.mutate() : undefined}
            deleteConfirmMessage={t("dns.deleteConfirm", { name: dns.domain })}
            tabs={{ idBase: "dns", label: dns.domain, active: activeTab, onChange: (k) => setActiveTab(k as TabKey), items: tabs, panelClassName: "space-y-5" }}
          >

          {activeTab === "overview" && (
            <DetailSplit profile={<DnsProfile dns={dns} tags={data.tags || []} responsaveis={responsaveis} />}>
              <CertificateCard dns={dns} canEdit={canEdit} />
            </DetailSplit>
          )}

          {activeTab === "issues" && (
            <IssuesBoard entityType="dns" entityId={id} canEdit={canEdit} />
          )}

          {activeTab === "topology" && (() => {
            const groups = [hostsGroup(linkedHosts, t), servicesGroup(linkedServices, t), projectsGroup(linkedProjects, t)];
            return (
              <TopologyPane graph={graph} loading={graphLoading} t={t}
                hasRelations={groups.some((g) => g.rows.length > 0)}
                relations={<RelationsCard groups={groups} t={t} />} />
            );
          })()}

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
      ) : (
        <EmptyState icon="globe" title={t("dns.notFound")} description={t("dns.notFoundDesc")}
          action={<Button size="sm" variant="secondary" onClick={() => router.push("/dns")}>{t("dns.backToList")}</Button>} />
      )}
    </PageShell>
  );
}
