"use client";

import type { ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import SectionHeading from "@/components/ui/SectionHeading";
import type { VMInfoType } from "@/lib/api";
import { scanLayout, type ScanSection } from "@/lib/scanSections";
import ScanResources from "./scan/ScanResources";
import ScanSystem from "./scan/ScanSystem";
import ScanPorts from "./scan/ScanPorts";
import ScanUsers from "./scan/ScanUsers";
import ScanProcesses from "./scan/ScanProcesses";
import ScanContainers from "./scan/ScanContainers";
import ScanServices from "./scan/ScanServices";
import ScanWebServers from "./scan/ScanWebServers";
import ScanGitRemotes from "./scan/ScanGitRemotes";
import ScanOutbound from "./scan/ScanOutbound";
import ScanPatch from "./scan/ScanPatch";
import ScanTraffic from "./scan/ScanTraffic";
import ScanPanels from "./scan/ScanPanels";
import ScanCode from "./scan/ScanCode";
import ScanOwner from "./scan/ScanOwner";
import ScanFirewall from "./scan/ScanFirewall";
import ScanRoles from "./scan/ScanRoles";
import ScanPackages from "./scan/ScanPackages";
import ScanSecurity from "./scan/ScanSecurity";

/**
 * One scan, as question-based groups (lib/scanSections): a group shows only
 * the sections that have data, and a closing note names the ones the scan
 * found nothing for and the ones it did not check. Used by the host
 * overview's scan pane (with `anchorPrefix`, so the jump bar can reach each
 * group) and by the Operations console after a scan.
 */
export default function VMInfoDisplay({ info, locale, anchorPrefix, dnsRecords, hostSlug }: {
  info: VMInfoType; locale: string; anchorPrefix?: string;
  /** The host the scan belongs to: secret chips link to its Segredos tab. */
  hostSlug: string;
  /** The host's linked DNS records: web-server names show linked / not linked. */
  dnsRecords?: { id: number; domain: string; inbound?: boolean }[];
}) {
  const { t } = useLocale();
  const sections: Record<ScanSection, ReactNode> = {
    resources: <ScanResources info={info} t={t} hostSlug={hostSlug} />,
    ports: <ScanPorts info={info} t={t} />,
    containers: <ScanContainers info={info} t={t} />,
    services: <ScanServices info={info} t={t} />,
    processes: <ScanProcesses info={info} t={t} hostSlug={hostSlug} />,
    git: <ScanGitRemotes info={info} t={t} />,
    roles: <ScanRoles info={info} t={t} />,
    webservers: <ScanWebServers info={info} t={t} dnsRecords={dnsRecords} />,
    traffic: <ScanTraffic info={info} t={t} />,
    code: <ScanCode info={info} t={t} />,
    panels: <ScanPanels info={info} t={t} />,
    users: <ScanUsers info={info} locale={locale} t={t} />,
    owner: <ScanOwner info={info} t={t} />,
    security: <ScanSecurity info={info} t={t} />,
    firewall: <ScanFirewall info={info} t={t} />,
    outbound: <ScanOutbound info={info} t={t} />,
    system: <ScanSystem info={info} locale={locale} t={t} />,
    patch: <ScanPatch info={info} t={t} />,
    packages: <ScanPackages info={info} t={t} hostSlug={hostSlug} />,
  };
  const { groups, empty, notRun } = scanLayout(info);
  const names = (list: ScanSection[]) => list.map((s) => t(`scan.pane.group.${s}`)).join(", ");

  return (
    <div className="space-y-6 text-sm">
      {groups.map(({ group, sections: shown }) => (
        <section key={group} id={anchorPrefix ? `${anchorPrefix}${group}` : undefined} className="scroll-mt-16 space-y-3">
          {/* A lone card already carries its own title. */}
          {shown.length > 1 && <SectionHeading variant="section">{t(`scan.pane.area.${group}`)}</SectionHeading>}
          {shown.map((s) => <div key={s}>{sections[s]}</div>)}
        </section>
      ))}
      {(empty.length > 0 || notRun.length > 0) && (
        <div className="space-y-1 text-xs text-[var(--text-muted)]">
          {notRun.length > 0 && <p>{t("scan.pane.notChecked", { list: names(notRun) })}</p>}
          {empty.length > 0 && <p>{t("scan.pane.nothingFound", { list: names(empty) })}</p>}
        </div>
      )}
    </div>
  );
}
