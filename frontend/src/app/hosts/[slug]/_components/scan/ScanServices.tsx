"use client";

import SectionCard from "@/components/ui/SectionCard";
import StatusDot from "@/components/ui/StatusDot";
import Badge from "@/components/ui/Badge";
import type { VMInfoType, Agent, DiscoveredService } from "@/lib/api";
import { SubBlock, type T } from "./shared";

const AGENT_CATEGORY_ORDER = ["monitoring", "logging", "security", "config-mgmt", "inventory", "backup", "cloud", "orchestration", "remote-access"];
const AGENT_CATEGORY_ICON: Record<string, string> = {
  monitoring: "📊", logging: "📝", security: "🛡️", "config-mgmt": "⚙️", inventory: "📋",
  backup: "💾", cloud: "☁️", orchestration: "🚢", "remote-access": "🔌",
};
const SERVICE_KIND_ORDER = ["web", "proxy", "database", "cache", "queue", "runtime", "dns", "mail", "file-sharing", "directory", "analytics", "logging"];
const SERVICE_KIND_ICON: Record<string, string> = {
  web: "🌐", proxy: "🔀", database: "🗄️", cache: "⚡", queue: "📨", runtime: "⚙️", dns: "📡",
  mail: "✉️", "file-sharing": "📁", directory: "🪪", analytics: "📈", logging: "📜",
};

type Entry = Partial<Agent> & Partial<DiscoveredService> & { name: string; label: string };
type Ns = "agents" | "services";

/**
 * What runs on the host as a service: management agents (what's watching
 * this host) and the classified service inventory (web, database, cache, …),
 * each collapsing systemd + process + package + port + container signals into
 * one row. Scans from before the inventory fall back to the raw systemd list.
 */
export default function ScanServices({ info, t }: { info: VMInfoType; t: T }) {
  const agents = info.agents ?? [];
  const services = info.service_inventory ?? [];
  const legacySystemd = !info.service_inventory ? info.systemd_services ?? [] : [];
  const empty = agents.length + services.length + legacySystemd.length === 0;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.services")} empty={empty ? t("scan.pane.empty.services") : undefined}>
      {agents.length > 0 && (
        <Inventory ns="agents" title={t("scan.agents.title")} entries={agents} groupOf={(a) => a.category ?? ""} order={AGENT_CATEGORY_ORDER} icons={AGENT_CATEGORY_ICON} t={t} />
      )}
      {services.length > 0 && (
        <Inventory ns="services" title={t("scan.services.title")} entries={services} groupOf={(s) => s.kind ?? ""} order={SERVICE_KIND_ORDER} icons={SERVICE_KIND_ICON} t={t} />
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

function Inventory({ ns, title, entries, groupOf, order, icons, t }: {
  ns: Ns; title: string; entries: Entry[]; groupOf: (e: Entry) => string; order: string[]; icons: Record<string, string>; t: T;
}) {
  // Canonical groups first, unknown ones appended alphabetically.
  const buckets = new Map<string, Entry[]>();
  for (const e of entries) buckets.set(groupOf(e), [...(buckets.get(groupOf(e)) ?? []), e]);
  const groups = [...buckets.keys()].sort((a, b) => {
    const ia = order.indexOf(a), ib = order.indexOf(b);
    return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b);
  });
  const active = entries.filter((e) => isActive(e.state)).length;
  const groupKey = ns === "agents" ? "category" : "kind";

  return (
    <SubBlock title={title} aside={`${active}/${entries.length} ${t(`scan.${ns}.activeOfTotal`)}`}>
      <p className="text-2xs text-[var(--text-muted)] mb-3">{t(`scan.${ns}.summary`)}</p>
      <div className="space-y-3">
        {groups.map((g) => {
          const list = buckets.get(g)!;
          return (
            <div key={g}>
              <span className="text-2xs font-semibold text-[var(--text-faint)] block mb-1">
                <span className="mr-1">{icons[g] ?? "⚙️"}</span>
                {tOr(t, `scan.${ns}.${groupKey}.${g}`, g)} <span className="opacity-60">({list.length})</span>
              </span>
              <div className="divide-y divide-[var(--border-subtle)]/60">
                {list.map((e) => <InventoryRow key={e.name} ns={ns} e={e} t={t} />)}
              </div>
            </div>
          );
        })}
      </div>
    </SubBlock>
  );
}

function InventoryRow({ ns, e, t }: { ns: Ns; e: Entry; t: T }) {
  return (
    <div className="flex flex-wrap items-center gap-2 py-1.5 text-xs">
      <span className="inline-flex items-center gap-1.5 w-20 shrink-0 text-2xs text-[var(--text-secondary)]">
        <StatusDot size="xs" color={stateColor(e.state)} />
        {tOr(t, `scan.${ns}.state.${e.state ?? "unknown"}`, e.state ?? "–")}
      </span>
      <span className="font-medium text-[var(--text-primary)]">{e.label}</span>
      {e.vendor && <span className="text-2xs text-[var(--text-faint)]">{e.vendor}</span>}
      {e.version && (
        <span className="text-2xs text-[var(--text-muted)] font-mono" title={e.package ? `${e.package} ${e.version}` : e.version}>v{e.version}</span>
      )}
      {e.enabled && <span title={t(`scan.${ns}.enabledAtBootTooltip`)}><Badge color="info">{t(`scan.${ns}.enabledAtBoot`)}</Badge></span>}
      {e.host_running && <span title={t("scan.services.hostInstanceTooltip")}><Badge color="purple">{t("scan.services.hostInstance")}</Badge></span>}
      {e.container_image && (
        <span title={e.container_id ? `${e.container_image} (${e.container_id.slice(0, 12)})` : e.container_image}>
          <Badge color="cyan" className="font-mono">{e.container_image}</Badge>
        </span>
      )}
      {e.ports && e.ports.length > 0 && (
        <span className="text-2xs text-[var(--text-muted)] font-mono" title={t(`scan.${ns}.portsTooltip`)}>:{e.ports.join(", :")}</span>
      )}
      <span className="ml-auto flex items-center gap-1.5 flex-wrap">
        {(e.sources ?? []).map((src) => (
          <span key={src} className="text-2xs text-[var(--text-faint)] font-mono" title={t(`scan.${ns}.source.${src}`)}>{src}</span>
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
