"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { databasesAPI, globalIssuesAPI, servicesAPI, projectsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFlag } from "@/contexts/FlagContext";
import { useEntityGraph } from "@/hooks/useEntityGraph";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import Drawer from "@/components/ui/Drawer";
import EmptyState from "@/components/ui/EmptyState";
import SectionCard from "@/components/ui/SectionCard";
import { Skeleton } from "@/components/ui/Skeleton";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import CardIndicator from "@/components/inventory/CardIndicator";
import DetailSplit from "@/components/detail/DetailSplit";
import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { servicesGroup, projectsGroup } from "@/components/detail/RelationsCard";
import IssuesBoard from "@/components/issues/IssuesBoard";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useApiLinkNames } from "@/app/atlas/apis/_components/apiDisplay";
import DatabaseForm from "../DatabaseForm";
import { ENVIRONMENT_COLOR, SENSITIVITY_COLOR, endpointLabel, lacksBackup } from "../_components/databaseInsights";
import DatabaseProfile from "./_components/DatabaseProfile";

type TabKey = "overview" | "issues" | "topology";

export default function DatabaseDetail({ id }: { id: number }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const flag = useFlag();
  const qc = useQueryClient();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";
  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [showEdit, setShowEdit] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);

  // No retry: a 404 (missing or not visible) should say so at once.
  const { data: db, isLoading } = useQuery({ queryKey: ["databases", id], queryFn: () => databasesAPI.get(id), retry: false });
  const { data: issues = [] } = useQuery({
    queryKey: ["issues", "database", id],
    queryFn: () => globalIssuesAPI.list({ entity_type: "database", entity_id: String(id) }),
    enabled: !!db,
  });
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list, enabled: activeTab === "topology" });
  const { data: allProjects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list, enabled: activeTab === "topology" });
  const { graph, loading: graphLoading } = useEntityGraph(db ? `database-${id}` : undefined, activeTab === "topology");

  const remove = useMutation({
    mutationFn: () => databasesAPI.remove(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["databases"] });
      qc.invalidateQueries({ queryKey: ["databases-trash"] });
      qc.invalidateQueries({ queryKey: ["database-instances"] });
      router.push("/atlas/databases");
    },
    onError: (err) => flag({ appearance: "error", title: t("common.delete"), description: err instanceof Error ? err.message : t("form.saveFailed") }),
  });

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
  if (!db) {
    return (
      <PageShell>
        <EmptyState icon="box" title={t("atlas.databases.notFound")} description={t("dns.notFoundDesc")}
          action={<Button size="sm" variant="secondary" onClick={() => router.push("/atlas/databases")}>{t("atlas.databases.backToList")}</Button>} />
      </PageShell>
    );
  }

  const openIssues = issues.filter((i) => !i.archived && i.status !== "done").length;
  const consumers = db.consumer_service_ids ?? [];
  const projects = db.project_ids ?? [];
  const tabs = [
    { key: "overview", label: t("host.tabOverview"), icon: ICON_PATHS.home },
    { key: "issues", label: t("host.tabTracking"), icon: ICON_PATHS.alert, badge: openIssues || undefined },
    { key: "topology", label: t("host.tabTopology"), icon: ICON_PATHS.bolt },
  ];
  const relationGroups = [
    servicesGroup(allServices.filter((s) => consumers.includes(s.id) || s.id === db.instance_service_id), t),
    projectsGroup(allProjects.filter((p) => projects.includes(p.id)), t),
  ];

  return (
    <PageShell>
      <div className="space-y-5">
        <PageHeader
          title={db.name}
          subtitle={[db.db_name, endpointLabel(db)].filter(Boolean).join(" · ") || undefined}
          subtitleFont="mono"
          status={
            <span className="inline-flex flex-wrap items-center gap-2">
              <Badge color="cyan">{t(`atlas.databases.engine.${db.engine}`)}</Badge>
              <Badge color={ENVIRONMENT_COLOR[db.environment]}>{t(`atlas.databases.environment.${db.environment}`)}</Badge>
              <Badge color={SENSITIVITY_COLOR[db.sensitivity]}>{t(`atlas.databases.sensitivity.${db.sensitivity}`)}</Badge>
              {db.instance_service_id == null && <span className="text-xs text-[var(--text-muted)]">{db.external_provider || t("atlas.databases.external")}</span>}
              {lacksBackup(db) && <Badge color="amber">{t("atlas.databases.kpi.prodNoBackup")}</Badge>}
            </span>
          }
          indicators={<>
            <CardIndicator icon={ICON_PATHS.link} count={consumers.length} color="accent" title={t("atlas.databases.consumers")} />
            <CardIndicator icon={ICON_PATHS.folder} count={projects.length} color="accent" title={t("topology.projects")} />
            <CardIndicator icon={ICON_PATHS.keyOutline} count={db.has_credential ? 1 : 0} color="success" hideCount
              title={db.has_credential ? t("atlas.databases.hasCredential") : t("atlas.databases.noCredential")} />
            <CardIndicator icon={ICON_PATHS.clipboard} count={openIssues} color="accent" title={`${openIssues} ${t("nav.issues").toLowerCase()}`} />
          </>}
          onEdit={canEdit ? () => setShowEdit(true) : undefined}
          onDelete={isAdmin ? () => remove.mutate() : undefined}
          deleteConfirmMessage={`${t("confirm.deleteTitle", { name: `"${db.name}"` })} ${t("atlas.databases.deleteHint")}`}
          tabs={{ idBase: "database", label: db.name, active: activeTab, onChange: (k) => setActiveTab(k as TabKey), panelClassName: "space-y-5", items: tabs }}
        >
          <SectionCard as="h2" title={t("atlas.databases.about")} collapsible defaultOpen={false}
            empty={db.description?.trim() || db.use_cases?.trim() ? undefined : t("atlas.databases.noDescription")}>
            <div className="space-y-4">
              {db.description?.trim() && <MarkdownContent content={db.description} />}
              {db.use_cases?.trim() && (
                <div>
                  <p className="text-xs font-medium text-[var(--text-secondary)] mb-1">{t("atlas.apis.useCases")}</p>
                  <MarkdownContent content={db.use_cases} />
                </div>
              )}
            </div>
          </SectionCard>

          {activeTab === "overview" && (
            <DetailSplit profile={<DatabaseProfile db={db} onEditResponsaveis={canEdit ? () => setShowEdit(true) : undefined} />}>
              <ConsumerLinks ids={consumers} empty={t("atlas.databases.noConsumers")} />
            </DetailSplit>
          )}

          {activeTab === "issues" && <IssuesBoard entityType="database" entityId={id} canEdit={canEdit} />}

          {activeTab === "topology" && (
            <TopologyPane graph={graph} loading={graphLoading} t={t}
              hasRelations={relationGroups.some((g) => g.rows.length > 0)}
              relations={<RelationsCard groups={relationGroups} t={t} />} />
          )}
        </PageHeader>

        <Drawer open={showEdit} onClose={() => setShowEdit(false)} title={t("form.editTitle", { name: db.name })} subHeader={formSubHeader} footer={formFooter}>
          <DatabaseForm
            initial={db}
            onClose={() => setShowEdit(false)}
            onSubHeaderChange={setFormSubHeader}
            onFooterChange={setFormFooter}
            onSuccess={() => {
              setShowEdit(false);
              qc.invalidateQueries({ queryKey: ["databases"] });
              qc.invalidateQueries({ queryKey: ["database-instances"] });
              qc.invalidateQueries({ queryKey: ["graph"] });
              qc.invalidateQueries({ queryKey: ["relations"] });
            }}
          />
        </Drawer>
      </div>
    </PageShell>
  );
}

/** The services that use the database, each opening its page. */
function ConsumerLinks({ ids, empty }: { ids: number[]; empty: string }) {
  const { t } = useLocale();
  const names = useApiLinkNames();
  return (
    <SectionCard as="h3" title={t("atlas.databases.consumers")} count={ids.length} empty={ids.length ? undefined : empty}>
      <div className="flex flex-wrap gap-x-3 gap-y-1">
        {ids.map((id) => (
          <Link key={id} href={`/services/${id}`} className="text-sm text-[var(--accent)] hover:underline">{names.service.get(id) ?? `#${id}`}</Link>
        ))}
      </div>
    </SectionCard>
  );
}
