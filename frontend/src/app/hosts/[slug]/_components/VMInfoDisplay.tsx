"use client";

import { useMemo } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { UsageBar } from "./UsageBar";
import { ContainersList } from "./SortableResourceList";
import { formatUptime, parseLoginEntry, formatLoginDate, portIcon, parseServiceRow } from "@/lib/utils";
import type { VMInfoType, ProcessDetail, PortOwner, CronInfo, CronJob, Agent, DiscoveredService, ResourceUsageSnapshot, ResourceProcess, ResourceDiskItem } from "@/lib/api";

// Login users that aren't real accounts — these come from `last`'s wtmp
// rollover and reboot bookkeeping. Filter them out before grouping logins
// per user so the Remote Users cards only show meaningful entries.
const NON_USER_LOGIN_TOKENS = new Set(["reboot", "shutdown", "wtmp", "runlevel", ""]);

export default function VMInfoDisplay({ info, locale, compact }: { info: VMInfoType; locale: string; compact?: boolean }) {
  const { t } = useLocale();
  const gridCols = compact
    ? "grid-cols-1 sm:grid-cols-2"
    : "grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4";

  return (
    <div className="space-y-6 text-sm">
      {/* Resources */}
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.resources")}</h3>
      <Card hover={false}>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {info.cpu_usage && <UsageBar label={t("vm.cpu")} total={info.cpu} used={info.cpu_usage} percent={info.cpu_usage} />}
          {info.ram_percent && <UsageBar label={t("vm.ram")} total={info.ram} used={info.ram_used} percent={info.ram_percent} />}
          {info.disk_percent && <UsageBar label={t("vm.disk")} total={info.storage} used={info.storage_used} percent={info.disk_percent} />}
        </div>
        {/* Top consumers — captured during the scan so the operator can
            answer "the host is at 87% RAM, who is using it?" without
            opening another terminal. Sublists are rendered side-by-side
            on wide viewports and stacked on narrow ones. */}
        {info.resource_top && (
          (info.resource_top.top_cpu?.length || 0) +
          (info.resource_top.top_mem?.length || 0) +
          (info.resource_top.top_disk?.length || 0) > 0
        ) && (
          <div className="mt-4 pt-4 border-t border-[var(--border-subtle)]/50">
            <ResourceTopPanel snapshot={info.resource_top} t={t} />
          </div>
        )}
      </Card>

      {/* System + Ports (merged) */}
      {(() => {
        const uptime = info.uptime ? formatUptime(info.uptime, locale) : "";
        const rows = [info.os && [t("vm.os"), info.os], info.kernel && [t("vm.kernel"), info.kernel], uptime && [t("vm.uptime"), uptime], info.hostname_remote && [t("vm.hostname"), info.hostname_remote], info.public_ip && [t("vm.publicIp"), info.public_ip], info.load_avg && [t("vm.loadAvg"), info.load_avg], info.logged_users && [t("vm.usersOnline"), info.logged_users], info.swap_total && [t("vm.swap"), `${info.swap_total} (used: ${info.swap_used})`]].filter(Boolean) as [string, string][];
        return rows.length > 0 ? (
          <>
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.system")}</h3>
          <Card hover={false}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2">
              {rows.map(([label, value]) => (
                <div key={label} className="flex justify-between py-1.5 border-b border-[var(--border-subtle)]/50 last:border-0">
                  <span className="text-xs text-[var(--text-muted)]">{label}</span>
                  <span className="text-xs text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{value}</span>
                </div>
              ))}
            </div>
            {info.ports?.length > 0 && (() => {
              const ownerByPort = new Map<number, PortOwner>();
              for (const po of info.port_owners ?? []) ownerByPort.set(po.port, po);

              // Badge color by owner type so the panel scans at a glance.
              // Each variant pairs a pale tint for both themes with a text
              // shade dark enough to read on light backgrounds (via light:)
              // and bright enough to read on the dark navy surface by
              // default. Border opacity bumps to 40% so the chip keeps a
              // visible edge over low-contrast light backgrounds.
              const ownerClasses = (ownerType?: string): string => {
                switch (ownerType) {
                  case "container": return "bg-cyan-500/15 text-cyan-200 light:text-cyan-800 border-cyan-500/40";
                  case "nginx": return "bg-fuchsia-500/15 text-fuchsia-200 light:text-fuchsia-800 border-fuchsia-500/40";
                  case "docker": return "bg-blue-500/15 text-blue-200 light:text-blue-800 border-blue-500/40";
                  // "agent" and "service" are back-filled from the
                  // catalogs when ss couldn't read the owning process
                  // (Zabbix on :10050, PostgreSQL on :5432, MongoDB on
                  // :27017 — each running under a dedicated user).
                  // Violet for agents, emerald for application services
                  // so the operator can distinguish "what's monitoring
                  // me" from "what am I actually running" at a glance.
                  case "agent": return "bg-violet-500/15 text-violet-200 light:text-violet-800 border-violet-500/40";
                  case "service": return "bg-emerald-500/15 text-emerald-200 light:text-emerald-800 border-emerald-500/40";
                  case "process": return "bg-[var(--bg-elevated)] text-[var(--text-secondary)] border-[var(--border-default)]";
                  default: return "bg-[var(--bg-elevated)] text-[var(--text-secondary)] border-[var(--border-default)]";
                }
              };

              return (
                <div className="py-1.5">
                  <span className="text-xs text-[var(--text-muted)] block mb-2">{t("scan.listeningPorts")}</span>
                  <div className="flex flex-wrap gap-1.5">
                    {info.ports.map((p, i) => {
                      const parts = p.split(/\s+/);
                      const portStr = parts[0] || "";
                      const portNum = parseInt(portStr, 10);
                      const rawProc = parts.slice(1).join(" ");
                      const owner = Number.isNaN(portNum) ? undefined : ownerByPort.get(portNum);

                      const displayName = owner?.owner_name || rawProc;
                      const tooltipParts: string[] = [];
                      if (owner?.owner_type) tooltipParts.push(owner.owner_type);
                      if (owner?.target) tooltipParts.push(owner.target);
                      if (rawProc && rawProc !== owner?.owner_name) tooltipParts.push(`proc=${rawProc}`);
                      const tooltip = tooltipParts.length > 0 ? tooltipParts.join(" · ") : rawProc;

                      return (
                        <span
                          key={i}
                          className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full border text-xs ${ownerClasses(owner?.owner_type)}`}
                          title={tooltip}
                        >
                          {portIcon(p)}
                          <span style={{ fontFamily: "var(--font-mono)" }}>:{portStr}</span>
                          {displayName && (
                            <span className="text-[10px] opacity-80 truncate max-w-[10rem]" style={{ fontFamily: "var(--font-mono)" }}>
                              {owner?.owner_type === "nginx" && owner.target ? `nginx → ${owner.target}` : displayName}
                            </span>
                          )}
                        </span>
                      );
                    })}
                  </div>
                </div>
              );
            })()}
            {/* Legacy flat rendering — only used when the scan predates the
                per-user discovery added below. Once remote_users is present,
                the richer "Remote Users" card takes over. */}
            {info.ssh_keys && info.ssh_keys.length > 0 && !(info.remote_users && info.remote_users.length > 0) && (() => {
              const authKeys = info.ssh_keys.filter(k => k.source === "authorized_keys");
              const privKeys = info.ssh_keys.filter(k => k.source !== "authorized_keys");
              const renderKey = (k: typeof info.ssh_keys[0], i: number) => (
                <div key={i} className="flex items-center gap-2 text-xs">
                  <svg className={`w-3.5 h-3.5 shrink-0 ${k.managed ? "text-emerald-400" : "text-[var(--text-faint)]"}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                  </svg>
                  <span className="text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{k.name}</span>
                  <span className="text-[var(--text-faint)]">{k.type}</span>
                  {k.managed ? (
                    <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-emerald-500/15 text-[10px] text-emerald-400 border border-emerald-500/20">
                      {k.managed_name || t("scan.managed")}
                    </span>
                  ) : (
                    <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-amber-500/15 text-[10px] text-amber-400 border border-amber-500/20">
                      {t("scan.unmanaged")}
                    </span>
                  )}
                  <span className="text-[var(--text-muted)] truncate hidden sm:inline ml-auto" style={{ fontFamily: "var(--font-mono)" }}>{k.fingerprint}</span>
                </div>
              );
              return (
                <div className="py-1.5 space-y-3">
                  {authKeys.length > 0 && (
                    <div>
                      <span className="text-xs text-[var(--text-muted)] block mb-2">{t("scan.authorizedKeys")}</span>
                      <div className="space-y-1">{authKeys.map(renderKey)}</div>
                    </div>
                  )}
                  {privKeys.length > 0 && (
                    <div>
                      <span className="text-xs text-[var(--text-muted)] block mb-2">{t("scan.sshKeys")}</span>
                      <div className="space-y-1">{privKeys.map(renderKey)}</div>
                    </div>
                  )}
                </div>
              );
            })()}
          </Card>
          </>
        ) : null;
      })()}

      {/* Remote Users + their keys (post-per-user-scan). Each user gets its
          own card with authorized_keys and private keys nested beneath it. */}
      {info.remote_users && info.remote_users.length > 0 && (() => {
        const allKeys = info.ssh_keys ?? [];
        type Key = (typeof allKeys)[number];
        const keysByUser = new Map<string, Key[]>();
        for (const k of allKeys) {
          if (!k.user) continue;
          const list = keysByUser.get(k.user) ?? [];
          list.push(k);
          keysByUser.set(k.user, list);
        }
        // Orphan keys — entries the scan discovered without a user tag (legacy
        // or unparseable rows). Shown under a synthetic group so nothing is
        // silently dropped.
        const orphanKeys = allKeys.filter((k) => !k.user);

        // Per-user logins. Modern scans carry the grouped slice on each
        // RemoteUserInfo (server-side, last 5 per user, bookkeeping rows
        // already filtered). Legacy scans only have the flat
        // info.last_logins []string — for those we fall back to grouping
        // client-side, which is best-effort because the legacy capture
        // window is narrower (top-N total, not per-user).
        const hasStructuredLogins = (info.remote_users ?? []).some((u) => u.last_logins && u.last_logins.length > 0);
        const loginsByUser = new Map<string, { from: string; when: string }[]>();
        if (!hasStructuredLogins) {
          for (const raw of info.last_logins ?? []) {
            const { user, from, when } = parseLoginEntry(raw);
            if (NON_USER_LOGIN_TOKENS.has(user)) continue;
            const list = loginsByUser.get(user) ?? [];
            if (list.length < 5) list.push({ from, when });
            loginsByUser.set(user, list);
          }
        }

        // Key icon color encodes managed status (green = known to the DB,
        // faint gray = unmanaged). The old "managed"/"unmanaged" text badge
        // was redundant with the color; we only show a chip now when there's
        // unique info to add — the managed_name of a matched DB key.
        const renderKey = (k: Key, i: number) => (
          <div key={i} className="flex gap-2 text-xs">
            <svg
              className={`w-3.5 h-3.5 shrink-0 mt-0.5 ${k.managed ? "text-emerald-400" : "text-[var(--text-faint)]"}`}
              fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
              aria-label={k.managed ? t("scan.managed") : t("scan.unmanaged")}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
            </svg>
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                <span className="text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>
                  {k.name || "\u2014"}
                </span>
                <span className="text-[10px] text-[var(--text-faint)] uppercase tracking-wider">{k.type}</span>
              </div>
              <div
                className="text-[10px] text-[var(--text-muted)] break-all leading-snug"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {k.fingerprint}
              </div>
              {/* Reserve the badge slot even for unmanaged keys so rows
                  with/without a managed_name align vertically. An invisible
                  chip keeps the height but doesn't render text/border. */}
              <div className="mt-1 h-[18px]">
                {k.managed && k.managed_name ? (
                  <span className="inline-block px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-[10px] text-emerald-400 border border-emerald-500/20">
                    {k.managed_name}
                  </span>
                ) : (
                  <span className="inline-block h-[18px]" aria-hidden="true" />
                )}
              </div>
            </div>
          </div>
        );

        return (
          <>
            <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.remoteUsers")}</h3>
            {/* auto-rows-fr forces every row to share the tallest row's
                height, so cards without keys don't collapse and the grid
                stays visually aligned across rows. */}
            <div className={`grid ${gridCols} gap-2 auto-rows-fr`}>
              {info.remote_users.map((u) => {
                const userKeys = keysByUser.get(u.name) ?? [];
                const authKeys = userKeys.filter((k) => k.source === "authorized_keys");
                const privKeys = userKeys.filter((k) => k.source !== "authorized_keys");
                const userLogins = (u.last_logins && u.last_logins.length > 0)
                  ? u.last_logins
                  : (loginsByUser.get(u.name) ?? []);
                return (
                  <Card key={u.name} hover={false} className="!p-3 flex flex-col h-full">
                    <div className="flex items-center gap-2 mb-1">
                      <svg
                        className={`w-3.5 h-3.5 shrink-0 ${u.is_current ? "text-cyan-400" : "text-[var(--text-faint)]"}`}
                        fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}
                        aria-label={u.is_current ? t("scan.userCurrent") : undefined}
                      >
                        <title>{u.is_current ? t("scan.userCurrent") : u.name}</title>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                      </svg>
                      <span className="text-sm font-medium text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{u.name}</span>
                      {!u.has_login && (
                        <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-[var(--bg-elevated)] text-[10px] text-[var(--text-faint)] border border-[var(--border-subtle)]">
                          {t("scan.userNoLogin")}
                        </span>
                      )}
                      {u.password_status === "P" && (
                        <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-amber-500/10 text-[10px] text-amber-400 light:text-amber-700 border border-amber-500/30" title={t("scan.userPasswordSetTooltip")}>
                          {t("scan.userPasswordSet")}
                        </span>
                      )}
                      {u.password_status === "L" && (
                        <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-[var(--bg-elevated)] text-[10px] text-[var(--text-faint)] border border-[var(--border-subtle)]" title={t("scan.userPasswordLockedTooltip")}>
                          {t("scan.userPasswordLocked")}
                        </span>
                      )}
                      {u.password_status === "NP" && (
                        <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-[10px] text-emerald-400 light:text-emerald-800 border border-emerald-500/30" title={t("scan.userPasswordNoneTooltip")}>
                          {t("scan.userPasswordNone")}
                        </span>
                      )}
                      <span
                        className="ml-auto shrink-0 px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[10px] text-[var(--text-faint)] border border-[var(--border-subtle)]"
                        style={{ fontFamily: "var(--font-mono)" }}
                      >
                        uid {u.uid}
                      </span>
                    </div>
                    {u.home && (
                      <p className="text-[10px] text-[var(--text-muted)] break-all leading-relaxed line-clamp-2" style={{ fontFamily: "var(--font-mono)" }}>
                        {u.home}
                      </p>
                    )}
                    {/* Divider sits directly under the home path (no gap)
                        and extends edge-to-edge through the card padding, so
                        every card has the same visual rhythm regardless of
                        how much key content follows. */}
                    <div className="-mx-3 mt-2 border-t border-[var(--border-subtle)]/50" />
                    {userKeys.length === 0 ? (
                      <p className="text-[10px] text-[var(--text-faint)] italic mt-auto pt-2">
                        {t("scan.userNoKeys")}
                      </p>
                    ) : (
                      <div className="pt-2 space-y-2">
                        {authKeys.length > 0 && (
                          <div>
                            <span className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider block mb-1">
                              {t("scan.authorizedKeys")} <span className="text-[var(--text-faint)]">({authKeys.length})</span>
                            </span>
                            <div className="space-y-1">{authKeys.map(renderKey)}</div>
                          </div>
                        )}
                        {privKeys.length > 0 && (
                          <div>
                            <span className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider block mb-1">
                              {t("scan.sshKeys")} <span className="text-[var(--text-faint)]">({privKeys.length})</span>
                            </span>
                            <div className="space-y-1">{privKeys.map(renderKey)}</div>
                          </div>
                        )}
                      </div>
                    )}
                    {userLogins.length > 0 && (
                      <div className="pt-2">
                        <span className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider block mb-1">
                          {t("scan.userLastLogins")} <span className="text-[var(--text-faint)]">({userLogins.length})</span>
                        </span>
                        <div className="space-y-0.5">
                          {userLogins.map((l, i) => (
                            <div
                              key={i}
                              className="flex items-baseline gap-2 text-[10px] text-[var(--text-secondary)] leading-snug"
                              style={{ fontFamily: "var(--font-mono)" }}
                            >
                              <span className="truncate text-[var(--text-primary)]" title={l.from}>{l.from || "—"}</span>
                              <span className="ml-auto shrink-0 text-[var(--text-muted)]">{formatLoginDate(l.when, locale)}</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </Card>
                );
              })}
              {orphanKeys.length > 0 && (
                <Card hover={false} className="!p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-sm font-semibold text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{t("scan.userOrphanKeys")}</span>
                  </div>
                  <div className="space-y-1">{orphanKeys.map(renderKey)}</div>
                </Card>
              )}
            </div>
          </>
        );
      })()}

      {/* Running Services — unified cards */}
      <ProcessCards info={info} t={t} gridCols={gridCols} />

      {/* Docker Containers */}
      {info.container_stats?.length > 0 && (
        <ContainersList
          stats={info.container_stats}
          parsedContainers={info.parsed_containers ?? []}
          title={t("scan.dockerContainers")}
        />
      )}

      {/* Management agents — well-known monitoring/security/cloud agents
          detected by cross-referencing systemd units, packages, processes
          and listen ports. Renders before generic systemd services so the
          operator sees "what's watching this host?" up front. */}
      {info.agents && info.agents.length > 0 && (
        <AgentsCard agents={info.agents} t={t} />
      )}

      {/* Service inventory — unified taxonomy of application services
          (web, db, cache, queue, …) classified into typed buckets. Each
          row collapses systemd + process + package + port + container
          signals into a single entry, so nginx-managed-by-systemd is no
          longer split between the Systemd and Running Services panels. */}
      {info.service_inventory && info.service_inventory.length > 0 && (
        <ServiceInventoryCard services={info.service_inventory} t={t} />
      )}

      {/* Systemd Services — legacy panel. Hidden when service_inventory is
          present (the unified card supersedes it); kept for backward compat
          with scans persisted before the inventory was introduced. */}
      {!info.service_inventory && info.systemd_services && info.systemd_services.length > 0 && (
        <>
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.systemdServices")}</h3>
          <Card hover={false}>
            <div className="flex flex-wrap gap-1.5">
              {info.systemd_services.map((svc, i) => (
                <span
                  key={i}
                  className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-[10px] border ${
                    svc.is_native
                      ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                      : "bg-sky-500/10 text-sky-400 border-sky-500/20"
                  }`}
                  title={svc.description || svc.unit}
                >
                  <span className={`w-1.5 h-1.5 rounded-full ${svc.is_native ? "bg-emerald-400" : "bg-sky-400"}`} />
                  {svc.unit.replace(".service", "")}
                  <span className="opacity-60">({svc.is_native ? t("scan.native") : t("scan.containerManaged")})</span>
                </span>
              ))}
            </div>
          </Card>
        </>
      )}

      {/* Installed Packages */}
      {info.installed_packages && info.installed_packages.length > 0 && (
        <>
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.installedPackages")}</h3>
          <Card hover={false}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-1.5">
              {info.installed_packages.map((pkg, i) => (
                <div key={i} className="flex items-center justify-between py-1 border-b border-[var(--border-subtle)]/50 last:border-0">
                  <span className="text-xs text-[var(--text-primary)]">{pkg.name}</span>
                  <span className="text-xs text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{pkg.version}</span>
                </div>
              ))}
            </div>
          </Card>
        </>
      )}

      {/* Cron Jobs — structured collector (cron daemon state + system/user
          crontabs + drop-in dirs + anacron + systemd timers). Falls back to
          the legacy raw `cron_jobs` blob for scans persisted before the
          structured collector was added. */}
      {info.cron ? (
        <CronInfoCard cron={info.cron} t={t} />
      ) : (
        info.cron_jobs && info.cron_jobs.length > 0 && (
          <>
            <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.cronJobs")}</h3>
            <Card hover={false}>
              <pre className="text-xs text-[var(--text-secondary)] whitespace-pre-wrap break-all" style={{ fontFamily: "var(--font-mono)" }}>
                {info.cron_jobs.join("\n")}
              </pre>
            </Card>
          </>
        )
      )}

      {/* Firewall Status */}
      {info.firewall_status && (
        <>
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.firewallStatus")}</h3>
          <Card hover={false}>
            <pre className="text-xs text-[var(--text-secondary)] whitespace-pre-wrap break-all" style={{ fontFamily: "var(--font-mono)" }}>
              {info.firewall_status}
            </pre>
          </Card>
        </>
      )}

      {/* SSH Auth Policy — answers "does this host accept passwords, and for whom?" */}
      {info.ssh_auth_policy && <SSHAuthPolicyCard policy={info.ssh_auth_policy} users={info.remote_users ?? []} t={t} />}
    </div>
  );
}

/* ─── SSH Auth Policy card ─── */

function SSHAuthPolicyCard({ policy, users, t }: {
  policy: NonNullable<VMInfoType["ssh_auth_policy"]>;
  users: NonNullable<VMInfoType["remote_users"]>;
  t: (k: string) => string;
}) {
  // Treat "yes" / "no" as boolean-ish; anything else (including "") is unknown.
  const yn = (v?: string): "yes" | "no" | "unknown" => {
    if (v === "yes") return "yes";
    if (v === "no") return "no";
    return "unknown";
  };
  const passwordAllowed = yn(policy.password_auth);
  const kbdAllowed = yn(policy.kbd_interactive_auth);
  // Effective password access = either directive may unlock password-style auth.
  const effectivePassword: "yes" | "no" | "unknown" =
    passwordAllowed === "yes" || kbdAllowed === "yes"
      ? "yes"
      : passwordAllowed === "no" && kbdAllowed === "no"
      ? "no"
      : "unknown";

  const verdictClass =
    effectivePassword === "yes"
      ? "bg-amber-500/15 text-amber-300 light:text-amber-800 border-amber-500/40"
      : effectivePassword === "no"
      ? "bg-emerald-500/15 text-emerald-300 light:text-emerald-800 border-emerald-500/40"
      : "bg-[var(--bg-elevated)] text-[var(--text-faint)] border-[var(--border-default)]";

  // Cross-reference users against AllowUsers/DenyUsers + their PasswordStatus
  // to compute "users who can actually log in via password".
  const allow = new Set((policy.allow_users ?? []).map((s) => s.toLowerCase()));
  const deny = new Set((policy.deny_users ?? []).map((s) => s.toLowerCase()));
  const passwordCapableUsers = users.filter((u) => {
    if (effectivePassword !== "yes") return false;
    if (u.password_status !== "P") return false;
    if (!u.has_login) return false;
    const name = u.name.toLowerCase();
    if (deny.has(name)) return false;
    if (allow.size > 0 && !allow.has(name)) return false;
    return true;
  });

  const sources = policy.directive_sources ?? {};
  const rows: { label: string; value: React.ReactNode; key: string }[] = [
    { key: "passwordauthentication", label: t("scan.policyPasswordAuth"), value: <YesNoBadge value={passwordAllowed} t={t} /> },
    { key: "kbdinteractiveauthentication", label: t("scan.policyKbdInteractive"), value: <YesNoBadge value={kbdAllowed} t={t} /> },
    { key: "pubkeyauthentication", label: t("scan.policyPubkeyAuth"), value: <YesNoBadge value={yn(policy.pubkey_auth)} t={t} /> },
    { key: "usepam", label: t("scan.policyUsePAM"), value: <YesNoBadge value={yn(policy.use_pam)} t={t} /> },
    {
      key: "permitrootlogin",
      label: t("scan.policyPermitRoot"),
      value: (
        <span className="text-xs text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>
          {policy.permit_root_login || "—"}
        </span>
      ),
    },
  ];
  if (policy.authentication_methods && policy.authentication_methods !== "any") {
    rows.push({
      key: "authenticationmethods",
      label: t("scan.policyAuthMethods"),
      value: (
        <span className="text-xs text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>
          {policy.authentication_methods}
        </span>
      ),
    });
  }

  const renderList = (label: string, items: string[] | undefined) =>
    items && items.length > 0 ? (
      <div className="flex flex-wrap items-center gap-2 py-1.5 border-b border-[var(--border-subtle)]/50 last:border-0">
        <span className="text-xs text-[var(--text-muted)] shrink-0">{label}</span>
        <div className="flex flex-wrap gap-1">
          {items.map((u) => (
            <span
              key={u}
              className="px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[10px] text-[var(--text-secondary)] border border-[var(--border-subtle)]"
              style={{ fontFamily: "var(--font-mono)" }}
            >
              {u}
            </span>
          ))}
        </div>
      </div>
    ) : null;

  return (
    <>
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.sshAuthPolicy")}</h3>
      <Card hover={false}>
        {/* Verdict line */}
        <div className="flex items-center gap-2 mb-3">
          <span className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-full border text-xs ${verdictClass}`}>
            <span className="w-1.5 h-1.5 rounded-full bg-current" />
            {effectivePassword === "yes"
              ? t("scan.policyPasswordAllowed")
              : effectivePassword === "no"
              ? t("scan.policyPasswordDenied")
              : t("scan.policyPasswordUnknown")}
          </span>
          {policy.source && (
            <span className="ml-auto text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              {policy.source}
            </span>
          )}
        </div>

        {/* Directives grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 mb-3">
          {rows.map(({ key, label, value }) => {
            const hits = sources[key];
            return (
              <div key={key} className="py-1.5 border-b border-[var(--border-subtle)]/50 last:border-0">
                <div className="flex items-center justify-between">
                  <span className="text-xs text-[var(--text-muted)]">{label}</span>
                  {value}
                </div>
                {hits && hits.length > 0 && (
                  <div className="mt-1 space-y-0.5">
                    {hits.map((h, i) => (
                      <div
                        key={i}
                        className="text-[10px] text-[var(--text-faint)] truncate"
                        style={{ fontFamily: "var(--font-mono)" }}
                        title={h.value ? `${h.file}:${h.line} → ${h.value}` : `${h.file}:${h.line}`}
                      >
                        <span className="opacity-60">↳</span> {h.file}:{h.line}
                        {h.value && <span className="ml-1 opacity-70">{h.value}</span>}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {/* Allow/Deny lists */}
        {(policy.allow_users?.length ||
          policy.allow_groups?.length ||
          policy.deny_users?.length ||
          policy.deny_groups?.length) ? (
          <div className="border-t border-[var(--border-subtle)]/50 pt-2">
            {renderList(t("scan.policyAllowUsers"), policy.allow_users)}
            {renderList(t("scan.policyAllowGroups"), policy.allow_groups)}
            {renderList(t("scan.policyDenyUsers"), policy.deny_users)}
            {renderList(t("scan.policyDenyGroups"), policy.deny_groups)}
          </div>
        ) : null}

        {/* Password-capable users — empty list when password auth is off */}
        {effectivePassword !== "no" && (
          <div className="border-t border-[var(--border-subtle)]/50 pt-3 mt-3">
            <span className="text-xs text-[var(--text-muted)] block mb-2">{t("scan.policyPasswordUsers")}</span>
            {passwordCapableUsers.length === 0 ? (
              <p className="text-[10px] text-[var(--text-faint)] italic">
                {effectivePassword === "yes" ? t("scan.policyPasswordUsersNone") : t("scan.policyPasswordUsersUnknown")}
              </p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {passwordCapableUsers.map((u) => (
                  <span
                    key={u.name}
                    className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-300 light:text-amber-800 border border-amber-500/30 text-xs"
                    style={{ fontFamily: "var(--font-mono)" }}
                    title={`uid ${u.uid} · ${u.shell || ""}`}
                  >
                    {u.name}
                  </span>
                ))}
              </div>
            )}
          </div>
        )}
      </Card>
    </>
  );
}

function YesNoBadge({ value, t }: { value: "yes" | "no" | "unknown"; t: (k: string) => string }) {
  if (value === "yes") {
    return (
      <span className="px-1.5 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 light:text-emerald-800 border border-emerald-500/30 text-[10px]">
        {t("common.yes")}
      </span>
    );
  }
  if (value === "no") {
    return (
      <span className="px-1.5 py-0.5 rounded-full bg-rose-500/15 text-rose-300 light:text-rose-800 border border-rose-500/30 text-[10px]">
        {t("common.no")}
      </span>
    );
  }
  return (
    <span className="px-1.5 py-0.5 rounded-full bg-[var(--bg-elevated)] text-[var(--text-faint)] border border-[var(--border-subtle)] text-[10px]">
      —
    </span>
  );
}

/* ─── Unified process/service card section ─── */

type NormalizedProcess = {
  key: string;
  icon: string;
  type: string;
  name: string;
  command: string;
  pid?: string;
  user?: string;
  cpu: string;
  mem: string;
  startedVia?: string;
  cwd?: string;
  venv?: string;
  ports?: string;
  isSystem?: boolean;
};

function ProcessCards({ info, t, gridCols }: { info: VMInfoType; t: (k: string) => string; gridCols: string }) {
  const processes = useMemo(() => {
    // New scan format — full process details
    if (info.process_details && info.process_details.length > 0) {
      const list = info.process_details.map((p): NormalizedProcess => ({
        key: p.pid,
        icon: processIcon(p.command),
        type: processType(p.command),
        name: processType(p.command),
        command: p.command,
        pid: p.pid,
        user: p.user,
        cpu: p.cpu,
        mem: p.mem,
        startedVia: p.started_via,
        cwd: p.cwd,
        venv: p.venv,
        ports: p.ports,
        isSystem: isSystemProcess(p.command, p.cwd),
      }));
      // App processes first, system processes last
      list.sort((a, b) => (a.isSystem ? 1 : 0) - (b.isSystem ? 1 : 0));
      return list;
    }
    // Legacy scan format — service_details strings
    if (info.service_details && info.service_details.length > 0) {
      return info.service_details.map((s, i): NormalizedProcess => {
        const row = parseServiceRow(s);
        return {
          key: `svc-${i}`,
          icon: processIcon(row.name),
          type: processType(row.name),
          name: row.name,
          command: row.name,
          cpu: row.cpu,
          mem: row.mem,
        };
      });
    }
    // Minimal — just service names
    if (info.services && info.services.length > 0) {
      return info.services.map((s, i): NormalizedProcess => ({
        key: `name-${i}`,
        icon: processIcon(s),
        type: processType(s),
        name: s,
        command: s,
        cpu: "--",
        mem: "--",
      }));
    }
    return [];
  }, [info.process_details, info.service_details, info.services]);

  if (processes.length === 0) return null;

  return (
    <>
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.runningServices")}</h3>
      <div className={`grid ${gridCols} gap-2`}>
        {processes.map((p) => (
          <Card key={p.key} hover={false} className={`!p-3 flex flex-col ${p.isSystem ? "opacity-60" : ""}`}>
            {/* Header: icon + type + system badge + PID badge */}
            <div className="flex items-center gap-2 mb-2">
              <span className="text-base shrink-0" title={p.type}>{p.icon}</span>
              <span className={`text-sm font-medium truncate ${p.isSystem ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]"}`}>{p.type}</span>
              {p.isSystem && (
                <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-[var(--bg-elevated)] text-[10px] text-[var(--text-faint)] border border-[var(--border-subtle)]">OS</span>
              )}
              {p.pid && (
                <span className="ml-auto shrink-0 px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[10px] text-[var(--text-faint)] border border-[var(--border-subtle)]" style={{ fontFamily: "var(--font-mono)" }}>
                  PID {p.pid}
                </span>
              )}
            </div>

            {/* Command line */}
            {p.command !== p.type && (
              <p className="text-[10px] text-[var(--text-muted)] break-all leading-relaxed mb-2 line-clamp-2" style={{ fontFamily: "var(--font-mono)" }}>
                {p.command}
              </p>
            )}

            {/* Metadata grid */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-x-3 gap-y-1.5 text-[10px]">
              {p.user && (
                <div>
                  <span className="text-[var(--text-faint)] block">{t("vm.user")}</span>
                  <span className="text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{p.user}</span>
                </div>
              )}
              {p.startedVia && (
                <div>
                  <span className="text-[var(--text-faint)] block">{t("scan.startedVia")}</span>
                  <span className={p.startedVia === "manual" ? "text-amber-400" : p.startedVia === "systemd" ? "text-emerald-400" : "text-[var(--text-secondary)]"}>
                    {p.startedVia}
                  </span>
                </div>
              )}
              <div>
                <span className="text-[var(--text-faint)] block">CPU / MEM</span>
                <span className="text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{p.cpu} / {p.mem}</span>
              </div>
              {p.ports && (
                <div>
                  <span className="text-[var(--text-faint)] block">{t("scan.listeningPorts")}</span>
                  <span className="text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{p.ports}</span>
                </div>
              )}
            </div>

            {/* Bottom details: cwd / venv */}
            {(p.cwd || p.venv) && (
              <div className="mt-auto pt-2 mt-2 border-t border-[var(--border-subtle)]/50 space-y-0.5 text-[10px]">
                {p.cwd && (
                  <div className="flex gap-1.5">
                    <span className="text-[var(--text-faint)] shrink-0">cwd</span>
                    <span className="text-[var(--text-muted)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{p.cwd}</span>
                  </div>
                )}
                {p.venv && (
                  <div className="flex gap-1.5">
                    <span className="text-[var(--text-faint)] shrink-0">venv</span>
                    <span className="text-[var(--text-muted)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{p.venv}</span>
                  </div>
                )}
              </div>
            )}
          </Card>
        ))}
      </div>
    </>
  );
}

/* ─── Process icon/type helpers ─── */

const PROCESS_MATCHERS: [string[], string, string][] = [
  [["python"], "\ud83d\udc0d", "Python"],
  [["node"], "\ud83d\udfe2", "Node.js"],
  [["java"], "\u2615", "Java"],
  [["ruby"], "\ud83d\udc8e", "Ruby"],
  [["php"], "\ud83d\udc18", "PHP"],
  [["nginx"], "\ud83c\udf10", "Nginx"],
  [["apache", "httpd"], "\ud83c\udf10", "Apache"],
  [["caddy", "haproxy", "traefik"], "\ud83c\udf10", "Proxy"],
  [["postgres"], "\ud83d\uddc4\ufe0f", "PostgreSQL"],
  [["mysql", "mariadb"], "\ud83d\uddc4\ufe0f", "MySQL"],
  [["redis"], "\ud83d\uddc4\ufe0f", "Redis"],
  [["mongo"], "\ud83d\uddc4\ufe0f", "MongoDB"],
  [["gunicorn"], "\u2699\ufe0f", "Gunicorn"],
  [["uvicorn"], "\u2699\ufe0f", "Uvicorn"],
  [["celery"], "\ud83e\uddf1", "Celery"],
  [["pm2"], "\u2699\ufe0f", "PM2"],
  [["supervisord"], "\u2699\ufe0f", "Supervisor"],
];

function processIcon(cmd: string): string {
  const lower = cmd.toLowerCase();
  for (const [keys, icon] of PROCESS_MATCHERS) {
    if (keys.some((k) => lower.includes(k))) return icon;
  }
  return "\ud83d\udd39";
}

function processType(cmd: string): string {
  const lower = cmd.toLowerCase();
  for (const [keys, , type] of PROCESS_MATCHERS) {
    if (keys.some((k) => lower.includes(k))) return type;
  }
  return "Process";
}

const SYSTEM_PATHS = ["/usr/bin/", "/usr/sbin/", "/usr/lib/", "/usr/share/", "/lib/systemd/", "/sbin/"];
const SYSTEM_COMMANDS = [
  "networkd-dispatcher", "unattended-upgrade", "check-new-release",
  "packagekitd", "snapd", "thermald", "accounts-daemon", "polkitd",
  "systemd-", "udisksd", "fwupd", "colord", "ModemManager",
  "irqbalance", "multipathd", "atd", "cron",
];

function isSystemProcess(command: string, cwd?: string): boolean {
  if (SYSTEM_PATHS.some((p) => command.startsWith(p))) return true;
  if (SYSTEM_COMMANDS.some((c) => command.toLowerCase().includes(c.toLowerCase()))) return true;
  if (cwd === "/" || cwd === "/root") return true;
  return false;
}

/* ─── Cron / scheduled tasks card ─── */

// Buckets jobs into "system" (anything package-manager / sysadmin-owned —
// /etc/crontab, /etc/cron.d/*, drop-in dirs, anacron, systemd timers) and
// "user" (per-user crontabs in /var/spool/cron/, edited by the owning
// account). This is the operator-facing distinction: "what does the OS
// schedule for me?" vs "what did somebody add by hand on this host?".
function bucketCronJobs(jobs: CronJob[]): { system: CronJob[]; user: CronJob[] } {
  const system: CronJob[] = [];
  const user: CronJob[] = [];
  for (const j of jobs) {
    if (j.source.startsWith("user:")) user.push(j);
    else system.push(j);
  }
  return { system, user };
}

function CronInfoCard({ cron, t }: { cron: CronInfo; t: (k: string) => string }) {
  const jobs = cron.jobs ?? [];
  const { system, user } = bucketCronJobs(jobs);

  const daemonState: "active" | "inactive" | "missing" =
    !cron.daemon_installed && !cron.daemon_active ? "missing" : cron.daemon_active ? "active" : "inactive";
  const daemonChip =
    daemonState === "active"
      ? "bg-emerald-500/15 text-emerald-300 light:text-emerald-800 border-emerald-500/40"
      : daemonState === "inactive"
      ? "bg-amber-500/15 text-amber-300 light:text-amber-800 border-amber-500/40"
      : "bg-slate-500/15 text-slate-300 light:text-slate-700 border-slate-500/40";
  const daemonLabel =
    daemonState === "active"
      ? t("scan.cron.daemonActive")
      : daemonState === "inactive"
      ? t("scan.cron.daemonInactive")
      : t("scan.cron.daemonMissing");

  return (
    <>
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.cronJobs")}</h3>
      <Card hover={false}>
        {/* Daemon state row */}
        <div className="flex flex-wrap items-center gap-2 mb-3 pb-3 border-b border-[var(--border-subtle)]/50">
          <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] border ${daemonChip}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${daemonState === "active" ? "bg-emerald-400" : daemonState === "inactive" ? "bg-amber-400" : "bg-slate-400"}`} />
            {(cron.daemon_name || "cron")}: {daemonLabel}
          </span>
          {cron.daemon_installed && (
            <span className="text-[10px] text-[var(--text-muted)]">
              {cron.daemon_enabled ? t("scan.cron.daemonEnabledAtBoot") : t("scan.cron.daemonNotEnabled")}
            </span>
          )}
          <span className="ml-auto text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
            {jobs.length} {jobs.length === 1 ? t("scan.cron.jobSingular") : t("scan.cron.jobPlural")}
          </span>
        </div>

        {jobs.length === 0 ? (
          <div className="text-xs text-[var(--text-muted)] py-2">{t("scan.cron.noJobs")}</div>
        ) : (
          <div className="space-y-4">
            {system.length > 0 && <CronJobGroup title={t("scan.cron.systemJobs")} jobs={system} t={t} />}
            {user.length > 0 && <CronJobGroup title={t("scan.cron.userJobs")} jobs={user} t={t} />}
          </div>
        )}
      </Card>
    </>
  );
}

function CronJobGroup({ title, jobs, t }: { title: string; jobs: CronJob[]; t: (k: string) => string }) {
  return (
    <div>
      <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-faint)] block mb-2">
        {title} <span className="opacity-60">({jobs.length})</span>
      </span>
      <div className="space-y-1.5">
        {jobs.map((j, i) => (
          <CronJobRow key={i} job={j} t={t} />
        ))}
      </div>
    </div>
  );
}

function CronJobRow({ job, t }: { job: CronJob; t: (k: string) => string }) {
  const isTimer = job.kind === "timer";
  const sourceLabel = job.source.startsWith("user:")
    ? job.source.slice(5)
    : job.source.startsWith("/etc/cron.d/")
    ? job.source.slice("/etc/cron.d/".length)
    : job.source;

  return (
    <div
      className={`flex flex-wrap items-center gap-2 px-2 py-1.5 rounded border border-[var(--border-subtle)]/50 ${
        job.disabled ? "opacity-50" : ""
      }`}
    >
      <span
        className="shrink-0 inline-flex items-center px-1.5 py-0.5 rounded bg-[var(--bg-elevated)] text-[10px] text-[var(--text-secondary)] border border-[var(--border-default)]"
        style={{ fontFamily: "var(--font-mono)" }}
        title={isTimer ? t("scan.cron.timerUnit") : t("scan.cron.schedule")}
      >
        {job.schedule || "—"}
      </span>
      {job.user && (
        <span
          className="shrink-0 px-1.5 py-0.5 rounded-full text-[10px] bg-sky-500/10 text-sky-300 light:text-sky-800 border border-sky-500/30"
          title={t("scan.cron.runAs")}
          style={{ fontFamily: "var(--font-mono)" }}
        >
          {job.user}
        </span>
      )}
      <span
        className="text-xs text-[var(--text-primary)] flex-1 min-w-0 truncate"
        style={{ fontFamily: "var(--font-mono)" }}
        title={job.command}
      >
        {job.command || "—"}
      </span>
      {job.disabled && (
        <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-slate-500/15 text-[10px] text-slate-300 light:text-slate-700 border border-slate-500/30">
          {t("scan.cron.disabled")}
        </span>
      )}
      {job.next_run && (
        <span className="shrink-0 text-[10px] text-[var(--text-muted)]" title={t("scan.cron.nextRun")}>
          → {job.next_run}
        </span>
      )}
      <span
        className="shrink-0 text-[10px] text-[var(--text-faint)]"
        title={job.source}
        style={{ fontFamily: "var(--font-mono)" }}
      >
        {sourceLabel}
      </span>
    </div>
  );
}

/* ─── Management agents card ─── */

const AGENT_CATEGORY_ORDER = [
  "monitoring",
  "logging",
  "security",
  "config-mgmt",
  "inventory",
  "backup",
  "cloud",
  "orchestration",
  "remote-access",
] as const;

const AGENT_CATEGORY_ICON: Record<string, string> = {
  monitoring: "📊",      // 📊
  logging: "📝",         // 📝
  security: "🛡️",  // 🛡️
  "config-mgmt": "⚙️",   // ⚙️
  inventory: "📋",       // 📋
  backup: "💾",          // 💾
  cloud: "☁️",           // ☁️
  orchestration: "🚢",   // 🚢
  "remote-access": "🔌", // 🔌
};

function agentStateClasses(state?: string): string {
  switch (state) {
    case "active":
    case "running":
      return "bg-emerald-500/15 text-emerald-300 light:text-emerald-800 border-emerald-500/40";
    case "failed":
      return "bg-red-500/15 text-red-300 light:text-red-800 border-red-500/40";
    case "stopped":
    case "inactive":
      return "bg-amber-500/15 text-amber-300 light:text-amber-800 border-amber-500/40";
    default:
      return "bg-slate-500/15 text-slate-300 light:text-slate-700 border-slate-500/40";
  }
}

function AgentsCard({ agents, t }: { agents: Agent[]; t: (k: string) => string }) {
  // Bucket by category in a stable display order; trailing categories (any
  // not in the canonical list) get appended alphabetically.
  const buckets = new Map<string, Agent[]>();
  for (const a of agents) {
    const list = buckets.get(a.category) ?? [];
    list.push(a);
    buckets.set(a.category, list);
  }
  const ordered: [string, Agent[]][] = [];
  for (const cat of AGENT_CATEGORY_ORDER) {
    const list = buckets.get(cat);
    if (list && list.length > 0) {
      ordered.push([cat, list]);
      buckets.delete(cat);
    }
  }
  for (const [cat, list] of [...buckets.entries()].sort()) {
    ordered.push([cat, list]);
  }

  const activeCount = agents.filter((a) => a.state === "active" || a.state === "running").length;

  return (
    <>
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">
        {t("scan.agents.title")}
      </h3>
      <Card hover={false}>
        <div className="flex flex-wrap items-center gap-2 mb-3 pb-3 border-b border-[var(--border-subtle)]/50">
          <span className="text-[10px] text-[var(--text-muted)]">
            {t("scan.agents.summary")}
          </span>
          <span className="ml-auto text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
            {activeCount}/{agents.length} {t("scan.agents.activeOfTotal")}
          </span>
        </div>
        <div className="space-y-4">
          {ordered.map(([cat, list]) => (
            <AgentCategoryGroup key={cat} category={cat} agents={list} t={t} />
          ))}
        </div>
      </Card>
    </>
  );
}

function AgentCategoryGroup({ category, agents, t }: { category: string; agents: Agent[]; t: (k: string) => string }) {
  const icon = AGENT_CATEGORY_ICON[category] ?? "⚙️";
  // i18n key falls back to the raw category id when no translation exists.
  const label = (() => {
    const key = `scan.agents.category.${category}`;
    const tr = t(key);
    return tr === key ? category : tr;
  })();

  return (
    <div>
      <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-faint)] block mb-2">
        <span className="mr-1">{icon}</span>
        {label} <span className="opacity-60">({agents.length})</span>
      </span>
      <div className="space-y-1.5">
        {agents.map((a) => (
          <AgentRow key={a.name} agent={a} t={t} />
        ))}
      </div>
    </div>
  );
}

function AgentRow({ agent, t }: { agent: Agent; t: (k: string) => string }) {
  const stateLabel = agent.state ? t(`scan.agents.state.${agent.state}`) : t("scan.agents.state.unknown");
  const stateText = stateLabel.startsWith("scan.agents.state.") ? agent.state ?? "—" : stateLabel;
  return (
    <div className="flex flex-wrap items-center gap-2 px-2 py-1.5 rounded border border-[var(--border-subtle)]/50">
      <span className={`shrink-0 inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] border ${agentStateClasses(agent.state)}`}>
        <span className={`w-1.5 h-1.5 rounded-full ${
          agent.state === "active" || agent.state === "running"
            ? "bg-emerald-400"
            : agent.state === "failed"
            ? "bg-red-400"
            : agent.state === "stopped" || agent.state === "inactive"
            ? "bg-amber-400"
            : "bg-slate-400"
        }`} />
        {stateText}
      </span>
      <span className="text-xs text-[var(--text-primary)] font-medium">{agent.label}</span>
      {agent.vendor && (
        <span className="text-[10px] text-[var(--text-faint)]">— {agent.vendor}</span>
      )}
      {agent.version && (
        <span
          className="text-[10px] text-[var(--text-muted)] px-1.5 py-0.5 rounded bg-[var(--bg-elevated)] border border-[var(--border-default)]"
          style={{ fontFamily: "var(--font-mono)" }}
          title={agent.package ? `${agent.package} ${agent.version}` : agent.version}
        >
          v{agent.version}
        </span>
      )}
      {agent.enabled && (
        <span
          className="shrink-0 px-1.5 py-0.5 rounded-full bg-sky-500/10 text-sky-300 light:text-sky-800 border border-sky-500/30 text-[10px]"
          title={t("scan.agents.enabledAtBootTooltip")}
        >
          {t("scan.agents.enabledAtBoot")}
        </span>
      )}
      {agent.ports && agent.ports.length > 0 && (
        <span
          className="shrink-0 text-[10px] text-[var(--text-muted)]"
          style={{ fontFamily: "var(--font-mono)" }}
          title={t("scan.agents.portsTooltip")}
        >
          :{agent.ports.join(", :")}
        </span>
      )}
      <span className="ml-auto flex items-center gap-1.5 flex-wrap">
        {(agent.sources ?? []).map((src) => (
          <span
            key={src}
            className="text-[10px] text-[var(--text-faint)] px-1.5 py-0.5 rounded border border-[var(--border-subtle)]"
            style={{ fontFamily: "var(--font-mono)" }}
            title={t(`scan.agents.source.${src}`)}
          >
            {src}
          </span>
        ))}
      </span>
    </div>
  );
}

/* ─── Service inventory card (unified web/db/cache/queue/… taxonomy) ─── */

const SERVICE_KIND_ORDER = [
  "web",
  "proxy",
  "database",
  "cache",
  "queue",
  "runtime",
  "dns",
  "mail",
  "file-sharing",
  "directory",
  "analytics",
  "logging",
] as const;

const SERVICE_KIND_ICON: Record<string, string> = {
  web: "🌐",
  proxy: "🔀",
  database: "🗄️",
  cache: "⚡",
  queue: "📨",
  runtime: "⚙️",
  dns: "📡",
  mail: "✉️",
  "file-sharing": "📁",
  directory: "🪪",
  analytics: "📈",
  logging: "📜",
};

function serviceStateClasses(state?: string): string {
  switch (state) {
    case "active":
    case "running":
      return "bg-emerald-500/15 text-emerald-300 light:text-emerald-800 border-emerald-500/40";
    case "failed":
      return "bg-red-500/15 text-red-300 light:text-red-800 border-red-500/40";
    case "stopped":
    case "inactive":
      return "bg-amber-500/15 text-amber-300 light:text-amber-800 border-amber-500/40";
    default:
      return "bg-slate-500/15 text-slate-300 light:text-slate-700 border-slate-500/40";
  }
}

function ServiceInventoryCard({ services, t }: { services: DiscoveredService[]; t: (k: string) => string }) {
  const buckets = new Map<string, DiscoveredService[]>();
  for (const s of services) {
    const list = buckets.get(s.kind) ?? [];
    list.push(s);
    buckets.set(s.kind, list);
  }
  const ordered: [string, DiscoveredService[]][] = [];
  for (const k of SERVICE_KIND_ORDER) {
    const list = buckets.get(k);
    if (list && list.length > 0) {
      ordered.push([k, list]);
      buckets.delete(k);
    }
  }
  for (const [k, list] of [...buckets.entries()].sort()) {
    ordered.push([k, list]);
  }

  const activeCount = services.filter((s) => s.state === "active" || s.state === "running").length;

  return (
    <>
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">
        {t("scan.services.title")}
      </h3>
      <Card hover={false}>
        <div className="flex flex-wrap items-center gap-2 mb-3 pb-3 border-b border-[var(--border-subtle)]/50">
          <span className="text-[10px] text-[var(--text-muted)]">{t("scan.services.summary")}</span>
          <span className="ml-auto text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
            {activeCount}/{services.length} {t("scan.services.activeOfTotal")}
          </span>
        </div>
        <div className="space-y-4">
          {ordered.map(([kind, list]) => (
            <ServiceKindGroup key={kind} kind={kind} services={list} t={t} />
          ))}
        </div>
      </Card>
    </>
  );
}

function ServiceKindGroup({ kind, services, t }: { kind: string; services: DiscoveredService[]; t: (k: string) => string }) {
  const icon = SERVICE_KIND_ICON[kind] ?? "🔧";
  const label = (() => {
    const key = `scan.services.kind.${kind}`;
    const tr = t(key);
    return tr === key ? kind : tr;
  })();
  return (
    <div>
      <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-faint)] block mb-2">
        <span className="mr-1">{icon}</span>
        {label} <span className="opacity-60">({services.length})</span>
      </span>
      <div className="space-y-1.5">
        {services.map((s) => (
          <ServiceRow key={s.name} service={s} t={t} />
        ))}
      </div>
    </div>
  );
}

function ServiceRow({ service, t }: { service: DiscoveredService; t: (k: string) => string }) {
  const stateLabelKey = `scan.services.state.${service.state ?? "unknown"}`;
  const stateLabel = t(stateLabelKey);
  const stateText = stateLabel === stateLabelKey ? service.state ?? "—" : stateLabel;

  return (
    <div className="flex flex-wrap items-center gap-2 px-2 py-1.5 rounded border border-[var(--border-subtle)]/50">
      <span className={`shrink-0 inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] border ${serviceStateClasses(service.state)}`}>
        <span className={`w-1.5 h-1.5 rounded-full ${
          service.state === "active" || service.state === "running"
            ? "bg-emerald-400"
            : service.state === "failed"
            ? "bg-red-400"
            : service.state === "stopped" || service.state === "inactive"
            ? "bg-amber-400"
            : "bg-slate-400"
        }`} />
        {stateText}
      </span>
      <span className="text-xs text-[var(--text-primary)] font-medium">{service.label}</span>
      {service.vendor && (
        <span className="text-[10px] text-[var(--text-faint)]">— {service.vendor}</span>
      )}
      {service.version && (
        <span
          className="text-[10px] text-[var(--text-muted)] px-1.5 py-0.5 rounded bg-[var(--bg-elevated)] border border-[var(--border-default)]"
          style={{ fontFamily: "var(--font-mono)" }}
          title={service.package ? `${service.package} ${service.version}` : service.version}
        >
          v{service.version}
        </span>
      )}
      {service.enabled && (
        <span
          className="shrink-0 px-1.5 py-0.5 rounded-full bg-sky-500/10 text-sky-300 light:text-sky-800 border border-sky-500/30 text-[10px]"
          title={t("scan.services.enabledAtBootTooltip")}
        >
          {t("scan.services.enabledAtBoot")}
        </span>
      )}
      {service.container_image && (
        <span
          className="shrink-0 text-[10px] text-cyan-300 light:text-cyan-800 px-1.5 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/30"
          style={{ fontFamily: "var(--font-mono)" }}
          title={service.container_id ? `${service.container_image} (${service.container_id.slice(0, 12)})` : service.container_image}
        >
          {service.container_image}
        </span>
      )}
      {service.ports && service.ports.length > 0 && (
        <span
          className="shrink-0 text-[10px] text-[var(--text-muted)]"
          style={{ fontFamily: "var(--font-mono)" }}
          title={t("scan.services.portsTooltip")}
        >
          :{service.ports.join(", :")}
        </span>
      )}
      <span className="ml-auto flex items-center gap-1.5 flex-wrap">
        {(service.sources ?? []).map((src) => (
          <span
            key={src}
            className="text-[10px] text-[var(--text-faint)] px-1.5 py-0.5 rounded border border-[var(--border-subtle)]"
            style={{ fontFamily: "var(--font-mono)" }}
            title={t(`scan.services.source.${src}`)}
          >
            {src}
          </span>
        ))}
      </span>
    </div>
  );
}

/* ─── Resource top consumers (top 10 CPU / RAM / Disk) ─── */

function ResourceTopPanel({
  snapshot,
  t,
}: {
  snapshot: ResourceUsageSnapshot;
  t: (k: string) => string;
}) {
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

function ResourceTopList({
  title,
  items,
}: {
  title: string;
  items: Array<{ primary: string; secondary: string; value: string; tooltip?: string }>;
}) {
  return (
    <div>
      <span className="block text-[10px] text-[var(--text-faint)] uppercase tracking-wider mb-2">
        {title}
      </span>
      <div className="space-y-1">
        {items.map((it, i) => (
          <div
            key={i}
            className="flex items-baseline gap-2 px-1.5 py-1 rounded hover:bg-[var(--bg-elevated)]/40 text-xs"
            title={it.tooltip}
          >
            <span className="shrink-0 text-[10px] text-[var(--text-faint)] w-4 tabular-nums">{i + 1}</span>
            <div className="flex-1 min-w-0">
              <div
                className="text-[var(--text-primary)] truncate"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {it.primary || "—"}
              </div>
              {it.secondary && (
                <div className="text-[10px] text-[var(--text-muted)] truncate">{it.secondary}</div>
              )}
            </div>
            <span
              className="shrink-0 text-[var(--text-secondary)] font-medium tabular-nums"
              style={{ fontFamily: "var(--font-mono)" }}
            >
              {it.value}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// truncateCommand cuts excessively long argv strings (Java classpaths, etc)
// to a 60-char preview so the rows stay aligned. The full string lives in
// the row's tooltip via the `tooltip` field.
function truncateCommand(cmd: string): string {
  if (!cmd) return "";
  if (cmd.length <= 60) return cmd;
  return cmd.slice(0, 60) + "…";
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
