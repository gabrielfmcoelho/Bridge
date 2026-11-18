"use client";

import SectionCard from "@/components/ui/SectionCard";
import type { FirewallTool, VMInfoType } from "@/lib/api";
import { Detail, SubBlock, type T } from "./shared";

/**
 * A10 firewall reality: ufw AND firewalld/nft (host 35's firewalld is invisible
 * today), the iptables rule count, fail2ban jails, and which admin ports are
 * exposed — and how wide (all interfaces / one IP / loopback). Only counts and
 * classes, never a bind address. Background scan only.
 */
export default function ScanFirewall({ info, t }: { info: VMInfoType; t: T }) {
  const f = info.deep_facts?.firewall;
  // The quick test's raw status (ufw / iptables output) shows here too, so
  // the firewall is one card; `sectionState` in lib/scanSections mirrors this.
  const raw = info.firewall_status;
  const empty = raw
    ? undefined
    : info.deep_facts == null
      ? t("scan.deep.notRun")
      : !f
        ? t("scan.pane.empty.firewall")
        : undefined;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.firewall")} empty={empty}>
      {f && (
        <SubBlock>
        <div className="space-y-3">
          <p className="text-2xs text-[var(--text-muted)]">{t("scan.deep.firewall.summary")}</p>
          <div className="flex flex-wrap gap-1.5">
            <ToolChip name="ufw" tool={f.ufw} t={t} />
            <ToolChip name="firewalld" tool={f.firewalld} t={t} />
            <ToolChip name="nft" tool={f.nft} t={t} />
            {f.iptables_rules ? (
              <span className="rounded px-2 py-0.5 text-xs bg-[var(--bg-elevated)] text-[var(--text-secondary)]">
                {t("scan.deep.firewall.iptables")} · {t("scan.deep.firewall.rules", { count: f.iptables_rules.toLocaleString() })}
              </span>
            ) : null}
          </div>
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
            {f.fail2ban && f.fail2ban.length > 0 && (
              <Detail label={t("scan.deep.firewall.fail2ban")}>
                <span className="inline-flex flex-wrap gap-1.5">
                  {f.fail2ban.map((j) => (
                    <span key={j.jail} className="rounded px-1.5 text-2xs bg-[var(--bg-elevated)] text-[var(--text-secondary)]">
                      {j.jail}{j.banned ? ` · ${t("scan.deep.firewall.banned", { count: j.banned.toLocaleString() })}` : ""}
                    </span>
                  ))}
                </span>
              </Detail>
            )}
            {f.admin_ports && f.admin_ports.length > 0 && (
              <Detail label={t("scan.deep.firewall.adminPorts")}>
                <span className="inline-flex flex-wrap gap-1.5">
                  {f.admin_ports.map((p) => (
                    <span key={p.port} className={`rounded px-1.5 text-2xs tabular-nums ${EXPOSURE_TONE[p.exposure] ?? EXPOSURE_TONE.loopback}`}>
                      {p.port} · {t(EXPOSURE_LABEL[p.exposure] ?? "scan.deep.firewall.exposureLoopback")}
                    </span>
                  ))}
                </span>
              </Detail>
            )}
          </dl>
        </div>
        </SubBlock>
      )}
      {raw && (
        <SubBlock title={f ? t("scan.firewallStatus") : undefined}>
          <pre className="text-xs text-[var(--text-secondary)] whitespace-pre-wrap break-all font-mono">{raw}</pre>
        </SubBlock>
      )}
    </SectionCard>
  );
}

function ToolChip({ name, tool, t }: { name: string; tool?: FirewallTool; t: T }) {
  if (!tool) return null;
  const tone = tool.active ? "bg-[var(--success)]/10 text-[var(--success)]" : "bg-[var(--bg-elevated)] text-[var(--text-muted)]";
  return (
    <span className={`rounded px-2 py-0.5 text-xs ${tone}`}>
      <span className="font-mono">{name}</span> · {t(tool.active ? "scan.deep.firewall.active" : "scan.deep.firewall.inactive")}
      {tool.rules ? ` · ${t("scan.deep.firewall.rules", { count: tool.rules.toLocaleString() })}` : ""}
    </span>
  );
}

// Literal class strings per exposure (Tailwind can't see a templated class).
const EXPOSURE_TONE: Record<string, string> = {
  all: "bg-[var(--danger)]/10 text-[var(--danger)]",
  "one-ip": "bg-[var(--warning)]/10 text-[var(--warning)]",
  loopback: "bg-[var(--success)]/10 text-[var(--success)]",
};
const EXPOSURE_LABEL: Record<string, string> = {
  all: "scan.deep.firewall.exposureAll",
  "one-ip": "scan.deep.firewall.exposureOneIp",
  loopback: "scan.deep.firewall.exposureLoopback",
};
