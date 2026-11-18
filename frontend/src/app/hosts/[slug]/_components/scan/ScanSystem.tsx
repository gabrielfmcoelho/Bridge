"use client";

import SectionCard from "@/components/ui/SectionCard";
import { formatUptime } from "@/lib/utils";
import type { VMInfoType } from "@/lib/api";
import { SubBlock, type T } from "./shared";

/** OS facts from the scan. */
export default function ScanSystem({ info, locale, t }: { info: VMInfoType; locale: string; t: T }) {
  const uptime = info.uptime ? formatUptime(info.uptime, locale) : "";
  const rows = [
    info.os && [t("vm.os"), info.os],
    info.kernel && [t("vm.kernel"), info.kernel],
    info.arch && [t("vm.arch"), info.arch],
    info.reboot_required && [t("vm.reboot"), t("host.rebootPending")],
    uptime && [t("vm.uptime"), uptime],
    info.hostname_remote && [t("vm.hostname"), info.hostname_remote],
    info.public_ip && [t("vm.publicIp"), info.public_ip],
    info.load_avg && [t("vm.loadAvg"), info.load_avg],
    info.logged_users && [t("vm.usersOnline"), info.logged_users],
    info.swap_total && [t("vm.swap"), `${info.swap_total} (used: ${info.swap_used})`],
  ].filter(Boolean) as [string, string][];
  return (
    <SectionCard as="h3" title={t("scan.pane.group.system")} empty={rows.length === 0 ? t("scan.pane.empty.system") : undefined}>
      {rows.length > 0 && (
        <SubBlock>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6">
            {rows.map(([label, value]) => (
              <div key={label} className="flex justify-between gap-3 py-1.5 border-b border-[var(--border-subtle)]/50">
                <span className="text-xs text-[var(--text-muted)]">{label}</span>
                <span className="text-xs text-[var(--text-primary)] font-mono text-right">{value}</span>
              </div>
            ))}
          </div>
        </SubBlock>
      )}
    </SectionCard>
  );
}
