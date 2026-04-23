"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { grafanaAPI, alertSettingsAPI } from "@/lib/api";
import KpiGrid from "@/components/inventory/KpiGrid";

interface Props {
  slug: string;
}

// Thresholds we fall back to if the backend call for alert settings fails —
// same numbers the backend ships with in migration v29.
const DEFAULT_CRITICAL = 80;
const DEFAULT_WARNING = 60;

export default function HostLiveKpis({ slug }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ["host-live-metrics", slug],
    queryFn: () => grafanaAPI.hostLiveMetrics(slug),
    refetchInterval: 15_000,
    staleTime: 10_000,
    retry: false,
  });

  const { data: thresholds } = useQuery({
    queryKey: ["alert-thresholds"],
    queryFn: alertSettingsAPI.get,
    staleTime: 300_000,
    retry: false,
  });

  const critical = thresholds?.resource_critical ?? DEFAULT_CRITICAL;
  const warning = thresholds?.resource_warning ?? DEFAULT_WARNING;

  const kpis = useMemo(() => {
    if (!data) return [];
    return [
      {
        label: "CPU",
        value: formatPct(data.cpu_pct),
        color: pickColor(data.cpu_pct, warning, critical),
        icon: "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z",
      },
      {
        label: "RAM",
        value: formatPct(data.ram_pct),
        color: pickColor(data.ram_pct, warning, critical),
        icon: "M4 6h16M4 12h16M4 18h16",
      },
      {
        label: "Disk",
        value: formatPct(data.disk_pct),
        color: pickColor(data.disk_pct, warning, critical),
        icon: "M5 12V7a2 2 0 012-2h10a2 2 0 012 2v5m-14 0v5a2 2 0 002 2h10a2 2 0 002-2v-5M5 12h14",
      },
      {
        label: "Load 1m",
        value: data.load_1m === null || data.load_1m === undefined ? "—" : data.load_1m.toFixed(2),
        color: "cyan",
        icon: "M13 10V3L4 14h7v7l9-11h-7z",
      },
      {
        label: "Uptime",
        value: formatUptime(data.uptime_seconds),
        color: data.host_up === false ? "red" : "emerald",
        icon: "M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z",
      },
    ];
  }, [data, warning, critical]);

  if (!isLoading && data && !data.enabled) {
    return null; // integration off — don't render anything
  }

  if (!isLoading && data && !data.configured) {
    return (
      <div className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-4 py-3 text-xs text-[var(--text-muted)]">
        Live KPIs are unavailable — set the Prometheus datasource UID in Settings → Integrations → Grafana, and ensure an API token is stored.
      </div>
    );
  }

  if (isLoading && !data) {
    return (
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 mb-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="h-[88px] rounded-[var(--radius-lg)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] animate-pulse" />
        ))}
      </div>
    );
  }

  if (!data) return null;

  return (
    <div>
      <KpiGrid kpis={kpis} columns={5} />
      {data.warnings && data.warnings.length > 0 && (
        <details className="-mt-3 mb-3 text-[10px] text-[var(--text-faint)]">
          <summary className="cursor-pointer hover:text-[var(--text-muted)]">
            {data.warnings.length} metric{data.warnings.length === 1 ? "" : "s"} failed to fetch
          </summary>
          <ul className="mt-1 space-y-0.5 pl-4 list-disc">
            {data.warnings.map((w, i) => (
              <li key={i} className="font-mono break-all">{w}</li>
            ))}
          </ul>
        </details>
      )}
      {data.host_up === false && (
        <div className="-mt-2 mb-3 rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 px-3 py-2 text-[11px] text-red-300">
          Prometheus reports this host as <strong>down</strong> (up=0). The tiles above show the last values Prometheus scraped.
        </div>
      )}
    </div>
  );
}

function formatPct(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—";
  return `${Math.round(v)}%`;
}

function pickColor(v: number | null | undefined, warning: number, critical: number): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "cyan";
  if (v >= critical) return "red";
  if (v >= warning) return "amber";
  return "emerald";
}

function formatUptime(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || seconds <= 0) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}
