"use client";

import SectionCard from "@/components/ui/SectionCard";
import StatusDot from "@/components/ui/StatusDot";
import Badge from "@/components/ui/Badge";
import type { VMInfoType, CronInfo, CronJob } from "@/lib/api";
import { SubBlock, type T } from "./shared";

/** Installed packages and scheduled jobs (cron, anacron, systemd timers). */
export default function ScanPackages({ info, t }: { info: VMInfoType; t: T }) {
  const packages = info.installed_packages ?? [];
  const legacyCron = !info.cron ? info.cron_jobs ?? [] : [];
  const empty = packages.length === 0 && !info.cron && legacyCron.length === 0;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.packages")} empty={empty ? t("scan.pane.empty.packages") : undefined}>
      {info.cron && <Cron cron={info.cron} t={t} />}
      {/* Scans persisted before the structured collector carry a raw blob. */}
      {legacyCron.length > 0 && (
        <SubBlock title={t("scan.cronJobs")}>
          <pre className="text-xs text-[var(--text-secondary)] whitespace-pre-wrap break-all font-mono">{legacyCron.join("\n")}</pre>
        </SubBlock>
      )}
      {packages.length > 0 && (
        <SubBlock title={t("scan.installedPackages")} aside={packages.length}>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6">
            {packages.map((pkg, i) => (
              <div key={i} className="flex items-center justify-between gap-3 py-1 border-b border-[var(--border-subtle)]/50">
                <span className="text-xs text-[var(--text-primary)] truncate">{pkg.name}</span>
                <span className="text-xs text-[var(--text-muted)] font-mono shrink-0">{pkg.version}</span>
              </div>
            ))}
          </div>
        </SubBlock>
      )}
    </SectionCard>
  );
}

// "System" = anything package-manager / sysadmin-owned (/etc/crontab,
// /etc/cron.d, drop-in dirs, anacron, timers); "user" = per-user crontabs —
// what the OS schedules vs what somebody added by hand.
function Cron({ cron, t }: { cron: CronInfo; t: T }) {
  const jobs = cron.jobs ?? [];
  const system = jobs.filter((j) => !j.source.startsWith("user:"));
  const user = jobs.filter((j) => j.source.startsWith("user:"));
  const state = !cron.daemon_installed && !cron.daemon_active ? "missing" : cron.daemon_active ? "active" : "inactive";

  return (
    <SubBlock title={t("scan.cronJobs")} aside={`${jobs.length} ${jobs.length === 1 ? t("scan.cron.jobSingular") : t("scan.cron.jobPlural")}`}>
      <div className="flex flex-wrap items-center gap-2 mb-3 text-2xs">
        <span className="inline-flex items-center gap-1.5 text-[var(--text-secondary)]">
          <StatusDot size="xs" color={state === "active" ? "success" : state === "inactive" ? "warning" : "muted"} />
          {cron.daemon_name || "cron"}: {t(state === "active" ? "scan.cron.daemonActive" : state === "inactive" ? "scan.cron.daemonInactive" : "scan.cron.daemonMissing")}
        </span>
        {cron.daemon_installed && (
          <span className="text-[var(--text-muted)]">{cron.daemon_enabled ? t("scan.cron.daemonEnabledAtBoot") : t("scan.cron.daemonNotEnabled")}</span>
        )}
      </div>
      {jobs.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)]">{t("scan.cron.noJobs")}</p>
      ) : (
        <div className="space-y-3">
          {system.length > 0 && <CronGroup title={t("scan.cron.systemJobs")} jobs={system} t={t} />}
          {user.length > 0 && <CronGroup title={t("scan.cron.userJobs")} jobs={user} t={t} />}
        </div>
      )}
    </SubBlock>
  );
}

function CronGroup({ title, jobs, t }: { title: string; jobs: CronJob[]; t: T }) {
  return (
    <div>
      <span className="text-2xs font-semibold text-[var(--text-faint)] block mb-1">{title} <span className="opacity-60">({jobs.length})</span></span>
      <div className="divide-y divide-[var(--border-subtle)]/60">
        {jobs.map((j, i) => {
          const source = j.source.startsWith("user:") ? j.source.slice(5) : j.source.startsWith("/etc/cron.d/") ? j.source.slice(12) : j.source;
          return (
            <div key={i} className={`flex flex-wrap items-center gap-2 py-1.5 text-xs ${j.disabled ? "opacity-50" : ""}`}>
              <span className="w-28 shrink-0 font-mono text-[var(--text-secondary)] truncate" title={j.kind === "timer" ? t("scan.cron.timerUnit") : t("scan.cron.schedule")}>{j.schedule || "–"}</span>
              {j.user && <span className="shrink-0 font-mono text-[var(--info)]" title={t("scan.cron.runAs")}>{j.user}</span>}
              <span className="flex-1 min-w-0 truncate font-mono text-[var(--text-primary)]" title={j.command}>{j.command || "–"}</span>
              {j.disabled && <Badge>{t("scan.cron.disabled")}</Badge>}
              {j.next_run && <span className="shrink-0 text-2xs text-[var(--text-muted)]" title={t("scan.cron.nextRun")}>→ {j.next_run}</span>}
              <span className="shrink-0 text-2xs text-[var(--text-faint)] font-mono" title={j.source}>{source}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
