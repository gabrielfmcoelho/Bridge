"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiCatalogAPI, apiKeysAPI, globalIssuesAPI, servicesAPI, projectsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFlag } from "@/contexts/FlagContext";
import { useEntityGraph } from "@/hooks/useEntityGraph";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Button from "@/components/ui/Button";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import Badge from "@/components/ui/Badge";
import Drawer from "@/components/ui/Drawer";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import DropdownMenu, { DropdownMenuGroup, DropdownMenuItem } from "@/components/ui/DropdownMenu";
import CardIndicator from "@/components/inventory/CardIndicator";
import DetailSplit from "@/components/detail/DetailSplit";
import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { servicesGroup, projectsGroup } from "@/components/detail/RelationsCard";
import IssuesBoard from "@/components/issues/IssuesBoard";
import ApiReference from "@/components/atlas/apis/ApiReference";
import ShareBundleModal from "@/components/atlas/apis/ShareBundleModal";
import { ICON_PATHS } from "@/lib/icon-paths";
import ApiForm from "../ApiForm";
import { useSpecActions } from "../_components/useSpecActions";
import { isSpecStale, ORIGEM_COLOR } from "../_components/apiInsights";
import { useApiLinkNames } from "../_components/apiDisplay";
import Link from "next/link";
import ApiProfile from "./_components/ApiProfile";
import SectionCard from "@/components/ui/SectionCard";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import ApiKeysTab from "./_components/ApiKeysTab";
import RequestsTab, { useTelemetryEnabled } from "@/components/telemetry/RequestsTab";

type TabKey = "overview" | "endpoints" | "issues" | "topology" | "keys" | "requests";

export default function ApiDetail({ id }: { id: number }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const flag = useFlag();
  const qc = useQueryClient();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";
  const canManageKeys = isAdmin || !!user?.permissions?.includes("apis.keys.manage");

  const searchParams = useSearchParams();
  const op = searchParams.get("op");
  // An endpoint-search hit (?op=) lands on the reference.
  const [activeTab, setActiveTab] = useState<TabKey>(op ? "endpoints" : "overview");
  const telemetryEnabled = useTelemetryEnabled();
  const [sharing, setSharing] = useState(false);
  const [showEditDrawer, setShowEditDrawer] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);

  // No retry: a 404 (missing or not visible) should say so at once.
  const { data: api, isLoading } = useQuery({ queryKey: ["api-catalog", id], queryFn: () => apiCatalogAPI.get(id), retry: false });
  const { data: spec } = useQuery({
    queryKey: ["api-catalog", id, "spec"],
    queryFn: () => apiCatalogAPI.getSpec(id),
    enabled: !!api && api.source_type !== "manual" && activeTab === "endpoints",
  });
  const { data: issues = [] } = useQuery({
    queryKey: ["issues", "api_catalog", id],
    queryFn: () => globalIssuesAPI.list({ entity_type: "api_catalog", entity_id: String(id) }),
    enabled: !!api,
  });
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list, enabled: activeTab === "topology" });
  const { data: allProjects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list, enabled: activeTab === "topology" });
  const { graph, loading: graphLoading } = useEntityGraph(api ? `api-${id}` : undefined, activeTab === "topology");
  const specActions = useSpecActions(api);
  // Same query key as the keys tab, so the badge and the list share one fetch.
  const { data: liveKeys = [] } = useQuery({
    queryKey: ["api-keys", id, false],
    queryFn: () => apiKeysAPI.list(id, false),
    enabled: !!api && api.key_management !== "none",
  });

  // Best-effort deep-jump to ?op=<op_key>: Scalar owns its DOM and its anchor
  // scheme is version-dependent, so scan for the path text after mount and
  // scroll the first match into view. No match → no-op.
  useEffect(() => {
    if (!op || !spec || !api || activeTab !== "endpoints") return;
    const path = api.operations?.find((o) => o.op_key === op)?.path;
    if (!path) return;
    const timer = setTimeout(() => {
      try {
        const root = document.querySelector(".scalar-api-reference") || document;
        const nodes = Array.from(root.querySelectorAll("h1, h2, h3, a, [class*='endpoint'], [class*='operation']"));
        const hit = nodes.find((n) => n.textContent?.includes(path)) as HTMLElement | undefined;
        hit?.scrollIntoView({ behavior: "smooth", block: "start" });
      } catch {
        /* best-effort only */
      }
    }, 800);
    return () => clearTimeout(timer);
  }, [op, spec, api, activeTab]);

  const remove = useMutation({
    mutationFn: () => apiCatalogAPI.remove(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-catalog"] });
      qc.invalidateQueries({ queryKey: ["api-catalog-trash"] });
      router.push("/atlas/apis");
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
  if (!api) {
    return (
      <PageShell>
        <EmptyState icon="box" title={t("atlas.apis.notFound")} description={t("dns.notFoundDesc")}
          action={<Button size="sm" variant="secondary" onClick={() => router.push("/atlas/apis")}>{t("atlas.apis.backToList")}</Button>} />
      </PageShell>
    );
  }

  const openIssues = issues.filter((i) => !i.archived && i.status !== "done").length;
  const services = api.service_ids ?? [];
  const consumers = api.consumer_service_ids ?? [];
  const projects = api.project_ids ?? [];
  const origem = api.origem ?? "propria";
  const manual = api.source_type === "manual";
  const stale = isSpecStale(api);
  const tabs = [
    { key: "overview", label: t("host.tabOverview"), icon: ICON_PATHS.home },
    { key: "endpoints", label: t("atlas.apis.tabEndpoints"), icon: ICON_PATHS.code, badge: api.operation_count || undefined },
    { key: "issues", label: t("host.tabTracking"), icon: ICON_PATHS.alert, badge: openIssues || undefined },
    { key: "topology", label: t("host.tabTopology"), icon: ICON_PATHS.bolt },
    ...(telemetryEnabled ? [{ key: "requests", label: t("telemetry.tab"), icon: ICON_PATHS.clock }] : []),
    // Entity-specific tab last. Admins see it even with key management off,
    // to switch it on.
    ...(api.key_management !== "none" || isAdmin
      ? [{ key: "keys", label: t("atlas.apis.keys.tab"), icon: ICON_PATHS.keyOutline, badge: liveKeys.filter((k) => k.status === "active").length || undefined }]
      : []),
  ];
  const relationGroups = [
    servicesGroup(allServices.filter((s) => services.includes(s.id) || consumers.includes(s.id)), t),
    projectsGroup(allProjects.filter((p) => projects.includes(p.id)), t),
  ];
  const open = (url: string) => window.open(url, "_blank", "noopener,noreferrer");

  return (
    <PageShell>
      <div className="space-y-5">
        <PageHeader
          title={api.name}
          subtitle={manual ? t("atlas.apis.noSpec") : [api.title, api.version_label].filter(Boolean).join(" · ") || undefined}
          subtitleFont="display"
          status={
            <span className="inline-flex items-center gap-2">
              <Badge color={ORIGEM_COLOR[origem]}>{t(`atlas.apis.origem.${origem}`)}</Badge>
              {api.fornecedor && <span className="text-xs text-[var(--text-muted)]">{api.fornecedor}</span>}
              {!manual && <Badge color="cyan">{api.spec_version ? `OpenAPI ${api.spec_version}` : "–"}</Badge>}
              <span className="text-xs text-[var(--text-muted)]">{t(`atlas.apis.sourceType.${api.source_type}`)}</span>
              {stale && <Badge color="amber">{t("atlas.apis.kpi.specStale")}</Badge>}
            </span>
          }
          indicators={<>
            <CardIndicator icon={ICON_PATHS.code} count={api.operation_count} color="info" title={t("atlas.apis.endpointsCount", { count: String(api.operation_count) })} />
            <CardIndicator icon={ICON_PATHS.serverStack} count={services.length} color="warning" title={t("atlas.apis.servedBy")} />
            <CardIndicator icon={ICON_PATHS.link} count={consumers.length} color="accent" title={t("atlas.apis.consumedBy")} />
            <CardIndicator icon={ICON_PATHS.folder} count={projects.length} color="accent" title={t("topology.projects")} />
            <CardIndicator icon={ICON_PATHS.clipboard} count={openIssues} color="accent" title={`${openIssues} ${t("nav.issues").toLowerCase()}`} />
          </>}
          actions={
            <>
              <Button size="sm" variant="secondary" onClick={() => setSharing(true)}>{t("atlas.apis.share")}</Button>
              <DropdownMenu trigger={<IconButton label={t("atlas.apis.moreActions")}><Icon path={ICON_PATHS.moreHorizontal} /></IconButton>}>
                {canEdit && (
                  <DropdownMenuGroup title={t("atlas.apis.specGroup")}>
                    {api.source_type === "url" && (
                      <DropdownMenuItem disabled={specActions.pending} onClick={() => specActions.refetch.mutate()}
                        elemBefore={<Icon path={ICON_PATHS.refresh} className="w-4 h-4" />}>{t("atlas.apis.refetch")}</DropdownMenuItem>
                    )}
                    <DropdownMenuItem disabled={specActions.pending} onClick={specActions.pickFile}
                      elemBefore={<Icon path={ICON_PATHS.upload} className="w-4 h-4" />}>{t("atlas.apis.replaceSpec")}</DropdownMenuItem>
                  </DropdownMenuGroup>
                )}
                <DropdownMenuGroup title={t("atlas.apis.openGroup")}>
                  <DropdownMenuItem disabled={!api.docs_url} onClick={() => api.docs_url && open(api.docs_url)}
                    elemBefore={<Icon path={ICON_PATHS.externalLink} className="w-4 h-4" />}>{t("atlas.apis.openDocs")}</DropdownMenuItem>
                  <DropdownMenuItem disabled={!api.base_url} onClick={() => api.base_url && open(api.base_url)}
                    elemBefore={<Icon path={ICON_PATHS.externalLink} className="w-4 h-4" />}>{t("atlas.apis.openBaseUrl")}</DropdownMenuItem>
                  {(api.urls ?? []).map((u) => (
                    <DropdownMenuItem key={u.url} onClick={() => open(u.url)}
                      elemBefore={<Icon path={ICON_PATHS.externalLink} className="w-4 h-4" />}>{t("atlas.apis.openUrl", { label: u.label || u.url })}</DropdownMenuItem>
                  ))}
                </DropdownMenuGroup>
              </DropdownMenu>
              {specActions.fileInput}
            </>
          }
          onEdit={canEdit ? () => setShowEditDrawer(true) : undefined}
          onDelete={isAdmin ? () => remove.mutate() : undefined}
          deleteConfirmMessage={`${t("confirm.deleteTitle", { name: `"${api.name}"` })} ${t("atlas.apis.deleteHint")}`}
          tabs={{ idBase: "api", label: api.name, active: activeTab, onChange: (k) => setActiveTab(k as TabKey), panelClassName: "space-y-5", items: tabs }}
        >
          <SectionCard as="h2" title={t("atlas.apis.aboutToggle")} collapsible defaultOpen={false}
            empty={api.description?.trim() || api.use_cases?.trim() ? undefined : t("atlas.apis.noDescription")}>
            <div className="space-y-4">
              {api.description?.trim() && <MarkdownContent content={api.description} />}
              {api.use_cases?.trim() && (
                <div>
                  <p className="text-xs font-medium text-[var(--text-secondary)] mb-1">{t("atlas.apis.useCases")}</p>
                  <MarkdownContent content={api.use_cases} />
                </div>
              )}
            </div>
          </SectionCard>

          {activeTab === "overview" && (
            <DetailSplit profile={<ApiProfile api={api} onEditResponsaveis={canEdit ? () => setShowEditDrawer(true) : undefined} />}>
              {origem !== "externa" && (
                <ServiceLinks title={t("atlas.apis.servedBy")} ids={services} empty={t("atlas.apis.noServedBy")} />
              )}
              <ServiceLinks title={t("atlas.apis.consumedBy")} ids={consumers} empty={t("atlas.apis.noConsumers")} />
            </DetailSplit>
          )}

          {activeTab === "endpoints" && manual && (
            <EmptyState icon="box" title={t("atlas.apis.noSpec")} description={t("atlas.apis.noSpecDesc")}
              action={api.docs_url ? <Button size="sm" variant="secondary" onClick={() => open(api.docs_url!)}>{t("atlas.apis.openDocs")}</Button> : undefined} />
          )}
          {activeTab === "endpoints" && !manual && (
            <div className="min-h-[70vh]">
              {spec ? <ApiReference content={spec} servers={[
                ...(api.base_url ? [{ url: api.base_url, description: t("atlas.apis.primaryUrl") }] : []),
                ...(api.urls ?? []).map((u) => ({ url: u.url, description: u.label || undefined })),
              ]} /> : <Skeleton className="h-[60vh] w-full rounded-[var(--radius-md)]" />}
            </div>
          )}

          {activeTab === "issues" && <IssuesBoard entityType="api_catalog" entityId={id} canEdit={canEdit} />}

          {activeTab === "requests" && telemetryEnabled && <RequestsTab kind="api" id={id} />}
          {activeTab === "keys" && <ApiKeysTab api={api} canManage={canManageKeys} isAdmin={isAdmin} />}

          {activeTab === "topology" && (
            <TopologyPane graph={graph} loading={graphLoading} t={t}
              hasRelations={relationGroups.some((g) => g.rows.length > 0)}
              relations={<RelationsCard groups={relationGroups} t={t} />} />
          )}
        </PageHeader>

        <Drawer open={showEditDrawer} onClose={() => setShowEditDrawer(false)} title={t("form.editTitle", { name: api.name })} subHeader={formSubHeader} footer={formFooter}>
          <ApiForm
            initial={api}
            onClose={() => setShowEditDrawer(false)}
            onSubHeaderChange={setFormSubHeader}
            onFooterChange={setFormFooter}
            onSuccess={() => {
              setShowEditDrawer(false);
              qc.invalidateQueries({ queryKey: ["api-catalog"] });
              qc.invalidateQueries({ queryKey: ["graph"] });
              qc.invalidateQueries({ queryKey: ["relations"] });
            }}
          />
        </Drawer>
        <ShareBundleModal open={sharing} onClose={() => setSharing(false)} api={api} />
      </div>
    </PageShell>
  );
}

/** A list of linked services (served by / consumed by), each opening its page. */
function ServiceLinks({ title, ids, empty }: { title: string; ids: number[]; empty: string }) {
  const names = useApiLinkNames();
  return (
    <SectionCard as="h3" title={title} count={ids.length} empty={ids.length ? undefined : empty}>
      <div className="flex flex-wrap gap-x-3 gap-y-1">
        {ids.map((id) => (
          <Link key={id} href={`/services/${id}`} className="text-sm text-[var(--accent)] hover:underline">{names.service.get(id) ?? `#${id}`}</Link>
        ))}
      </div>
    </SectionCard>
  );
}
