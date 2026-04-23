"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { servicesAPI, issuesAPI, hostsAPI, dnsAPI, graphAPI, integrationsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFilteredGraph } from "@/hooks/useFilteredGraph";
import PageShell from "@/components/layout/PageShell";
import Badge from "@/components/ui/Badge";
import TabBar from "@/components/ui/TabBar";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import DetailHeader from "@/components/ui/DetailHeader";
import DetailActions from "@/components/ui/DetailActions";
import { Skeleton } from "@/components/ui/Skeleton";
import ServiceForm from "../ServiceForm";
import OverviewTab from "./_components/OverviewTab";
import ConnectionsTab from "./_components/ConnectionsTab";
import TopologyTab from "./_components/TopologyTab";
import CredentialsTab from "./_components/CredentialsTab";
import IssuesTab from "./_components/IssuesTab";
import MetricsTab from "./_components/MetricsTab";

type TabKey = "overview" | "connections" | "topology" | "credentials" | "issues" | "metrics";

const VIEW_TABS: { key: TabKey; label: string; icon?: string }[] = [
  { key: "overview", label: "Overview", icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" },
  { key: "connections", label: "Connections", icon: "M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" },
  { key: "topology", label: "Topology", icon: "M13 10V3L4 14h7v7l9-11h-7z" },
  { key: "credentials", label: "Credentials", icon: "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" },
  { key: "issues", label: "Issues", icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" },
];

export default function ServiceDetail({ id }: { id: number }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const isAdmin = user?.role === "admin";
  const canEdit = user?.role === "admin" || user?.role === "editor";

  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [showEditDrawer, setShowEditDrawer] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);

  // ── Data queries ──
  const { data, isLoading } = useQuery({
    queryKey: ["service", id],
    queryFn: () => servicesAPI.get(id),
  });

  const { data: allServices = [] } = useQuery({
    queryKey: ["services"],
    queryFn: servicesAPI.list,
  });

  const { data: allHosts = [] } = useQuery({
    queryKey: ["hosts"],
    queryFn: () => hostsAPI.list(),
  });

  const { data: allDns = [] } = useQuery({
    queryKey: ["dns"],
    queryFn: dnsAPI.list,
  });

  const { data: issues = [] } = useQuery({
    queryKey: ["service-issues", id],
    queryFn: () => issuesAPI.listByService(id),
  });

  const { data: graphData } = useQuery({
    queryKey: ["graph"],
    queryFn: graphAPI.get,
    enabled: activeTab === "topology",
  });

  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const grafanaEnabled = integrations?.grafana?.grafana_enabled === "true";

  // ── Derived data ──
  const dependsOnServices = allServices.filter((s) => data?.depends_on_ids?.includes(s.id));
  const dependentServices = allServices.filter((s) => data?.dependent_ids?.includes(s.id));
  const linkedHosts = allHosts.filter((h) => data?.host_ids?.includes(h.id));
  const linkedDns = allDns.filter((d) => data?.dns_ids?.includes(d.id));

  const entityNodeId = data ? `service-${id}` : undefined;
  const filteredGraph = useFilteredGraph(entityNodeId, graphData, activeTab === "topology");

  // ── Mutations ──
  const deleteMutation = useMutation({
    mutationFn: () => servicesAPI.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["services"] });
      router.push("/services");
    },
  });

  const fixateMutation = useMutation({
    mutationFn: () => servicesAPI.fixate(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["service", id] });
      queryClient.invalidateQueries({ queryKey: ["services"] });
    },
  });

  return (
    <PageShell>
      {isLoading ? (
        <div className="space-y-6">
          <Skeleton className="h-4 w-20" />
          <div className="flex justify-between">
            <div className="space-y-2">
              <Skeleton className="h-7 w-48" />
              <Skeleton className="h-4 w-32" />
            </div>
            <Skeleton className="h-6 w-20 rounded-full" />
          </div>
          <div className="h-40 skeleton rounded-[var(--radius-lg)]" />
        </div>
      ) : data ? (
        <div className="space-y-5">
          {/* ── Header ── */}
          <DetailHeader
            backHref="/services"
            backLabel={t("common.back")}
            title={data.service.nickname}
            titleFont="mono"
            titleColor="var(--accent)"
            subtitle={data.service.service_type ? `${data.service.service_type}${data.service.service_subtype ? ` / ${data.service.service_subtype}` : ""}` : undefined}
            description={data.service.description}
            badges={
              <>
                {data.service.source !== "manual" && (
                  <Badge color={data.service.source === "auto" ? "blue" : "emerald"} compact>
                    {data.service.source === "auto" ? t("service.sourceAuto") : t("service.sourceFixed")}
                  </Badge>
                )}
                {data.service.container_status && (
                  <Badge color={data.service.container_status === "online" ? "emerald" : "default"} compact>
                    {data.service.container_status === "online" ? t("service.containerOnline") : t("service.containerOffline")}
                  </Badge>
                )}
                {data.service.is_external_dependency ? (
                  <Badge color="red" compact>{t("service.isExternalDependency") || "External Dep"}</Badge>
                ) : (
                  <Badge color={data.service.developed_by === "internal" ? "cyan" : "amber"} compact>
                    {data.service.developed_by === "internal" ? t("service.internal") || "Internal" : t("service.external") || "External"}
                  </Badge>
                )}
                {data.service.environment && <Badge>{data.service.environment}</Badge>}
                {data.service.technology_stack && <Badge>{data.service.technology_stack}</Badge>}
              </>
            }
          >
            <div className="flex items-center gap-2">
              {canEdit && data.service.source === "auto" && (
                <Button size="sm" variant="secondary" onClick={() => fixateMutation.mutate()} loading={fixateMutation.isPending}>
                  {t("service.fixate")}
                </Button>
              )}
              <DetailActions
                canEdit={canEdit}
                isAdmin={isAdmin}
                onEdit={() => setShowEditDrawer(true)}
                onDelete={() => deleteMutation.mutate()}
                deleteConfirmMessage={`Delete service "${data.service.nickname}"? This cannot be undone.`}
              />
            </div>
          </DetailHeader>

          {/* ── Tab bar ── */}
          <TabBar
            tabs={[
              ...VIEW_TABS,
              ...(grafanaEnabled
                ? [{ key: "metrics" as TabKey, label: "Metrics", icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" }]
                : []),
            ].map((tab) => ({
              key: tab.key,
              label: tab.label,
              icon: tab.icon,
              badge: tab.key === "issues" && issues.length > 0 ? issues.length : undefined,
            }))}
            activeTab={activeTab}
            onChange={(key) => setActiveTab(key as TabKey)}
          />

          {/* ── Tab content ── */}
          {activeTab === "overview" && (
            <OverviewTab service={data.service} tags={data.tags || []} responsaveis={data.responsaveis || []} t={t} />
          )}

          {activeTab === "connections" && (
            <ConnectionsTab
              dependsOnServices={dependsOnServices}
              dependentServices={dependentServices}
              linkedHosts={linkedHosts}
              linkedDns={linkedDns}
              t={t}
            />
          )}

          {activeTab === "topology" && (
            <TopologyTab
              filteredGraph={filteredGraph}
              dependsOnServices={dependsOnServices}
              dependentServices={dependentServices}
              linkedHosts={linkedHosts}
              linkedDns={linkedDns}
              t={t}
            />
          )}

          {activeTab === "credentials" && (
            <CredentialsTab
              credentials={data.credentials}
              serviceId={id}
              isAdmin={isAdmin}
              t={t}
            />
          )}

          {activeTab === "issues" && (
            <IssuesTab issues={issues} t={t} />
          )}

          {activeTab === "metrics" && grafanaEnabled && (
            <MetricsTab serviceId={id} nickname={data.service.nickname} />
          )}

          {/* ── Edit Drawer ── */}
          <Drawer
            open={showEditDrawer}
            onClose={() => setShowEditDrawer(false)}
            title={t("common.edit")}
            subHeader={formSubHeader}
          >
            <ServiceForm
              initial={data.service}
              onSubHeaderChange={setFormSubHeader}
              onSuccess={() => {
                setShowEditDrawer(false);
                queryClient.invalidateQueries({ queryKey: ["service", id] });
                queryClient.invalidateQueries({ queryKey: ["services"] });
              }}
            />
          </Drawer>
        </div>
      ) : null}
    </PageShell>
  );
}
