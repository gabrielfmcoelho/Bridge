"use client";

import { useMemo } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { appDepsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { usePageTab } from "@/hooks/usePageTab";
import { useEntityGraph } from "@/hooks/useEntityGraph";
import { useAppDepLinks } from "@/hooks/useAppDepLinks";
import { buildCiGraph, endpoint, indexByCi, linkRows } from "@/lib/appDeps";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import SectionCard from "@/components/ui/SectionCard";
import StatusAlert from "@/components/ui/StatusAlert";
import EmptyState from "@/components/ui/EmptyState";
import Icon from "@/components/ui/Icon";
import { tableClasses } from "@/components/ui/Table";
import { Skeleton } from "@/components/ui/Skeleton";
import AppDepsGraph from "@/components/graph/AppDepsGraph";
import TopologyPane from "@/components/detail/TopologyPane";
import RelationsCard, { dnsGroup, hostsGroup, projectsGroup, servicesGroup } from "@/components/detail/RelationsCard";
import AppDepLinksButton from "@/components/appdeps/AppDepLinksManager";
import { useAppDepAssets } from "@/components/appdeps/useAppDepAssets";
import AppDepLinksTable from "../_components/AppDepLinksTable";
import CriticalityBadge from "../_components/CriticalityBadge";

const dash = <span className="text-[var(--text-faint)]">–</span>;

const TABS = ["topology"] as const;

/** One application (CI). Dependências: its diagram — components and the CIs it
 *  links to — then the links and the components as tables. Topologia: the
 *  hosts, services, DNS records and projetos it is linked to in Bridge. */
export default function AppDepDetail({ ci }: { ci: string }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [tab, selectTab] = usePageTab(TABS);
  const onTopology = tab === "topology";
  const { data, isLoading, error } = useQuery({ queryKey: ["app-deps"], queryFn: appDepsAPI.list });
  const index = useMemo(() => indexByCi(data ?? []), [data]);
  const entry = index.byCi.get(ci);
  const graph = useMemo(() => buildCiGraph(ci, index), [ci, index]);
  const links = useMemo(() => (entry ? linkRows(entry.catalog, index) : []), [entry, index]);
  const { graph: topo, loading: topoLoading } = useEntityGraph(entry ? `app-${entry.source.gitlab_project_id}` : undefined, onTopology);
  const { links: assetLinks } = useAppDepLinks({ ci }, onTopology);
  const assets = useAppDepAssets(onTopology);

  if (error) {
    return <PageShell><StatusAlert variant="error">{(error as Error).message || t("appDeps.loadError")}</StatusAlert></PageShell>;
  }
  if (isLoading) {
    return <PageShell><Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" /></PageShell>;
  }
  if (!entry || !graph) {
    return (
      <PageShell>
        <EmptyState
          icon="search"
          title={t("appDeps.detail.notFound", { ci })}
          description={t("appDeps.detail.notFoundHint")}
          action={<Link href="/atlas/app-deps" className="text-sm font-medium text-[var(--accent)] hover:underline">{t("appDeps.detail.back")}</Link>}
        />
      </PageShell>
    );
  }

  const { catalog: c, source } = entry;
  const dup = index.duplicates.get(ci);
  const th = tableClasses.compact.th;
  const td = `${tableClasses.compact.td} align-top`;
  return (
    <PageShell>
      <PageHeader
        title={c.ci}
        titleFont="mono"
        description={c.description}
        showEmptyDescription
        indicators={
          <span className="text-xs text-[var(--text-muted)]">
            {[
              c.entity.map((e) => e.name || e.slug).join(", "),
              c.owners.length ? t("appDeps.detail.owners", { owners: c.owners.map((o) => o.name || o.username).join(", ") }) : "",
            ].filter(Boolean).join(" · ") || "–"}
          </span>
        }
        actions={
          <a
            href={source.file_url || source.web_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 h-8 px-3 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-elevated)] text-xs font-mono text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--border-strong)]"
          >
            <Icon path={ICON_PATHS.branch} className="w-3.5 h-3.5" />
            {source.project_path}@{source.ref}
            <Icon path={ICON_PATHS.externalLink} className="w-3 h-3" />
          </a>
        }
        tabs={{
          idBase: "app-dep",
          label: c.ci,
          active: onTopology ? "topology" : "overview",
          onChange: selectTab,
          panelClassName: "space-y-4",
          items: [
            { key: "overview", label: t("appDeps.detail.tabDependencies"), icon: ICON_PATHS.branch },
            { key: "topology", label: t("appDeps.detail.tabTopology"), icon: ICON_PATHS.servers },
          ],
        }}
      >
      {onTopology ? (() => {
        const ids = (type: string) => new Set(assetLinks.filter((l) => l.entity_type === type).map((l) => l.entity_id));
        const [h, s, d, p] = [ids("host"), ids("service"), ids("dns"), ids("project")];
        const groups = [
          hostsGroup(assets.hosts.filter((x) => h.has(x.id)), t),
          servicesGroup(assets.services.filter((x) => s.has(x.id)), t),
          dnsGroup(assets.dns.filter((x) => d.has(x.id)), t),
          projectsGroup(assets.projects.filter((x) => p.has(x.id)), t),
        ];
        // A CI linked to nothing is no graph node: show no graph rather than the whole map,
        // but keep the card (and its link manager) for editors.
        const inGraph = topo.nodes.some((n) => n.id === topo.focus);
        return (
          <TopologyPane graph={inGraph ? topo : { nodes: [], edges: [] }} loading={topoLoading || assets.loading} t={t}
            hasRelations={canEdit || groups.some((g) => g.rows.length > 0)}
            relations={<RelationsCard groups={groups} t={t} controls={canEdit ? <AppDepLinksButton ci={c.ci} /> : undefined} />} />
        );
      })() : (
      <>
        {dup && (
          <StatusAlert variant="warning">
            {t("appDeps.detail.duplicate", { repo: source.project_path, others: dup.map((s) => s.project_path).join(", ") })}
          </StatusAlert>
        )}
        <AppDepsGraph graph={graph} />
        <SectionCard title={t("appDeps.detail.links")} count={links.length} body="padded">
          <AppDepLinksTable rows={links} />
        </SectionCard>
        <SectionCard title={t("appDeps.detail.components")} count={c.components.length} body="padded" empty={c.components.length ? undefined : t("appDeps.detail.noComponents")}>
          {c.components.length > 0 && (
            <div className="overflow-x-auto">
              <table className={tableClasses.compact.table}>
                <thead>
                  <tr className={tableClasses.compact.headRow}>
                    <th className={th}>{t("appDeps.detail.component")}</th>
                    <th className={th}>{t("appDeps.detail.category")}</th>
                    <th className={th}>{t("appDeps.links.technology")}</th>
                    <th className={th}>{t("appDeps.links.endpoint")}</th>
                    <th className={th}>{t("appDeps.links.criticality")}</th>
                    <th className={th}>{t("appDeps.links.description")}</th>
                  </tr>
                </thead>
                <tbody>
                  {c.components.map((x) => (
                    <tr key={x.name} className={tableClasses.compact.row}>
                      <td className={`${td} font-mono text-xs font-semibold`}>{x.name}</td>
                      <td className={`${td} text-xs`}>{x.category || dash}</td>
                      <td className={`${td} font-mono text-xs`}>{x.technology || dash}</td>
                      <td className={`${td} font-mono text-xs`}>{endpoint(x.protocol, x.port) || dash}</td>
                      <td className={td}><CriticalityBadge value={x.criticality} /></td>
                      <td className={`${td} text-xs text-[var(--text-secondary)]`}>{x.description || dash}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </SectionCard>
      </>
      )}
      </PageHeader>
    </PageShell>
  );
}
