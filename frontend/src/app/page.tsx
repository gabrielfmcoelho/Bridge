"use client";

import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import Link from "next/link";
import { dashboardAPI, hostsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import StatCard from "@/components/ui/StatCard";
import { SkeletonStats } from "@/components/ui/Skeleton";
import SectionHeading from "@/components/ui/SectionHeading";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import StatusDot from "@/components/ui/StatusDot";

const statConfig = [
  { key: "hosts", color: "cyan", icon: "M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zm0 10h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4a2 2 0 012-2z" },
  { key: "dns", color: "emerald", icon: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93z" },
  { key: "projects", color: "purple", icon: "M2 6a2 2 0 012-2h5l2 2h9a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" },
  { key: "services", color: "amber", icon: "M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" },
  { key: "issues", color: "rose", icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" },
];

function ResourceMiniBar({ label, value, total }: { label: string; value: number; total?: string }) {
  const color = value > 80 ? "bg-[var(--danger)]" : value > 60 ? "bg-[var(--warning)]" : "bg-[var(--success)]";
  const textColor = value > 80 ? "text-[var(--danger)]" : value > 60 ? "text-[var(--warning)]" : "text-[var(--success)]";
  return (
    <div>
      <div className="flex items-center justify-between mb-0.5">
        <span className="text-2xs text-[var(--text-faint)]">{label}</span>
        <span className={`text-2xs font-semibold ${textColor} font-mono`}>{value}%</span>
      </div>
      <div className="h-1 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
        <div className={`h-full rounded-full ${color} transition duration-500`} style={{ width: `${value}%` }} />
      </div>
      {total && <p className="text-3xs text-[var(--text-faint)] mt-0.5 text-right font-mono">{total}</p>}
    </div>
  );
}

function getTimeAgo(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `${diffH}h ago`;
  const diffD = Math.floor(diffH / 24);
  if (diffD < 30) return `${diffD}d ago`;
  return date.toLocaleDateString();
}

export default function DashboardPage() {
  const { t } = useLocale();
  const { data: stats, isLoading } = useQuery({
    queryKey: ["dashboard"],
    queryFn: dashboardAPI.get,
  });

  const { data: hosts = [] } = useQuery({
    queryKey: ["hosts"],
    queryFn: () => hostsAPI.list(),
  });

  // Compute resource aggregations from hosts with scan data
  const resourceAnalysis = useMemo(() => {
    const scannedHosts = hosts.filter(h => h.has_scan && h.scan_resources);
    if (scannedHosts.length === 0) return null;

    const parsePercent = (s?: string) => {
      if (!s) return null;
      const n = parseFloat(s.replace("%", ""));
      return isNaN(n) ? null : n;
    };

    // Extract numeric value from strings like "8 vCPU", "15Gi", "150G", "7,8Gi"
    const parseResource = (s?: string) => {
      if (!s || typeof s !== "string") return null;
      const cleaned = s.replace(/,/g, ".").trim();
      const match = cleaned.match(/([\d.]+)\s*(.*)/);
      if (!match) return null;
      return { value: parseFloat(match[1]), unit: match[2].trim() };
    };

    type GroupData = { count: number; cpu: number[]; ram: number[]; disk: number[]; totalCpu: string[]; totalRam: string[]; totalDisk: string[] };

    const byHospedagem: Record<string, GroupData> = {};
    const bySituacao: Record<string, GroupData> = {};

    const initGroup = (): GroupData => ({ count: 0, cpu: [], ram: [], disk: [], totalCpu: [], totalRam: [], totalDisk: [] });

    for (const h of scannedHosts) {
      const sr = h.scan_resources!;
      const cpu = parsePercent(sr.cpu_usage);
      const ram = parsePercent(sr.ram_percent);
      const disk = parsePercent(sr.disk_percent);

      const hosp = h.hospedagem || "Unknown";
      if (!byHospedagem[hosp]) byHospedagem[hosp] = initGroup();
      byHospedagem[hosp].count++;
      if (cpu !== null) byHospedagem[hosp].cpu.push(cpu);
      if (ram !== null) byHospedagem[hosp].ram.push(ram);
      if (disk !== null) byHospedagem[hosp].disk.push(disk);
      if (sr.cpu) byHospedagem[hosp].totalCpu.push(sr.cpu);
      if (sr.ram) byHospedagem[hosp].totalRam.push(sr.ram);
      if (sr.storage) byHospedagem[hosp].totalDisk.push(sr.storage);

      const sit = h.situacao || "Unknown";
      if (!bySituacao[sit]) bySituacao[sit] = initGroup();
      bySituacao[sit].count++;
      if (cpu !== null) bySituacao[sit].cpu.push(cpu);
      if (ram !== null) bySituacao[sit].ram.push(ram);
      if (disk !== null) bySituacao[sit].disk.push(disk);
      if (sr.cpu) bySituacao[sit].totalCpu.push(sr.cpu);
      if (sr.ram) bySituacao[sit].totalRam.push(sr.ram);
      if (sr.storage) bySituacao[sit].totalDisk.push(sr.storage);
    }

    const avg = (arr: number[]) => arr.length > 0 ? Math.round(arr.reduce((a, b) => a + b, 0) / arr.length) : 0;

    // Sum up resource values from strings
    const sumResources = (arr: string[]) => {
      let total = 0;
      let unit = "";
      for (const s of arr) {
        const parsed = parseResource(s);
        if (parsed) {
          total += parsed.value;
          if (!unit) unit = parsed.unit;
        }
      }
      if (total === 0) return "";
      return `${Number.isInteger(total) ? total : total.toFixed(1)} ${unit}`;
    };

    const mapGroup = ([name, d]: [string, GroupData]) => ({
      name, count: d.count,
      cpu: avg(d.cpu), ram: avg(d.ram), disk: avg(d.disk),
      totalCpu: sumResources(d.totalCpu),
      totalRam: sumResources(d.totalRam),
      totalDisk: sumResources(d.totalDisk),
    });

    return {
      byHospedagem: Object.entries(byHospedagem).map(mapGroup).sort((a, b) => b.count - a.count),
      bySituacao: Object.entries(bySituacao).map(mapGroup).sort((a, b) => b.count - a.count),
      totalScanned: scannedHosts.length,
    };
  }, [hosts]);

  const statValues = stats
    ? [stats.hosts.total, stats.dns_records, stats.projects, stats.services, stats.open_issues]
    : [0, 0, 0, 0, 0];

  const statLabels = [
    t("dashboard.totalHosts"),
    t("dashboard.totalDns"),
    t("dashboard.totalProjects"),
    t("dashboard.totalServices"),
    t("dashboard.openIssues"),
  ];

  const scanPct = stats && stats.hosts.total > 0
    ? Math.round((stats.hosts.with_scans / stats.hosts.total) * 100)
    : 0;

  const hospedagemEntries = stats?.hosts.by_hospedagem
    ? Object.entries(stats.hosts.by_hospedagem).sort((a, b) => b[1] - a[1])
    : [];

  const recentScans = stats?.recent_scans ?? [];

  return (
    <PageShell>
      <PageHeader title={t("dashboard.title")} />

      {isLoading ? (
        <SkeletonStats />
      ) : stats ? (
        <>
          {/* Stat cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3 md:gap-4 mb-6">
            {statConfig.map((cfg, i) => (
              <StatCard
                key={cfg.key}
                label={statLabels[i]}
                value={statValues[i]}
                icon={cfg.icon}
                color={cfg.color}
                className="stagger-in" style={{ "--i": i } as React.CSSProperties}
              />
            ))}
          </div>

          {/* Insight cards row */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
            {/* Scan Coverage */}
            <Card hover={false} className="stagger-in" style={{ "--i": 5 } as React.CSSProperties}>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-sm font-semibold text-[var(--text-secondary)] font-display">
                  {t("dashboard.scanCoverage")}
                </h2>
                <Icon path={ICON_PATHS.scan} className="w-4 h-4 text-[var(--accent)]" />
              </div>
              <div className="flex items-end gap-3 mb-3">
                <span className="text-2xl font-bold text-[var(--accent)] font-display">
                  {stats.hosts.with_scans}
                </span>
                <span className="text-sm text-[var(--text-muted)] mb-0.5">/ {stats.hosts.total} {t("host.title").toLowerCase()}</span>
              </div>
              <div className="h-2 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
                <div
                  className="h-full rounded-full bg-[var(--accent)] transition duration-700"
                  style={{ width: `${scanPct}%` }}
                />
              </div>
              <p className="text-xs text-[var(--text-faint)] mt-1.5 font-mono">{scanPct}% {t("dashboard.scanned")}</p>
            </Card>

            {/* Alerts */}
            <Card hover={false} className="stagger-in" style={{ "--i": 6 } as React.CSSProperties}>
              {(() => {
                const alertCount = hosts.filter(h => h.alerts && h.alerts.length > 0).length;
                const criticalCount = hosts.filter(h => h.alerts?.some(a => a.level === "critical")).length;
                return (
                  <>
                    <div className="flex items-center justify-between mb-3">
                      <h2 className="text-sm font-semibold text-[var(--text-secondary)] font-display">
                        {t("dashboard.maintenanceAlerts")}
                      </h2>
                      <svg className={`w-4 h-4 ${alertCount > 0 ? "text-[var(--warning)]" : "text-[var(--success)]"}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                      </svg>
                    </div>
                    <div className="flex items-end gap-3 mb-2">
                      <span className={`text-3xl font-bold ${alertCount > 0 ? "text-[var(--warning)]" : "text-[var(--success)]"} font-display`}>
                        {alertCount}
                      </span>
                      {criticalCount > 0 && (
                        <span className="text-xs text-[var(--danger)] font-semibold mb-1">{t("dashboard.criticalCount", { count: String(criticalCount) })}</span>
                      )}
                    </div>
                    <p className="text-xs text-[var(--text-faint)]">
                      {alertCount > 0 ? t("dashboard.hostsNeedAttention") : t("dashboard.allHostsHealthy")}
                    </p>
                  </>
                );
              })()}
            </Card>

            {/* Hosting Distribution */}
            <Card hover={false} className="stagger-in" style={{ "--i": 7 } as React.CSSProperties}>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-sm font-semibold text-[var(--text-secondary)] font-display">
                  {t("dashboard.infrastructure")}
                </h2>
                <Icon path={ICON_PATHS.archiveBox} className="w-4 h-4 text-[var(--purple)]" />
              </div>
              {hospedagemEntries.length > 0 ? (
                <div className="space-y-2">
                  {hospedagemEntries.slice(0, 4).map(([name, count]) => {
                    const pct = Math.round((count / stats.hosts.total) * 100);
                    return (
                      <div key={name}>
                        <div className="flex items-center justify-between text-xs mb-0.5">
                          <span className="text-[var(--text-secondary)] truncate">{name}</span>
                          <span className="text-[var(--text-faint)] ml-2 font-mono">{count}</span>
                        </div>
                        <div className="h-1 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
                          <div
                            className="h-full rounded-full bg-[var(--purple)]/60 transition duration-500"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      </div>
                    );
                  })}
                  {hospedagemEntries.length > 4 && (
                    <p className="text-2xs text-[var(--text-faint)]">+{hospedagemEntries.length - 4} {t("common.more")}</p>
                  )}
                </div>
              ) : (
                <p className="text-xs text-[var(--text-faint)]">{t("dashboard.noHostingData")}</p>
              )}
            </Card>
          </div>

          {/* Bottom section: Recent Scans + Hosts by Status */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
            {/* Recent Scans */}
            <Card hover={false} className="stagger-in" style={{ "--i": 8 } as React.CSSProperties}>
              <SectionHeading variant="section">
                {t("dashboard.recentScans")}
              </SectionHeading>
              {recentScans.length > 0 ? (
                <div className="space-y-2">
                  {recentScans.map((scan) => (
                    <Link
                      key={scan.id}
                      href={`/hosts/${scan.slug}`}
                      className="flex items-center gap-3 p-2 -mx-2 rounded-[var(--radius-md)] hover:bg-[var(--bg-elevated)] transition-colors"
                    >
                      <StatusDot className="bg-[var(--success)]" />
                      <span className="text-sm text-[var(--text-primary)] font-medium truncate font-mono">
                        {scan.nickname}
                      </span>
                      <span className="text-xs text-[var(--text-faint)] ml-auto whitespace-nowrap">
                        {getTimeAgo(scan.scanned_at)}
                      </span>
                    </Link>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-[var(--text-faint)]">{t("dashboard.noScansYet")}</p>
              )}
            </Card>

            {/* Hosts by status */}
            {Object.keys(stats.hosts.by_situacao).length > 0 && (
              <Card hover={false} className="stagger-in" style={{ "--i": 9 } as React.CSSProperties}>
                <SectionHeading variant="section">
                  {t("dashboard.hostsByStatus", { hosts: t("host.title"), status: t("common.status") })}
                </SectionHeading>
                <div className="space-y-5">
                  {Object.entries(stats.hosts.by_situacao).map(([situacao, count]) => {
                    const total = stats.hosts.total || 1;
                    const pct = Math.round(((count as number) / total) * 100);
                    return (
                      <div key={situacao}>
                        <div className="flex items-center justify-between mb-2">
                          <Badge variant="situacao" situacao={situacao} dot>{situacao}</Badge>
                          <span className="text-sm text-[var(--text-secondary)] font-mono">
                            {count as number} <span className="text-[var(--text-faint)]">({pct}%)</span>
                          </span>
                        </div>
                        <div className="h-1.5 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
                          <div
                            className="h-full rounded-full transition duration-700"
                            style={{ width: `${pct}%`, backgroundColor: "var(--accent)" }}
                          />
                        </div>
                      </div>
                    );
                  })}
                </div>
              </Card>
            )}
          </div>

          {/* Resource Analysis by Infrastructure & Situação */}
          {resourceAnalysis && resourceAnalysis.totalScanned > 0 && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {/* Resources by Infrastructure */}
              <Card hover={false} className="stagger-in" style={{ "--i": 10 } as React.CSSProperties}>
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-sm font-semibold text-[var(--text-secondary)] font-display">
                    {t("dashboard.resourcesByInfra")}
                  </h2>
                  <span className="text-2xs text-[var(--text-faint)]">{t("dashboard.avgUsage")}</span>
                </div>
                <div className="space-y-4">
                  {resourceAnalysis.byHospedagem.map((entry) => (
                    <div key={entry.name}>
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-xs text-[var(--text-primary)] font-medium">{entry.name}</span>
                        <span className="text-2xs text-[var(--text-faint)] font-mono">
                          {entry.count} {entry.count === 1 ? t("dashboard.hostSingular") : t("dashboard.hostPlural")}
                        </span>
                      </div>
                      <div className="grid grid-cols-3 gap-2">
                        <ResourceMiniBar label="CPU" value={entry.cpu} total={entry.totalCpu} />
                        <ResourceMiniBar label="RAM" value={entry.ram} total={entry.totalRam} />
                        <ResourceMiniBar label={t("dashboard.diskLabel")} value={entry.disk} total={entry.totalDisk} />
                      </div>
                    </div>
                  ))}
                </div>
              </Card>

              {/* Resources by Situação */}
              <Card hover={false} className="stagger-in" style={{ "--i": 11 } as React.CSSProperties}>
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-sm font-semibold text-[var(--text-secondary)] font-display">
                    {t("dashboard.resourcesBySituacao")}
                  </h2>
                  <span className="text-2xs text-[var(--text-faint)]">{t("dashboard.avgUsage")}</span>
                </div>
                <div className="space-y-4">
                  {resourceAnalysis.bySituacao.map((entry) => (
                    <div key={entry.name}>
                      <div className="flex items-center justify-between mb-2">
                        <Badge variant="situacao" situacao={entry.name} dot>{entry.name}</Badge>
                        <span className="text-2xs text-[var(--text-faint)] font-mono">
                          {entry.count} {entry.count === 1 ? t("dashboard.hostSingular") : t("dashboard.hostPlural")}
                        </span>
                      </div>
                      <div className="grid grid-cols-3 gap-2">
                        <ResourceMiniBar label="CPU" value={entry.cpu} total={entry.totalCpu} />
                        <ResourceMiniBar label="RAM" value={entry.ram} total={entry.totalRam} />
                        <ResourceMiniBar label={t("dashboard.diskLabel")} value={entry.disk} total={entry.totalDisk} />
                      </div>
                    </div>
                  ))}
                </div>
              </Card>
            </div>
          )}
        </>
      ) : null}
    </PageShell>
  );
}
