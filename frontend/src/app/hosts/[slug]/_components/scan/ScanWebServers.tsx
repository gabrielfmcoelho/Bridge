"use client";

import Link from "next/link";
import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import type { KongNode, VMInfoType, WebServer, WebSite } from "@/lib/api";
import { linkedIndex, nameStatus, poolUsers, undeclaredRecords } from "@/lib/webDns";
import { siteFacts } from "@/lib/scanSections";
import { TrafficDetails, TrafficHeadline } from "./ScanTraffic";
import { CodeChips, CodeDetails } from "./ScanCode";
import { Detail, Row, Rows, SubBlock, type T } from "./shared";

const KIND_LABEL: Record<WebServer["kind"], string> = { nginx: "nginx", apache: "Apache", "php-fpm": "PHP-FPM", kong: "Kong" };

/**
 * nginx / Apache / PHP-FPM found by the scan — on the host or in a container —
 * with the sites each serves. With the host's DNS records, each name shows
 * whether it is linked (the scan links existing records and creates new ones
 * that resolve; service.RecordWebDNS). Without them (Operations console) the
 * names are plain.
 */
export default function ScanWebServers({ info, t, dnsRecords }: {
  info: VMInfoType; t: T; dnsRecords?: { id: number; domain: string; inbound?: boolean }[];
}) {
  const servers = info.web_servers ?? [];
  const facts = siteFacts(info).bySite;
  const linked = dnsRecords ? linkedIndex(dnsRecords) : null;
  // A record another host's proxy routes here (inbound) is explained on the Topology tab.
  const undeclared = dnsRecords && servers.length > 0 ? undeclaredRecords(dnsRecords.filter((r) => !r.inbound), servers) : [];

  return (
    <SectionCard as="h3" title={t("scan.pane.group.webservers")} empty={servers.length === 0 ? t("scan.pane.empty.webservers") : undefined}>
      {servers.length > 0 && <p className="text-2xs text-[var(--text-muted)] mb-3">{t("scan.web.summary")}</p>}
      {servers.map((s, i) => <ServerBlock key={i} server={s} servers={servers} facts={facts} linked={linked} t={t} />)}
      {undeclared.length > 0 && (
        <SubBlock title={t("scan.web.undeclared")}>
          <div className="flex flex-wrap gap-1.5">
            {undeclared.map((r) => (
              <Link key={r.id} href={`/dns/${r.id}`}><Badge color="gray" compact>{r.domain}</Badge></Link>
            ))}
          </div>
        </SubBlock>
      )}
    </SectionCard>
  );
}

type SiteFacts = ReturnType<typeof siteFacts>["bySite"];

function ServerBlock({ server: s, servers, facts, linked, t }: {
  server: WebServer; servers: WebServer[]; facts: SiteFacts; linked: Map<string, number> | null; t: T;
}) {
  const where = s.container ? t("scan.web.inContainer", { name: s.container }) : t("scan.web.onHost");
  return (
    <SubBlock title={`${KIND_LABEL[s.kind] ?? s.kind} · ${where}`} aside={s.stopped ? t("scan.web.stopped") : s.container_image}>
      {s.error && <p className="text-xs text-[var(--warning)] mb-2 break-all">{s.error}</p>}
      {s.kong && !s.error && <Rows><KongRow k={s.kong} t={t} /></Rows>}
      {(s.sites ?? []).length > 0 && (
        <Rows>{s.sites!.map((site, i) => <SiteRow key={i} site={site} facts={facts.get(site)} linked={linked} muted={s.stopped} t={t} />)}</Rows>
      )}
      {(s.pools ?? []).length > 0 && (
        <Rows>
          {s.pools!.map((p, i) => {
            const users = poolUsers(p, servers);
            return (
              <Row
                key={i}
                muted={s.stopped}
                summary={<>
                  <span className="font-mono text-[var(--text-primary)]">{p.name}</span>
                  <span className="ml-auto shrink-0 font-mono text-[var(--text-muted)]">{p.listen}</span>
                </>}
              >
                {users.length > 0 ? <Detail label={t("scan.web.usedBy")}>{users.join(", ")}</Detail> : undefined}
              </Row>
            );
          })}
        </Rows>
      )}
    </SubBlock>
  );
}

const TARGET_COLOR: Record<string, "success" | "danger"> = { HEALTHY: "success", UNHEALTHY: "danger", DNS_ERROR: "danger" };

/** A Kong node's status: version, database, routes; expands to traffic, listeners and upstream target health. */
function KongRow({ k, t }: { k: KongNode; t: T }) {
  const db = k.database && k.database !== "off" ? `${k.database} ${k.db_host}:${k.db_port}` : t("scan.web.kongDbLess");
  const targets = (k.upstreams ?? []).flatMap((u) => u.targets ?? []);
  const failing = targets.filter((x) => TARGET_COLOR[x.health ?? ""] === "danger").length;
  return (
    <Row
      summary={<>
        <span className="font-mono text-[var(--text-primary)] shrink-0">Kong {k.version}</span>
        <span className="font-mono text-[var(--text-muted)] truncate" title={db}>{db}</span>
        {k.db_reachable === false && <Badge color="danger" compact className="shrink-0 whitespace-nowrap">{t("scan.web.kongDbUnreachable")}</Badge>}
        {failing > 0 && <Badge color="danger" compact className="shrink-0 whitespace-nowrap">{t("scan.web.kongFailingTargets", { count: String(failing) })}</Badge>}
        <span className="ml-auto shrink-0 text-[var(--text-muted)]">{t("scan.web.kongCounts", { services: String(k.services), routes: String(k.routes) })}</span>
      </>}
    >
      <Detail label={t("scan.web.kongTraffic")}>
        {t("scan.web.kongTrafficValue", { connections: String(k.connections_active ?? 0), requests: String(k.total_requests ?? 0) })}
      </Detail>
      {(k.proxy_ports ?? []).length > 0 && <Detail label={t("scan.web.kongProxyPorts")}>{k.proxy_ports!.join(", ")}</Detail>}
      {(k.admin_ports ?? []).length > 0 && <Detail label={t("scan.web.kongAdminPorts")}>{k.admin_ports!.join(", ")}</Detail>}
      {(k.upstreams ?? []).map((u) => (
        <Detail key={u.name} label={u.name}>
          <span className="flex flex-wrap gap-1">
            {(u.targets ?? []).map((x) => (
              <span key={x.target} title={x.health}><Badge color={TARGET_COLOR[x.health ?? ""] ?? "gray"} compact>{x.target}</Badge></span>
            ))}
          </span>
        </Detail>
      ))}
    </Row>
  );
}

/**
 * One site: its names, ports and targets, plus — from a background scan — its
 * access-log traffic and its docroot's code (lib/scanSections `siteFacts`).
 */
function SiteRow({ site, facts, linked, muted, t }: {
  site: WebSite; facts?: NonNullable<ReturnType<SiteFacts["get"]>>; linked: Map<string, number> | null; muted?: boolean; t: T;
}) {
  const names = site.names ?? [];
  const details = [
    site.locations?.length ? (
      <Detail key="l" label={t("scan.web.locations")}>
        <span className="flex flex-col gap-0.5">
          {site.locations.map((l) => (
            <span key={l.path} className="font-mono break-all">
              {l.path} → {[...(l.proxy ?? []), ...(l.fastcgi ?? [])].join(", ")}
            </span>
          ))}
        </span>
      </Detail>
    ) : null,
    !site.locations?.length && site.proxy?.length ? <Detail key="p" label={t("scan.web.proxy")}>{site.proxy.join(", ")}</Detail> : null,
    !site.locations?.length && site.fastcgi?.length ? <Detail key="f" label="FastCGI">{site.fastcgi.join(", ")}</Detail> : null,
    site.root ? <Detail key="r" label={t("scan.web.root")}>{site.root}</Detail> : null,
    site.source ? <Detail key="s" label={t("scan.web.source")}>{site.source}</Detail> : null,
    facts?.traffic ? (
      <div key="t" className="pt-2 border-t border-[var(--border-subtle)]/60 space-y-1.5">
        <p className="font-medium text-[var(--text-secondary)]">{t("scan.pane.group.traffic")}</p>
        <TrafficDetails s={facts.traffic} t={t} />
      </div>
    ) : null,
    facts?.code ? (
      <div key="c" className="pt-2 border-t border-[var(--border-subtle)]/60 space-y-1.5">
        <p className="font-medium text-[var(--text-secondary)]">{t("scan.pane.group.code")} · <span className="font-mono font-normal">{facts.code.docroot}</span></p>
        <CodeDetails d={facts.code} t={t} />
      </div>
    ) : null,
  ].filter(Boolean);
  return (
    <Row
      muted={muted}
      summary={<>
        <span className="flex flex-wrap gap-1 min-w-0">
          {names.length > 0
            ? names.map((n) => <NameChip key={n} name={n} linked={linked} t={t} />)
            : <span className="text-[var(--text-faint)]">{t("scan.web.noName")}</span>}
        </span>
        <span className="ml-auto flex shrink-0 items-center gap-1.5">
          {facts?.code && <CodeChips d={facts.code} />}
          {facts?.traffic && <span className="tabular-nums text-[var(--text-secondary)]"><TrafficHeadline s={facts.traffic} t={t} /></span>}
          <span className="font-mono text-[var(--text-muted)]">{(site.listen ?? []).join(", ")}{site.tls ? " · TLS" : ""}</span>
        </span>
      </>}
    >
      {details.length > 0 ? details : undefined}
    </Row>
  );
}

function NameChip({ name, linked, t }: { name: string; linked: Map<string, number> | null; t: T }) {
  if (!linked) return <Badge compact>{name}</Badge>;
  const st = nameStatus(name, linked);
  if (st.kind === "linked") {
    return (
      <Link href={`/dns/${st.dnsId}`} title={t("scan.web.linked")}>
        <Badge color="success" compact dot>{name}</Badge>
      </Link>
    );
  }
  if (st.kind === "unlinked") return <span title={t("scan.web.unlinked")}><Badge color="gray" compact>{name}</Badge></span>;
  return <span title={t("scan.web.ignored")} className="font-mono text-[var(--text-faint)]">{name}</span>;
}
