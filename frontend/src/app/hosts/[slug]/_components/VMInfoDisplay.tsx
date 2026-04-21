"use client";

import { useState, useMemo } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import { UsageBar } from "./UsageBar";
import { ContainersList } from "./SortableResourceList";
import { formatUptime, parseLoginEntry, formatLoginDate, portIcon, parseServiceRow } from "@/lib/utils";
import type { VMInfoType, ProcessDetail, PortOwner } from "@/lib/api";

export default function VMInfoDisplay({ info, locale, compact }: { info: VMInfoType; locale: string; compact?: boolean }) {
  const { t } = useLocale();
  const [loginsView, setLoginsView] = useState<"cards" | "table">("cards");
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

      {/* Last SSH Logins */}
      {info.last_logins?.length > 0 && (
        <>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">{t("scan.lastLogins")}</h3>
          <ViewToggle
            value={loginsView}
            onChange={(v) => setLoginsView(v as "cards" | "table")}
            options={[
              { key: "cards", label: t("common.cards"), icon: VIEW_ICONS.cards },
              { key: "table", label: t("common.table"), icon: VIEW_ICONS.table },
            ]}
          />
        </div>
        {loginsView === "cards" ? (
          <div className={`grid ${gridCols} gap-2`}>
            {info.last_logins.map((l, i) => {
              const { user, from, when } = parseLoginEntry(l);
              return (
                <Card key={i} hover={false} className="!p-3">
                  <div className="flex items-center gap-2 mb-2">
                    <svg className="w-3.5 h-3.5 text-[var(--text-faint)] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                    </svg>
                    <span className="text-sm font-medium text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{user}</span>
                  </div>
                  <div className="grid grid-cols-2 gap-3 text-xs">
                    <div>
                      <span className="text-[var(--text-faint)] block mb-0.5">{t("vm.from")}</span>
                      <span className="text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{from}</span>
                    </div>
                    <div>
                      <span className="text-[var(--text-faint)] block mb-0.5">{t("vm.when")}</span>
                      <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{formatLoginDate(when, locale)}</span>
                    </div>
                  </div>
                </Card>
              );
            })}
          </div>
        ) : (
          <SortableTable columns={[{ key: "user" as const, label: t("vm.user") }, { key: "from" as const, label: t("vm.from") }, { key: "when" as const, label: t("vm.when") }]} defaultSort="user">
            {(sk, sd) => {
              const parsed = info.last_logins.map((l) => ({ ...parseLoginEntry(l), raw: l }));
              const sorted = sortRows(parsed, sk, sd, { user: (a, b) => a.user.localeCompare(b.user), from: (a, b) => a.from.localeCompare(b.from), when: (a, b) => a.when.localeCompare(b.when) });
              return sorted.map((row, i) => (
                <tr key={i} className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}>
                  <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{row.user}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>{row.from}</td>
                  <td className="px-4 py-2.5 text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{formatLoginDate(row.when, locale)}</td>
                </tr>
              ));
            }}
          </SortableTable>
        )}
        </>
      )}

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

      {/* Systemd Services */}
      {info.systemd_services && info.systemd_services.length > 0 && (
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

      {/* Cron Jobs */}
      {info.cron_jobs && info.cron_jobs.length > 0 && (
        <>
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("scan.cronJobs")}</h3>
          <Card hover={false}>
            <pre className="text-xs text-[var(--text-secondary)] whitespace-pre-wrap break-all" style={{ fontFamily: "var(--font-mono)" }}>
              {info.cron_jobs.join("\n")}
            </pre>
          </Card>
        </>
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
    </div>
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
