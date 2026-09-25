"use client";

import type { ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { VMInfoType } from "@/lib/api";
import ScanResources from "./scan/ScanResources";
import ScanSystem from "./scan/ScanSystem";
import ScanUsers from "./scan/ScanUsers";
import ScanProcesses from "./scan/ScanProcesses";
import ScanContainers from "./scan/ScanContainers";
import ScanServices from "./scan/ScanServices";
import ScanPackages from "./scan/ScanPackages";
import ScanSecurity from "./scan/ScanSecurity";

export const SCAN_GROUPS = ["resources", "system", "users", "processes", "containers", "services", "packages", "security"] as const;
export type ScanGroup = (typeof SCAN_GROUPS)[number];

/**
 * One scan, as its eight groups — always all eight, each saying so when the
 * scan has nothing for it. Used by the host overview's scan pane (with
 * `anchorPrefix`, so the jump bar can reach each group) and by the
 * Operations console after a scan.
 */
export default function VMInfoDisplay({ info, locale, anchorPrefix }: { info: VMInfoType; locale: string; anchorPrefix?: string }) {
  const { t } = useLocale();
  const groups: Record<ScanGroup, ReactNode> = {
    resources: <ScanResources info={info} t={t} />,
    system: <ScanSystem info={info} locale={locale} t={t} />,
    users: <ScanUsers info={info} locale={locale} t={t} />,
    processes: <ScanProcesses info={info} t={t} />,
    containers: <ScanContainers info={info} t={t} />,
    services: <ScanServices info={info} t={t} />,
    packages: <ScanPackages info={info} t={t} />,
    security: <ScanSecurity info={info} t={t} />,
  };
  return (
    <div className="space-y-5 text-sm">
      {SCAN_GROUPS.map((g) => (
        <div key={g} id={anchorPrefix ? `${anchorPrefix}${g}` : undefined} className="scroll-mt-16">
          {groups[g]}
        </div>
      ))}
    </div>
  );
}
