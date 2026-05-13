"use client";

import { useMemo, useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import type { DockerLogsReport } from "@/lib/api";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import type { Host } from "@/lib/types";
import Button from "@/components/ui/Button";
import BatchOperationShell, { type ScopeOption } from "./BatchOperationShell";
import { useBatchRunner } from "./useBatchRunner";

type Scope = "with_containers" | "all_eligible" | "flagged";

// We collect per-host reports outside the runner because useBatchRunner only
// surfaces success/error. The Map keys on the host slug so the analysis
// panel can render even after the run finishes — Maps survive re-renders
// because we hold a single instance via useState.
type ReportMap = Map<string, DockerLogsReport>;
type HostError = { slug: string; nickname: string; error: string };

export default function BatchDockerLogsModal({
  hosts,
  onClose,
  t,
}: {
  hosts: Host[];
  onClose: () => void;
  t: (key: string) => string;
}) {
  const queryClient = useQueryClient();
  const [concurrency, setConcurrency] = useLocalStorage("hosts.dockerLogsConcurrency", 5);
  const [scope, setScope] = useLocalStorage<Scope>("hosts.dockerLogsScope", "with_containers");
  const runner = useBatchRunner();

  // Reports captured per host. New runs replace previous data so the
  // analysis panel stays in sync with the current scope.
  const [reports, setReports] = useState<ReportMap>(() => new Map());
  const [errors, setErrors] = useState<HostError[]>([]);

  // Eligibility:
  //   "with_containers" — hosts known to have at least one container from
  //                       the last scan. Most precise; skips hosts where
  //                       docker isn't installed.
  //   "all_eligible"    — every host with a credential. Best-effort; the
  //                       backend returns "docker not reachable" cleanly
  //                       for hosts without docker, so it's safe but
  //                       wastes a connection per non-docker host.
  //   "flagged"         — replays the inspection only against hosts that
  //                       previous run flagged warning/critical.
  const eligible = useMemo(() => hosts.filter(h => h.has_password || h.has_key), [hosts]);
  const withContainers = useMemo(() => eligible.filter(h => (h.containers_count || 0) > 0), [eligible]);
  const flagged = useMemo(() => {
    if (reports.size === 0) return [] as Host[];
    return eligible.filter(h => {
      const r = reports.get(h.oficial_slug);
      return r && (r.risk_level === "warning" || r.risk_level === "critical");
    });
  }, [eligible, reports]);

  const effectiveScope: Scope =
    scope === "with_containers" && withContainers.length > 0 ? "with_containers" :
    scope === "flagged" && flagged.length > 0 ? "flagged" :
    "all_eligible";
  const targets =
    effectiveScope === "with_containers" ? withContainers :
    effectiveScope === "flagged" ? flagged :
    eligible;

  const scopeOptions: ScopeOption[] = [
    { key: "with_containers", label: t("host.batchDockerLogsScopeWithContainers"), count: withContainers.length },
    { key: "all_eligible", label: t("host.batchDockerLogsScopeAll"), count: eligible.length },
    { key: "flagged", label: t("host.batchDockerLogsScopeFlagged"), count: flagged.length },
  ];

  const handleStart = useCallback(() => {
    // Stable reference for the captured map: we mutate it inside runOne
    // and update React state with a fresh Map after each write so the
    // analysis panel reflects in-flight results live.
    const collected: ReportMap = new Map();
    const collectedErrors: HostError[] = [];
    setReports(collected);
    setErrors(collectedErrors);

    runner.start({
      hosts: targets,
      concurrency,
      runOne: async (host) => {
        try {
          const res = await sshAPI.dockerLogsInspect(host.oficial_slug);
          if (!res.success) {
            collectedErrors.push({ slug: host.oficial_slug, nickname: host.nickname, error: res.error || "Failed" });
            setErrors([...collectedErrors]);
            return { success: false, error: res.error || "Failed" };
          }
          if (res.report) {
            collected.set(host.oficial_slug, res.report);
            // Push a fresh Map snapshot so memoized children see new data.
            setReports(new Map(collected));
          }
          return { success: true };
        } catch (err) {
          const msg = err instanceof Error ? err.message : "Failed";
          collectedErrors.push({ slug: host.oficial_slug, nickname: host.nickname, error: msg });
          setErrors([...collectedErrors]);
          return { success: false, error: msg };
        }
      },
    }).then(() => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
    });
  }, [targets, concurrency, runner, queryClient]);

  const showAnalysis = reports.size > 0 && !runner.running;

  return (
    <div className="space-y-4">
      <BatchOperationShell
        description={t("host.batchDockerLogsDesc")}
        scopeLabel={t("host.scanScope")}
        scope={effectiveScope}
        onScopeChange={(s) => setScope(s as Scope)}
        scopeOptions={scopeOptions}
        concurrency={concurrency}
        onConcurrencyChange={setConcurrency}
        concurrencyLabel={t("host.scanConcurrency")}
        targetHosts={targets}
        progress={runner.progress}
        running={runner.running}
        doneCount={runner.doneCount}
        successCount={runner.successCount}
        failedCount={runner.failedCount}
        startLabel={t("host.batchDockerLogsStart")}
        rerunLabel={t("host.rescan")}
        stopLabel={t("host.stopScan")}
        cancelLabel={t("common.cancel")}
        progressLabel={t("host.scanProgress")}
        runningLabel={t("host.batchRunning")}
        emptyHint={targets.length === 0 ? t("host.batchDockerLogsEmpty") : undefined}
        onStart={handleStart}
        onStop={runner.stop}
        onClose={onClose}
      />

      {showAnalysis && (
        <FleetAnalysisPanel
          hosts={hosts}
          reports={reports}
          errors={errors}
          t={t}
        />
      )}
    </div>
  );
}

/* ─── Fleet analysis ─── */

function FleetAnalysisPanel({
  hosts,
  reports,
  errors,
  t,
}: {
  hosts: Host[];
  reports: ReportMap;
  errors: HostError[];
  t: (key: string) => string;
}) {
  const hostBySlug = useMemo(() => {
    const m = new Map<string, Host>();
    for (const h of hosts) m.set(h.oficial_slug, h);
    return m;
  }, [hosts]);

  // Aggregate stats across the fleet. We tally:
  //   - total bytes consumed by docker logs across all reporting hosts
  //   - per-risk counts so the operator sees the verdict distribution
  //   - "top offenders" sorted by total log size (descending)
  //   - "unbounded" containers across the fleet — these are the candidates
  //     for `Apply Docker Log Rotation` to fix
  const stats = useMemo(() => {
    let totalBytes = 0;
    let totalUnbounded = 0;
    let totalContainers = 0;
    const byRisk: Record<string, number> = { critical: 0, warning: 0, ok: 0 };
    const rows: Array<{ host: Host; report: DockerLogsReport }> = [];
    for (const [slug, report] of reports.entries()) {
      const host = hostBySlug.get(slug);
      if (!host) continue;
      totalBytes += report.total_log_bytes || 0;
      totalUnbounded += report.unbounded_containers || 0;
      totalContainers += (report.containers?.length || 0);
      byRisk[report.risk_level] = (byRisk[report.risk_level] || 0) + 1;
      rows.push({ host, report });
    }
    rows.sort((a, b) => (b.report.total_log_bytes || 0) - (a.report.total_log_bytes || 0));
    return { totalBytes, totalUnbounded, totalContainers, byRisk, rows };
  }, [reports, hostBySlug]);

  return (
    <div className="space-y-3 border-t border-[var(--border-subtle)] pt-4">
      <h4 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">
        {t("host.batchDockerLogsAnalysis")}
      </h4>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
        <Stat label={t("host.batchDockerLogsTotalSize")} value={humanizeBytes(stats.totalBytes)} mono />
        <Stat label={t("host.batchDockerLogsTotalContainers")} value={String(stats.totalContainers)} mono />
        <Stat label={t("host.batchDockerLogsUnbounded")} value={String(stats.totalUnbounded)} mono
              tone={stats.totalUnbounded > 0 ? "amber" : "ok"} />
        <Stat label={t("host.batchDockerLogsHostsScanned")} value={String(reports.size)} mono />
      </div>

      <div className="flex flex-wrap items-center gap-2 text-[11px]">
        <RiskChip risk="critical" count={stats.byRisk.critical || 0} />
        <RiskChip risk="warning" count={stats.byRisk.warning || 0} />
        <RiskChip risk="ok" count={stats.byRisk.ok || 0} />
        {errors.length > 0 && (
          <span className="px-2 py-0.5 rounded-full bg-[var(--bg-elevated)] text-[var(--text-faint)] border border-[var(--border-subtle)]">
            {errors.length} {t("host.batchDockerLogsErrored")}
          </span>
        )}
      </div>

      {stats.rows.length > 0 && (
        <div className="max-h-72 overflow-y-auto border border-[var(--border-subtle)] rounded-[var(--radius-md)] divide-y divide-[var(--border-subtle)]/50">
          <div className="grid grid-cols-[1fr_auto_auto_auto] items-center gap-3 px-3 py-1.5 text-[10px] uppercase tracking-wider text-[var(--text-faint)] bg-[var(--bg-elevated)]/30">
            <span>{t("host.title")}</span>
            <span className="text-right">{t("host.batchDockerLogsTotalSize")}</span>
            <span className="text-right">{t("host.batchDockerLogsUnbounded")}</span>
            <span className="text-right">Risk</span>
          </div>
          {stats.rows.map(({ host, report }) => (
            <FleetRow key={host.oficial_slug} host={host} report={report} />
          ))}
        </div>
      )}

      {errors.length > 0 && (
        <details className="text-xs">
          <summary className="cursor-pointer text-[var(--text-muted)] hover:text-[var(--text-secondary)]">
            {errors.length} {t("host.batchDockerLogsHostsErrored")}
          </summary>
          <div className="mt-2 space-y-1">
            {errors.map((e) => (
              <div key={e.slug} className="px-2 py-1 rounded text-[11px] bg-red-500/5 border border-red-500/20">
                <span className="font-medium text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>
                  {e.nickname}
                </span>
                <span className="ml-2 text-red-400">{e.error}</span>
              </div>
            ))}
          </div>
        </details>
      )}
    </div>
  );
}

function FleetRow({ host, report }: { host: Host; report: DockerLogsReport }) {
  const riskClass =
    report.risk_level === "critical"
      ? "bg-red-500/15 text-red-300 light:text-red-800 border-red-500/40"
      : report.risk_level === "warning"
      ? "bg-amber-500/15 text-amber-300 light:text-amber-800 border-amber-500/40"
      : "bg-emerald-500/15 text-emerald-300 light:text-emerald-800 border-emerald-500/40";
  return (
    <div className="grid grid-cols-[1fr_auto_auto_auto] items-center gap-3 px-3 py-1.5 text-xs">
      <span className="text-[var(--text-primary)] font-medium truncate" style={{ fontFamily: "var(--font-mono)" }} title={host.nickname}>
        {host.nickname}
      </span>
      <span className="text-right text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>
        {humanizeBytes(report.total_log_bytes)}
      </span>
      <span
        className={`text-right ${report.unbounded_containers > 0 ? "text-amber-400" : "text-[var(--text-faint)]"}`}
        style={{ fontFamily: "var(--font-mono)" }}
        title={report.unbounded_containers > 0 ? "Containers without rotation" : "All containers have rotation"}
      >
        {report.unbounded_containers}
      </span>
      <span className={`text-right inline-flex items-center justify-end px-1.5 py-0.5 rounded-full text-[10px] border ${riskClass}`}>
        {report.risk_level}
      </span>
    </div>
  );
}

function Stat({ label, value, mono, tone }: { label: string; value: string; mono?: boolean; tone?: "ok" | "amber" | "red" }) {
  const valueClass =
    tone === "amber" ? "text-amber-400"
      : tone === "red" ? "text-red-400"
      : "text-[var(--text-primary)]";
  return (
    <div>
      <span className="block text-[10px] text-[var(--text-faint)] uppercase tracking-wider">{label}</span>
      <span className={`font-medium ${valueClass}`} style={mono ? { fontFamily: "var(--font-mono)" } : undefined}>
        {value}
      </span>
    </div>
  );
}

function RiskChip({ risk, count }: { risk: "critical" | "warning" | "ok"; count: number }) {
  const cls =
    risk === "critical"
      ? "bg-red-500/15 text-red-300 light:text-red-800 border-red-500/40"
      : risk === "warning"
      ? "bg-amber-500/15 text-amber-300 light:text-amber-800 border-amber-500/40"
      : "bg-emerald-500/15 text-emerald-300 light:text-emerald-800 border-emerald-500/40";
  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full border ${cls}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${
        risk === "critical" ? "bg-red-400"
          : risk === "warning" ? "bg-amber-400"
          : "bg-emerald-400"
      }`} />
      {count} {risk}
    </span>
  );
}

function humanizeBytes(n: number): string {
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
