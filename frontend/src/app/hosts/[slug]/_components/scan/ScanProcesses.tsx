"use client";

import { useMemo } from "react";
import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import { parseServiceRow } from "@/lib/utils";
import type { VMInfoType } from "@/lib/api";
import { Rows, Row, Detail, type T } from "./shared";

type NormalizedProcess = {
  key: string;
  icon: string;
  type: string;
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

/**
 * Running processes as compact rows — type, command, user, CPU/MEM, PID —
 * app processes first, OS ones dimmed at the end; a row expands to how it
 * was started, its ports, cwd and venv. Accepts all three scan formats.
 */
export default function ScanProcesses({ info, t }: { info: VMInfoType; t: T }) {
  const processes = useMemo((): NormalizedProcess[] => {
    if (info.process_details?.length) {
      const list = info.process_details.map((p): NormalizedProcess => ({
        key: p.pid,
        icon: processIcon(p.command),
        type: processType(p.command),
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
      list.sort((a, b) => (a.isSystem ? 1 : 0) - (b.isSystem ? 1 : 0));
      return list;
    }
    if (info.service_details?.length) {
      return info.service_details.map((s, i) => {
        const row = parseServiceRow(s);
        return { key: `svc-${i}`, icon: processIcon(row.name), type: processType(row.name), command: row.name, cpu: row.cpu, mem: row.mem };
      });
    }
    return (info.services ?? []).map((s, i) => ({ key: `name-${i}`, icon: processIcon(s), type: processType(s), command: s, cpu: "–", mem: "–" }));
  }, [info.process_details, info.service_details, info.services]);

  return (
    <SectionCard
      as="h3"
      title={t("scan.pane.group.processes")}
      count={processes.length || undefined}
      body="flush"
      empty={processes.length === 0 ? t("scan.pane.empty.processes") : undefined}
    >
      <Rows>
        {processes.map((p) => {
          const details = p.startedVia || p.ports || p.cwd || p.venv || p.command.length > 60;
          return (
            <Row
              key={p.key}
              muted={p.isSystem}
              summary={
                <>
                  <span className="text-sm shrink-0 w-5 text-center" title={p.type}>{p.icon}</span>
                  <span className="w-24 shrink-0 font-medium text-[var(--text-primary)] truncate">{p.type}</span>
                  {p.isSystem && <Badge>{t("vm.os")}</Badge>}
                  <span className="flex-1 min-w-0 truncate font-mono text-[var(--text-muted)]" title={p.command}>{p.command}</span>
                  {p.user && <span className="hidden sm:inline shrink-0 font-mono text-[var(--text-secondary)]">{p.user}</span>}
                  <span className="shrink-0 font-mono tabular-nums text-[var(--text-secondary)]" title={t("vm.cpuMemLabel")}>{p.cpu} / {p.mem}</span>
                  {p.pid && <span className="hidden md:inline shrink-0 w-16 text-right font-mono text-[var(--text-faint)]">{p.pid}</span>}
                </>
              }
            >
              {details && (
                <>
                  <p className="font-mono text-[var(--text-secondary)] break-all">{p.command}</p>
                  {p.startedVia && (
                    <Detail label={t("scan.startedVia")} mono={false}>
                      <span className={p.startedVia === "manual" ? "text-[var(--warning)]" : p.startedVia === "systemd" ? "text-[var(--success)]" : ""}>{p.startedVia}</span>
                    </Detail>
                  )}
                  {p.ports && <Detail label={t("scan.listeningPorts")}>{p.ports}</Detail>}
                  {p.cwd && <Detail label={t("vm.cwd")}>{p.cwd}</Detail>}
                  {p.venv && <Detail label={t("vm.venv")}>{p.venv}</Detail>}
                </>
              )}
            </Row>
          );
        })}
      </Rows>
    </SectionCard>
  );
}

/* ─── Process icon/type helpers ─── */

const PROCESS_MATCHERS: [string[], string, string][] = [
  [["python"], "🐍", "Python"],
  [["node"], "🟢", "Node.js"],
  [["java"], "☕", "Java"],
  [["ruby"], "💎", "Ruby"],
  [["php"], "🐘", "PHP"],
  [["nginx"], "🌐", "Nginx"],
  [["apache", "httpd"], "🌐", "Apache"],
  [["caddy", "haproxy", "traefik"], "🌐", "Proxy"],
  [["postgres"], "🗄️", "PostgreSQL"],
  [["mysql", "mariadb"], "🗄️", "MySQL"],
  [["redis"], "🗄️", "Redis"],
  [["mongo"], "🗄️", "MongoDB"],
  [["gunicorn"], "⚙️", "Gunicorn"],
  [["uvicorn"], "⚙️", "Uvicorn"],
  [["celery"], "🧱", "Celery"],
  [["pm2"], "⚙️", "PM2"],
  [["supervisord"], "⚙️", "Supervisor"],
];

function processIcon(cmd: string): string {
  const lower = cmd.toLowerCase();
  return PROCESS_MATCHERS.find(([keys]) => keys.some((k) => lower.includes(k)))?.[1] ?? "🔹";
}

function processType(cmd: string): string {
  const lower = cmd.toLowerCase();
  return PROCESS_MATCHERS.find(([keys]) => keys.some((k) => lower.includes(k)))?.[2] ?? "Process";
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
  return cwd === "/" || cwd === "/root";
}
