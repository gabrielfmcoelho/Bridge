"use client";

import Link from "next/link";
import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import type { VMInfoType, WebServer, WebSite } from "@/lib/api";
import { linkedIndex, nameStatus, poolUsers, undeclaredRecords } from "@/lib/webDns";
import { Detail, Row, Rows, SubBlock, type T } from "./shared";

const KIND_LABEL: Record<WebServer["kind"], string> = { nginx: "nginx", apache: "Apache", "php-fpm": "PHP-FPM" };

/**
 * nginx / Apache / PHP-FPM found by the scan — on the host or in a container —
 * with the sites each serves. With the host's DNS records, each name shows
 * whether it is linked (the scan links existing records and creates new ones
 * that resolve; service.RecordWebDNS). Without them (Operations console) the
 * names are plain.
 */
export default function ScanWebServers({ info, t, dnsRecords }: {
  info: VMInfoType; t: T; dnsRecords?: { id: number; domain: string }[];
}) {
  const servers = info.web_servers ?? [];
  const linked = dnsRecords ? linkedIndex(dnsRecords) : null;
  const undeclared = dnsRecords && servers.length > 0 ? undeclaredRecords(dnsRecords, servers) : [];

  return (
    <SectionCard as="h3" title={t("scan.pane.group.webservers")} empty={servers.length === 0 ? t("scan.pane.empty.webservers") : undefined}>
      {servers.length > 0 && <p className="text-2xs text-[var(--text-muted)] mb-3">{t("scan.web.summary")}</p>}
      {servers.map((s, i) => <ServerBlock key={i} server={s} servers={servers} linked={linked} t={t} />)}
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

function ServerBlock({ server: s, servers, linked, t }: {
  server: WebServer; servers: WebServer[]; linked: Map<string, number> | null; t: T;
}) {
  const where = s.container ? t("scan.web.inContainer", { name: s.container }) : t("scan.web.onHost");
  return (
    <SubBlock title={`${KIND_LABEL[s.kind] ?? s.kind} · ${where}`} aside={s.stopped ? t("scan.web.stopped") : s.container_image}>
      {s.error && <p className="text-xs text-[var(--warning)] mb-2 break-all">{s.error}</p>}
      {(s.sites ?? []).length > 0 && (
        <Rows>{s.sites!.map((site, i) => <SiteRow key={i} site={site} linked={linked} muted={s.stopped} t={t} />)}</Rows>
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

function SiteRow({ site, linked, muted, t }: { site: WebSite; linked: Map<string, number> | null; muted?: boolean; t: T }) {
  const names = site.names ?? [];
  const details = [
    site.proxy?.length ? <Detail key="p" label={t("scan.web.proxy")}>{site.proxy.join(", ")}</Detail> : null,
    site.fastcgi?.length ? <Detail key="f" label="FastCGI">{site.fastcgi.join(", ")}</Detail> : null,
    site.root ? <Detail key="r" label={t("scan.web.root")}>{site.root}</Detail> : null,
    site.source ? <Detail key="s" label={t("scan.web.source")}>{site.source}</Detail> : null,
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
        <span className="ml-auto shrink-0 font-mono text-[var(--text-muted)]">
          {(site.listen ?? []).join(", ")}{site.tls ? " · TLS" : ""}
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
