"use client";

import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import { UsageBar } from "../UsageBar";
import { pctBarColor } from "@/lib/utils";
import type { VMInfoType, ResourceUsageSnapshot } from "@/lib/api";
import { pctColor, type T } from "./shared";

/** Usage at scan time (CPU/RAM/disk bars), every partition, plus the top consumers of each. */
export default function ScanResources({ info, t }: { info: VMInfoType; t: T }) {
  // The disk % is the fullest partition's: its bar shows that partition's own
  // used / size (the totals are on the partition list's header).
  const fullest = info.filesystems?.find((fs) => fs.mount === info.disk_fullest && !fs.network);
  const bars = [
    info.cpu_usage && <UsageBar key="cpu" label={t("vm.cpu")} total={info.cpu} used={info.cpu_usage} percent={info.cpu_usage} />,
    info.ram_percent && <UsageBar key="ram" label={t("vm.ram")} total={info.ram} used={info.ram_used} percent={info.ram_percent} />,
    info.disk_percent && (fullest
      ? <UsageBar key="disk" label={t("scan.storage.diskFullest", { mount: fullest.mount })} total={humanizeBytesUI(fullest.size_bytes)} used={humanizeBytesUI(fullest.used_bytes)} percent={info.disk_percent} />
      : <UsageBar key="disk" label={t("vm.disk")} total={info.storage} used={info.storage_used} percent={info.disk_percent} />),
  ].filter(Boolean);
  const top = info.resource_top;
  const hasTop = !!top && (top.top_cpu?.length || 0) + (top.top_mem?.length || 0) + (top.top_disk?.length || 0) > 0;
  const hasStorage = (info.filesystems?.length ?? 0) > 0;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.resources")} empty={bars.length === 0 && !hasTop && !hasStorage ? t("scan.pane.empty.resources") : undefined}>
      {bars.length > 0 && <div className="grid grid-cols-1 md:grid-cols-3 gap-4">{bars}</div>}
      {hasStorage && (
        <div className={bars.length > 0 ? "mt-4 pt-4 border-t border-[var(--border-subtle)]" : ""}>
          <StoragePanel info={info} t={t} />
        </div>
      )}
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

/**
 * Every partition with its own bar (the disk % above is only the fullest),
 * then the raw disks — one with nothing on it is "unused" — and LVM free space.
 */
function StoragePanel({ info, t }: { info: VMInfoType; t: T }) {
  const disks = info.disks ?? [];
  const vgs = info.volume_groups ?? [];
  const disksTotal = disks.reduce((n, d) => n + d.size_bytes, 0);
  const local = (info.filesystems ?? []).filter((fs) => !fs.network);
  return (
    <div>
      <span className="block text-2xs text-[var(--text-faint)] mb-2">
        {t("scan.storage.partitions")}
        {local.length > 0 && ` · ${t("scan.storage.total", {
          used: humanizeBytesUI(local.reduce((n, fs) => n + fs.used_bytes, 0)),
          size: humanizeBytesUI(local.reduce((n, fs) => n + fs.size_bytes, 0)),
          count: String(local.length),
        })}`}
      </span>
      <div className="space-y-2.5">
        {(info.filesystems ?? []).map((fs) => (
          <div key={fs.mount} className="text-xs min-w-0">
            <div className="flex items-baseline gap-2 min-w-0">
              <span className="font-mono text-[var(--text-primary)] truncate" title={fs.mount}>{fs.mount}</span>
              {fs.network && <Badge compact color="info">{t("scan.storage.network")}</Badge>}
              <span className="text-2xs text-[var(--text-faint)] font-mono truncate" title={fs.device}>{fs.device} · {fs.fstype}</span>
              <span className={`ml-auto shrink-0 font-mono tabular-nums ${fs.size_bytes ? pctColor(fs.used_pct) : "text-[var(--text-faint)]"}`}>
                {fs.size_bytes
                  ? `${humanizeBytesUI(fs.used_bytes)} / ${humanizeBytesUI(fs.size_bytes)} (${Math.round(fs.used_pct)}%)`
                  : t("scan.storage.noAnswer")}
              </span>
            </div>
            {fs.size_bytes > 0 && (
              <div className="mt-1 w-full h-1.5 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
                <div className={`h-full rounded-full ${pctBarColor(fs.used_pct)}`} style={{ width: `${Math.min(fs.used_pct, 100)}%` }} />
              </div>
            )}
          </div>
        ))}
      </div>
      {(disks.length > 0 || vgs.length > 0) && (
        <div className="mt-3 flex flex-wrap items-center gap-1.5">
          <span className="text-2xs text-[var(--text-faint)] mr-1">
            {t("scan.storage.disks")}{disksTotal > 0 ? ` (${humanizeBytesUI(disksTotal)})` : ""}
          </span>
          {disks.map((d) => (
            <Badge key={d.name} compact color={d.unused ? "warning" : "default"}>
              <span className="font-mono">{d.name}</span> {humanizeBytesUI(d.size_bytes)}{d.unused ? ` · ${t("scan.storage.unused")}` : ""}
            </Badge>
          ))}
          {vgs.map((v) => (
            <Badge key={v.name} compact color={v.free_bytes > 0 ? "info" : "default"}>
              LVM <span className="font-mono">{v.name}</span>: {humanizeBytesUI(v.free_bytes)} {t("scan.storage.free")}
            </Badge>
          ))}
        </div>
      )}
    </div>
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
