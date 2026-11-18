"use client";

import SectionCard from "@/components/ui/SectionCard";
import { portIcon } from "@/lib/utils";
import type { VMInfoType, PortOwner } from "@/lib/api";
import type { T } from "./shared";

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

/**
 * The listening ports and who owns each — first in the workloads group, right
 * above the containers, services and processes the chips name.
 */
export default function ScanPorts({ info, t }: { info: VMInfoType; t: T }) {
  const ports = info.ports ?? [];
  const ownerByPort = new Map<number, PortOwner>();
  for (const po of info.port_owners ?? []) ownerByPort.set(po.port, po);

  return (
    <SectionCard as="h3" title={t("scan.pane.group.ports")} count={ports.length || undefined} empty={ports.length === 0 ? t("scan.pane.empty.ports") : undefined}>
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
    </SectionCard>
  );
}
