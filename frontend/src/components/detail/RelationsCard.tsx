"use client";

import type { ReactNode } from "react";
import SectionCard from "@/components/ui/SectionCard";
import SituacaoText from "@/components/ui/SituacaoText";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import { RowGroup, ListRow } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";

type T = (k: string) => string;

export interface Relation {
  key: string | number;
  href?: string;
  icon: string;
  /** CSS colour for the icon, a token: "var(--accent)". */
  tone: string;
  label: string;
  mono?: boolean;
  trailing?: ReactNode;
}
export interface RelationGroup { title: string; rows: Relation[] }

/** One card, grouped flush rows: what an asset is linked to. Empty groups drop out. */
export default function RelationsCard({ groups, t }: { groups: RelationGroup[]; t: T }) {
  const shown = groups.filter((g) => g.rows.length > 0);
  const count = shown.reduce((n, g) => n + g.rows.length, 0);
  return (
    <SectionCard as="h3" title={t("topology.relations")} count={count} body="flush" empty={count === 0 ? t("host.noTopologyDesc") : undefined}>
      {shown.map((g) => (
        <RowGroup key={g.title} title={g.title}>
          {g.rows.map((r) => (
            <ListRow key={r.key} href={r.href}>
              <Icon path={r.icon} className="w-3.5 h-3.5 shrink-0" style={{ color: r.tone }} />
              <span className={`text-sm text-[var(--text-primary)] truncate flex-1 ${r.mono ? "font-mono" : ""}`}>{r.label}</span>
              {r.trailing}
            </ListRow>
          ))}
        </RowGroup>
      ))}
    </SectionCard>
  );
}

/* Group builders, so every detail page lists the same kind the same way. */

export const hostsGroup = (hosts: { id: number; nickname: string; oficial_slug?: string; situacao?: string }[], t: T): RelationGroup => ({
  title: t("nav.hosts"),
  rows: hosts.map((h) => ({
    key: `h${h.id}`, href: h.oficial_slug ? `/hosts/${h.oficial_slug}` : undefined, icon: ICON_PATHS.server, tone: "var(--info)",
    label: h.nickname, trailing: h.situacao ? <SituacaoText situacao={h.situacao} /> : undefined,
  })),
});

export const servicesGroup = (services: { id: number; nickname: string; technology_stack?: string }[], t: T): RelationGroup => ({
  title: t("topology.services"),
  rows: services.map((s) => ({
    key: `s${s.id}`, href: `/services/${s.id}`, icon: ICON_PATHS.serverStack, tone: "var(--accent)",
    label: s.nickname, trailing: s.technology_stack ? <Badge>{s.technology_stack}</Badge> : undefined,
  })),
});

export const dnsGroup = (records: { id: number; domain: string; has_https?: boolean; situacao?: string }[], t: T): RelationGroup => ({
  title: t("topology.dnsRecords"),
  rows: records.map((d) => ({
    key: `d${d.id}`, href: `/dns/${d.id}`, icon: ICON_PATHS.globeMeridian, tone: "var(--success)", label: d.domain, mono: true,
    trailing: (
      <>
        <span className={d.has_https ? "text-[var(--success)]" : "text-[var(--text-faint)]/40"} title={d.has_https ? t("topology.https") : t("topology.noHttps")}>
          <Icon path={ICON_PATHS.lock} className="w-3.5 h-3.5" />
        </span>
        {d.situacao && <SituacaoText situacao={d.situacao} />}
      </>
    ),
  })),
});

export const projectsGroup = (projects: { id: number; name: string; situacao?: string }[], t: T): RelationGroup => ({
  title: t("topology.projects"),
  rows: projects.map((p) => ({
    key: `p${p.id}`, href: `/projects/${p.id}`, icon: ICON_PATHS.folder, tone: "var(--warning)",
    label: p.name, trailing: p.situacao ? <SituacaoText situacao={p.situacao} /> : undefined,
  })),
});
