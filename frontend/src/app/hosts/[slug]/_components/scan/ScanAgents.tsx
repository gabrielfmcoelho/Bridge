"use client";

import SectionCard from "@/components/ui/SectionCard";
import StatusDot from "@/components/ui/StatusDot";
import Badge from "@/components/ui/Badge";
import type { VMInfoType, Agent } from "@/lib/api";
import { SubBlock, type T } from "./shared";

const CATEGORY_ORDER = ["monitoring", "logging", "security", "config-mgmt", "inventory", "backup", "cloud", "orchestration", "remote-access"];
const CATEGORY_ICON: Record<string, string> = {
  monitoring: "📊", logging: "📝", security: "🛡️", "config-mgmt": "⚙️", inventory: "📋",
  backup: "💾", cloud: "☁️", orchestration: "🚢", "remote-access": "🔌",
};

/**
 * Management agents — what is watching this host (monitoring, logging,
 * security, backup…), each collapsing systemd + process + package + port
 * signals into one row, grouped by category. The classified services are
 * workloads (ScanWorkloads). Scans from before the inventory fall back to the
 * raw systemd list.
 */
export default function ScanAgents({ info, t }: { info: VMInfoType; t: T }) {
  const agents = info.agents ?? [];
  const legacySystemd = !info.service_inventory ? info.systemd_services ?? [] : [];
  const empty = agents.length + legacySystemd.length === 0;

  // Canonical categories first, unknown ones appended alphabetically.
  const buckets = new Map<string, Agent[]>();
  for (const a of agents) buckets.set(a.category ?? "", [...(buckets.get(a.category ?? "") ?? []), a]);
  const categories = [...buckets.keys()].sort((a, b) => {
    const ia = CATEGORY_ORDER.indexOf(a), ib = CATEGORY_ORDER.indexOf(b);
    return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b);
  });
  const active = agents.filter((a) => isActive(a.state)).length;

  return (
    <SectionCard
      as="h3"
      title={t("scan.pane.group.agents")}
      description={agents.length > 0 ? t("scan.agents.summary") : undefined}
      empty={empty ? t("scan.pane.empty.agents") : undefined}
    >
      {agents.length > 0 && (
        <SubBlock>
          <p className="text-2xs text-[var(--text-faint)] font-mono mb-2">{active}/{agents.length} {t("scan.agents.activeOfTotal")}</p>
          <div className="space-y-3">
            {categories.map((c) => {
              const list = buckets.get(c)!;
              return (
                <div key={c}>
                  <span className="text-2xs font-semibold text-[var(--text-faint)] block mb-1">
                    <span className="mr-1">{CATEGORY_ICON[c] ?? "⚙️"}</span>
                    {tOr(t, `scan.agents.category.${c}`, c)} <span className="opacity-60">({list.length})</span>
                  </span>
                  <div className="divide-y divide-[var(--border-subtle)]/60">
                    {list.map((a) => <AgentRow key={`${a.name}@${a.instance ?? ""}`} a={a} t={t} />)}
                  </div>
                </div>
              );
            })}
          </div>
        </SubBlock>
      )}
      {legacySystemd.length > 0 && (
        <SubBlock title={t("scan.systemdServices")} aside={legacySystemd.length}>
          <div className="flex flex-wrap gap-1.5">
            {legacySystemd.map((svc, i) => (
              <span key={i} title={svc.description || svc.unit}>
                <Badge color={svc.is_native ? "success" : "info"} dot>
                  {svc.unit.replace(".service", "")} <span className="opacity-60">({svc.is_native ? t("scan.native") : t("scan.containerManaged")})</span>
                </Badge>
              </span>
            ))}
          </div>
        </SubBlock>
      )}
    </SectionCard>
  );
}

function AgentRow({ a, t }: { a: Agent; t: T }) {
  return (
    <div className="flex flex-wrap items-center gap-2 py-1.5 text-xs">
      <span className="inline-flex items-center gap-1.5 w-20 shrink-0 text-2xs text-[var(--text-secondary)]">
        <StatusDot size="xs" color={stateColor(a.state)} />
        {tOr(t, `scan.agents.state.${a.state ?? "unknown"}`, a.state ?? "–")}
      </span>
      <span className="font-medium text-[var(--text-primary)]">{a.label}</span>
      {a.vendor && <span className="text-2xs text-[var(--text-faint)]">{a.vendor}</span>}
      {a.version && (
        <span className="text-2xs text-[var(--text-muted)] font-mono" title={a.package ? `${a.package} ${a.version}` : a.version}>v{a.version}</span>
      )}
      {a.enabled && <span title={t("scan.agents.enabledAtBootTooltip")}><Badge color="info">{t("scan.agents.enabledAtBoot")}</Badge></span>}
      {a.ports && a.ports.length > 0 && (
        <span className="text-2xs text-[var(--text-muted)] font-mono" title={t("scan.agents.portsTooltip")}>:{a.ports.join(", :")}</span>
      )}
      <span className="ml-auto flex items-center gap-1.5 flex-wrap">
        {(a.sources ?? []).map((src) => (
          <span key={src} className="text-2xs text-[var(--text-faint)] font-mono" title={t(`scan.agents.source.${src}`)}>{src}</span>
        ))}
      </span>
    </div>
  );
}

const isActive = (s?: string) => s === "active" || s === "running";

function stateColor(s?: string): "success" | "danger" | "warning" | "muted" {
  if (isActive(s)) return "success";
  if (s === "failed") return "danger";
  if (s === "stopped" || s === "inactive") return "warning";
  return "muted";
}

/** Translation, or `fallback` when the catalogue has no such key. */
function tOr(t: T, key: string, fallback: string): string {
  const v = t(key);
  return v === key ? fallback : v;
}
