"use client";

import SectionCard from "@/components/ui/SectionCard";
import { UsageBar } from "../UsageBar";
import type { VMInfoType, ResourceUsageSnapshot } from "@/lib/api";
import type { T } from "./shared";

/** Usage at scan time (CPU/RAM/disk bars) plus the top consumers of each. */
export default function ScanResources({ info, t }: { info: VMInfoType; t: T }) {
  const bars = [
    info.cpu_usage && <UsageBar key="cpu" label={t("vm.cpu")} total={info.cpu} used={info.cpu_usage} percent={info.cpu_usage} />,
    info.ram_percent && <UsageBar key="ram" label={t("vm.ram")} total={info.ram} used={info.ram_used} percent={info.ram_percent} />,
    info.disk_percent && <UsageBar key="disk" label={t("vm.disk")} total={info.storage} used={info.storage_used} percent={info.disk_percent} />,
  ].filter(Boolean);
  const top = info.resource_top;
  const hasTop = !!top && (top.top_cpu?.length || 0) + (top.top_mem?.length || 0) + (top.top_disk?.length || 0) > 0;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.resources")} empty={bars.length === 0 && !hasTop ? t("scan.pane.empty.resources") : undefined}>
      {bars.length > 0 && <div className="grid grid-cols-1 md:grid-cols-3 gap-4">{bars}</div>}
      {/* Top consumers answer "the host is at 87% RAM, who is using it?"
          without opening another terminal. */}
      {hasTop && (
        <div className={bars.length > 0 ? "mt-4 pt-4 border-t border-[var(--border-subtle)]" : ""}>
          <ResourceTopPanel snapshot={top!} t={t} />
        </div>
      )}
    </SectionCard>
  );
}

function ResourceTopPanel({ snapshot, t }: { snapshot: ResourceUsageSnapshot; t: T }) {
  const cpu = snapshot.top_cpu ?? [];
  const mem = snapshot.top_mem ?? [];
  const disk = snapshot.top_disk ?? [];
  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
      {cpu.length > 0 && (
        <ResourceTopList
          title={t("scan.resourceTop.cpu")}
          items={cpu.map((p) => ({
            primary: truncateCommand(p.command),
            secondary: `${p.user}  pid ${p.pid}`,
            value: `${p.cpu_percent.toFixed(1)}%`,
            tooltip: p.command,
          }))}
        />
      )}
      {mem.length > 0 && (
        <ResourceTopList
          title={t("scan.resourceTop.mem")}
          items={mem.map((p) => ({
            primary: truncateCommand(p.command),
            secondary: `${p.user}  ${humanizeBytesUI(p.rss_bytes)} (${p.mem_percent.toFixed(1)}%)`,
            value: humanizeBytesUI(p.rss_bytes),
            tooltip: p.command,
          }))}
        />
      )}
      {disk.length > 0 && (
        <ResourceTopList
          title={t("scan.resourceTop.disk")}
          items={disk.map((d) => ({
            primary: d.path,
            secondary: "",
            value: d.human_size || humanizeBytesUI(d.size_bytes),
            tooltip: d.path,
          }))}
        />
      )}
    </div>
  );
}

function ResourceTopList({ title, items }: {
  title: string;
  items: Array<{ primary: string; secondary: string; value: string; tooltip?: string }>;
}) {
  return (
    <div>
      <span className="block text-2xs text-[var(--text-faint)] mb-2">{title}</span>
      <div className="space-y-1">
        {items.map((it, i) => (
          <div key={i} className="flex items-baseline gap-2 px-1.5 py-1 rounded hover:bg-[var(--bg-elevated)]/40 text-xs" title={it.tooltip}>
            <span className="shrink-0 text-2xs text-[var(--text-faint)] w-4 tabular-nums">{i + 1}</span>
            <div className="flex-1 min-w-0">
              <div className="text-[var(--text-primary)] truncate font-mono">{it.primary || "–"}</div>
              {it.secondary && <div className="text-2xs text-[var(--text-muted)] truncate">{it.secondary}</div>}
            </div>
            <span className="shrink-0 text-[var(--text-secondary)] font-medium tabular-nums font-mono">{it.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// Long argv strings (Java classpaths, etc) are cut to a 60-char preview so the
// rows stay aligned; the full string lives in the row's tooltip.
function truncateCommand(cmd: string): string {
  if (!cmd) return "";
  return cmd.length <= 60 ? cmd : cmd.slice(0, 60) + "…";
}

function humanizeBytesUI(n: number): string {
  if (!n || n < 1024) return `${n || 0} B`;
  const units = ["KiB", "MiB", "GiB", "TiB", "PiB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${units[i]}`;
}
