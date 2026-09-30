"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { servicesAPI, hostsAPI, dnsAPI, projectsAPI, globalIssuesAPI, integrationsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useEntityGraph } from "@/hooks/useEntityGraph";
import PageShell from "@/components/layout/PageShell";
import { StatusText } from "@/components/ui/SituacaoText";
import CardIndicator from "@/components/inventory/CardIndicator";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import PageHeader from "@/components/ui/PageHeader";
import { Skeleton } from "@/components/ui/Skeleton";
import DetailSplit from "@/components/detail/DetailSplit";
import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { hostsGroup, dnsGroup, servicesGroup, projectsGroup } from "@/components/detail/RelationsCard";
import IssuesBoard from "@/components/issues/IssuesBoard";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";
import { serviceTitle } from "@/lib/serviceDisplay";
import ServiceForm from "../ServiceForm";
import ServiceProfile from "./_components/ServiceProfile";
import RuntimeCard from "./_components/RuntimeCard";
import CredentialsTab from "./_components/CredentialsTab";
import MetricsTab from "./_components/MetricsTab";

type TabKey = "overview" | "issues" | "topology" | "credentials" | "metrics";

const GENERATED = /^Auto-discovered /;

export default function ServiceDetail({ id }: { id: number }) {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const isAdmin = user?.role === "admin";
  const canEdit = user?.role === "admin" || user?.role === "editor";

  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [showEditDrawer, setShowEditDrawer] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);

  // No retry: a 404 (missing or not visible) should say so at once.
  const { data, isLoading } = useQuery({ queryKey: ["service", id], queryFn: () => servicesAPI.get(id), retry: false });
  // Names for the linked ids; the lists are cached from the inventory pages.
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list });
  const { data: allHosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: allDns = [] } = useQuery({ queryKey: ["dns"], queryFn: dnsAPI.list });
  const { data: allProjects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });
  const { data: issues = [] } = useQuery({
    queryKey: ["issues", "service", id],
    queryFn: () => globalIssuesAPI.list({ entity_type: "service", entity_id: String(id) }),
    enabled: !!data,
  });
  const { data: integrations } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get, retry: false, staleTime: 60_000 });
  const grafanaEnabled = integrations?.grafana?.grafana_enabled === "true";
  const { graph, loading: graphLoading } = useEntityGraph(data ? `service-${id}` : undefined, activeTab === "topology");

  const dependsOn = allServices.filter((s) => data?.depends_on_ids?.includes(s.id));
  const dependents = allServices.filter((s) => data?.dependent_ids?.includes(s.id));
  const linkedHosts = allHosts.filter((h) => data?.host_ids?.includes(h.id));
  const linkedDns = allDns.filter((d) => data?.dns_ids?.includes(d.id));
  const project = allProjects.find((p) => p.id === data?.service.project_id);

  const deleteMutation = useMutation({
    mutationFn: () => servicesAPI.delete(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["services"] }); router.push("/services"); },
  });
  const fixateMutation = useMutation({
    mutationFn: () => servicesAPI.fixate(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["service", id] });
      queryClient.invalidateQueries({ queryKey: ["services"] });
    },
  });

  const openIssues = issues.filter((i) => !i.archived && i.status !== "done").length;
  const tabs = [
    { key: "overview", label: t("host.tabOverview"), icon: ICON_PATHS.home },
    { key: "issues", label: t("host.tabTracking"), icon: ICON_PATHS.alert, badge: openIssues || undefined },
    { key: "topology", label: t("host.tabTopology"), icon: ICON_PATHS.bolt },
    { key: "credentials", label: t("service.credentials"), icon: ICON_PATHS.lock },
    ...(grafanaEnabled ? [{ key: "metrics", label: t("host.tabMetrics"), icon: ICON_PATHS.layoutGrid }] : []),
  ];

  if (isLoading) {
    return (
      <PageShell>
        <div className="space-y-6">
          <Skeleton className="h-4 w-20" />
          <div className="space-y-2"><Skeleton className="h-7 w-48" /><Skeleton className="h-4 w-32" /></div>
          <div className="h-40 skeleton rounded-[var(--radius-lg)]" />
        </div>
      </PageShell>
    );
  }
  if (!data) {
    return (
      <PageShell>
        <EmptyState icon="box" title={t("service.notFound")} description={t("dns.notFoundDesc")}
          action={<Button size="sm" variant="secondary" onClick={() => router.push("/services")}>{t("service.backToList")}</Button>} />
      </PageShell>
    );
  }

  const svc = data.service;
  const { title, mono, id: runtimeId } = serviceTitle(svc);
  const online = svc.container_status === "online";
  const relationGroups = [
    hostsGroup(linkedHosts, t),
    dnsGroup(linkedDns, t),
    { ...servicesGroup(dependsOn, t), title: t("service.dependsOn") },
    { ...servicesGroup(dependents, t), title: t("service.dependents") },
    projectsGroup(project ? [project] : [], t),
  ];

  return (
    <PageShell>
      <div className="space-y-5">
        <PageHeader
          showEmptyDescription
          title={title}
          titleFont={mono ? "mono" : "display"}
          subtitle={runtimeId}
          subtitleFont="mono"
          description={svc.description && !GENERATED.test(svc.description) ? svc.description : undefined}
          status={svc.container_status ? (
            <span className="inline-flex items-center gap-2">
              <StatusText color={online ? "var(--success)" : "var(--text-muted)"} on={online} label={t(`service.status.${svc.container_status}`)} />
              {svc.last_seen_at && <span className="text-xs text-[var(--text-muted)]">{t("service.seenAgo", { ago: getTimeAgo(svc.last_seen_at, locale) })}</span>}
            </span>
          ) : undefined}
          indicators={<>
            <CardIndicator icon={ICON_PATHS.server} count={linkedHosts.length} color="info" title={t("service.hostsCount", { count: String(linkedHosts.length) })} />
            <CardIndicator icon={ICON_PATHS.globe} count={linkedDns.length} color="success" title={t("service.dnsCount", { count: String(linkedDns.length) })} />
            <CardIndicator icon={ICON_PATHS.link} count={dependsOn.length} color="warning" title={t("service.depsCount", { count: String(dependsOn.length) })} />
            <CardIndicator icon={ICON_PATHS.clipboard} count={openIssues} color="accent" title={`${openIssues} ${t("nav.issues").toLowerCase()}`} />
          </>}
          actions={canEdit && svc.source === "auto" ? (
            <Button size="sm" variant="secondary" onClick={() => fixateMutation.mutate()} loading={fixateMutation.isPending} title={t("service.fixateHint")}>
              {t("service.fixate")}
            </Button>
          ) : undefined}
          onEdit={canEdit ? () => setShowEditDrawer(true) : undefined}
          onDelete={isAdmin ? () => deleteMutation.mutate() : undefined}
          deleteConfirmMessage={`${t("confirm.deleteTitle", { name: `"${title}"` })} ${t("confirm.cannotUndo")}`}
          tabs={{ idBase: "service", label: title, active: activeTab, onChange: (k) => setActiveTab(k as TabKey), panelClassName: "space-y-5", items: tabs }}
        >
          {activeTab === "overview" && (
            <DetailSplit profile={<ServiceProfile service={svc} tags={data.tags || []} responsaveis={data.responsaveis || []} projectName={project?.name} onEditResponsaveis={canEdit ? () => setShowEditDrawer(true) : undefined} />}>
              <RuntimeCard service={svc} hosts={linkedHosts} />
            </DetailSplit>
          )}

          {activeTab === "issues" && <IssuesBoard entityType="service" entityId={id} canEdit={canEdit} />}

          {activeTab === "topology" && (
            <TopologyPane graph={graph} loading={graphLoading} t={t}
              hasRelations={relationGroups.some((g) => g.rows.length > 0)}
              relations={<RelationsCard groups={relationGroups} t={t} />} />
          )}

          {activeTab === "credentials" && <CredentialsTab serviceId={id} isAdmin={isAdmin} t={t} />}

          {activeTab === "metrics" && grafanaEnabled && <MetricsTab serviceId={id} nickname={svc.nickname} />}
        </PageHeader>

        <Drawer open={showEditDrawer} onClose={() => setShowEditDrawer(false)} title={t("form.editTitle", { name: title })} subHeader={formSubHeader} footer={formFooter}>
          <ServiceForm
            initial={svc}
            initialGrants={data.entidades}
            initialTags={data.tags}
            initialHostIds={data.host_ids}
            initialDnsIds={data.dns_ids}
            initialDependsOnIds={data.depends_on_ids}
            initialResponsaveis={data.responsaveis}
            onClose={() => setShowEditDrawer(false)}
            onSubHeaderChange={setFormSubHeader}
            onFooterChange={setFormFooter}
            onSuccess={() => {
              setShowEditDrawer(false);
              queryClient.invalidateQueries({ queryKey: ["service", id] });
              queryClient.invalidateQueries({ queryKey: ["services"] });
            }}
          />
        </Drawer>
      </div>
    </PageShell>
  );
}
