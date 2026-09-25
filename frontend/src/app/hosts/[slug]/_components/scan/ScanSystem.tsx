"use client";

import SectionCard from "@/components/ui/SectionCard";
import { formatUptime, portIcon } from "@/lib/utils";
import type { VMInfoType, PortOwner } from "@/lib/api";
import { SubBlock, type T } from "./shared";

// Chip colour by port owner type, so the panel scans at a glance: agents
// (what's monitoring me) apart from services (what am I actually running).
function ownerClasses(ownerType?: string): string {
  switch (ownerType) {
    case "container": return "bg-[var(--cyan)]/15 text-[var(--cyan)] border-[var(--cyan)]/40";
    case "nginx":
    case "agent": return "bg-[var(--accent)]/15 text-[var(--accent)] border-[var(--accent)]/40";
    case "docker": return "bg-[var(--info)]/15 text-[var(--info)] border-[var(--info)]/40";
    case "service": return "bg-[var(--success)]/15 text-[var(--success)] border-[var(--success)]/40";
    default: return "bg-[var(--bg-elevated)] text-[var(--text-secondary)] border-[var(--border-default)]";
  }
}

/** OS facts from the scan plus the listening ports and who owns them. */
export default function ScanSystem({ info, locale, t }: { info: VMInfoType; locale: string; t: T }) {
  const uptime = info.uptime ? formatUptime(info.uptime, locale) : "";
  const rows = [
    info.os && [t("vm.os"), info.os],
    info.kernel && [t("vm.kernel"), info.kernel],
    uptime && [t("vm.uptime"), uptime],
    info.hostname_remote && [t("vm.hostname"), info.hostname_remote],
    info.public_ip && [t("vm.publicIp"), info.public_ip],
    info.load_avg && [t("vm.loadAvg"), info.load_avg],
    info.logged_users && [t("vm.usersOnline"), info.logged_users],
    info.swap_total && [t("vm.swap"), `${info.swap_total} (used: ${info.swap_used})`],
  ].filter(Boolean) as [string, string][];
  const ports = info.ports ?? [];
  const ownerByPort = new Map<number, PortOwner>();
  for (const po of info.port_owners ?? []) ownerByPort.set(po.port, po);

  return (
    <SectionCard as="h3" title={t("scan.pane.group.system")} empty={rows.length === 0 && ports.length === 0 ? t("scan.pane.empty.system") : undefined}>
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
      {ports.length > 0 && (
        <SubBlock title={t("scan.listeningPorts")} aside={ports.length}>
          <div className="flex flex-wrap gap-1.5">
            {ports.map((p, i) => {
              const parts = p.split(/\s+/);
              const portStr = parts[0] || "";
              const portNum = parseInt(portStr, 10);
              const rawProc = parts.slice(1).join(" ");
              const owner = Number.isNaN(portNum) ? undefined : ownerByPort.get(portNum);
              const displayName = owner?.owner_name || rawProc;
              const tooltip = [owner?.owner_type, owner?.target, rawProc && rawProc !== owner?.owner_name ? `proc=${rawProc}` : ""]
                .filter(Boolean).join(" · ") || rawProc;
              return (
                <span key={i} className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full border text-xs ${ownerClasses(owner?.owner_type)}`} title={tooltip}>
                  {portIcon(p)}
                  <span className="font-mono">:{portStr}</span>
                  {displayName && (
                    <span className="text-2xs opacity-80 truncate max-w-[10rem] font-mono">
                      {owner?.owner_type === "nginx" && owner.target ? `nginx → ${owner.target}` : displayName}
                    </span>
                  )}
                </span>
              );
            })}
          </div>
        </SubBlock>
      )}
    </SectionCard>
  );
}
