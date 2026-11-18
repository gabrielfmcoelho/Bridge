"use client";

import { useMemo } from "react";
import SectionCard from "@/components/ui/SectionCard";
import StatusDot from "@/components/ui/StatusDot";
import Badge from "@/components/ui/Badge";
import type { ProcessDetail, VMInfoType } from "@/lib/api";
import { scanWorkloads, type Workload } from "@/lib/scanSections";
import { Rows, Row, Detail, SecretText, pctColor, type T } from "./shared";

const KIND_COLOR = { container: "cyan", service: "success", process: "info" } as const;

/**
 * What runs on the host, one row per workload (lib/scanSections
 * `scanWorkloads`): a container, a classified service with the processes
 * behind it, or a process nothing else claims — each with the ports it
 * listens on and its CPU / memory. A row expands to that workload's details.
 * OS housekeeping processes come last, dimmed.
 */
export default function ScanWorkloads({ info, t, hostSlug }: { info: VMInfoType; t: T; hostSlug: string }) {
  const { workloads } = useMemo(() => scanWorkloads(info), [info]);

  return (
    <SectionCard
      as="h3"
      title={t("scan.pane.group.workloads")}
      count={workloads.length || undefined}
      body="flush"
      empty={workloads.length === 0 ? t("scan.pane.empty.workloads") : undefined}
    >
      <Rows>
        {workloads.map((w) => (
          <Row
            key={w.key}
            muted={w.system || !w.running}
            summary={
              <>
                <StatusDot size="xs" color={w.running ? "success" : "muted"} title={w.container?.container?.status ?? w.service?.state} />
                <span className="w-20 shrink-0"><Badge color={KIND_COLOR[w.kind]}>{t(`scan.workloads.kind.${w.kind}`)}</Badge></span>
                <span className="shrink-0 max-w-[14rem] truncate font-mono font-medium text-[var(--text-primary)]" title={w.name}>{w.name}</span>
                {w.system && <Badge>{t("vm.os")}</Badge>}
                <span className="flex-1 min-w-0 truncate font-mono text-[var(--text-muted)]"><Secondary w={w} hostSlug={hostSlug} /></span>
                {w.ports.map((p) => (
                  <span key={p} className="shrink-0 px-1.5 py-0.5 rounded-full border text-2xs font-mono bg-[var(--cyan)]/15 text-[var(--cyan)] border-[var(--cyan)]/40" title={t("scan.listeningPorts")}>:{p}</span>
                ))}
                {(w.cpu || w.mem) && (
                  <span className="hidden sm:inline shrink-0 font-mono tabular-nums text-[var(--text-secondary)]" title={t("vm.cpuMemLabel")}>
                    <span className={pctColor(parseFloat(w.cpu ?? "") || 0)}>{w.cpu || "–"}</span> / {w.mem || "–"}
                  </span>
                )}
              </>
            }
          >
            {w.container && <ContainerDetails w={w} t={t} />}
            {w.service && <ServiceDetails w={w} t={t} />}
            {w.processes.map((p, i) => <ProcessDetails key={p.pid ?? i} p={p} t={t} hostSlug={hostSlug} />)}
          </Row>
        ))}
      </Rows>
    </SectionCard>
  );
}

/** The muted middle of a row: image, service kind and version, or the command line. */
function Secondary({ w, hostSlug }: { w: Workload; hostSlug: string }) {
  if (w.kind === "container") {
    const image = w.container?.container?.image;
    return <>{w.service ? `${w.service.label} · ` : ""}{image || "–"}</>;
  }
  if (w.kind === "service") {
    return <>{[w.service?.kind, w.service?.version && `v${w.service.version}`, w.service?.unit?.replace(/\.service$/, "")].filter(Boolean).join(" · ")}</>;
  }
  return <SecretText text={w.processes[0]?.command ?? ""} hostSlug={hostSlug} />;
}

function ContainerDetails({ w, t }: { w: Workload; t: T }) {
  const r = w.container!;
  const c = r.container;
  const bindings = parseContainerPortBindings(c?.ports ?? "");
  return (
    <>
      <Detail label={t("common.status")}>{c?.status || "–"}</Detail>
      <Detail label={t("scan.workloads.image")}>{c?.image || "–"}</Detail>
      {c?.id && <Detail label="ID">{c.id.slice(0, 12)}</Detail>}
      {r.net && <Detail label={t("scan.workloads.network")}>{r.net}</Detail>}
      {bindings.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {bindings.map((b, i) => (
            <span
              key={i}
              className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full border text-2xs bg-[var(--cyan)]/15 text-[var(--cyan)] border-[var(--cyan)]/40 font-mono"
              title={t("host.ops.portBindingTooltip", { hostPort: b.hostPort, containerPort: b.containerPort, proto: b.proto })}
            >
              :{b.hostPort}<span className="opacity-70">→{b.containerPort}</span>
            </span>
          ))}
        </div>
      )}
    </>
  );
}

function ServiceDetails({ w, t }: { w: Workload; t: T }) {
  const s = w.service!;
  const state = s.state ? tOr(t, `scan.services.state.${s.state}`, s.state) : "";
  return (
    <>
      {/* On a container row the service is what the container runs. */}
      {w.kind === "container" && <Detail label={t("scan.workloads.kind.service")} mono={false}>{s.label}</Detail>}
      {state && <Detail label={t("common.status")} mono={false}>{state}{s.enabled ? ` · ${t("scan.services.enabledAtBoot")}` : ""}</Detail>}
      {s.unit && <Detail label={t("scan.workloads.unit")}>{s.unit}</Detail>}
      {s.instance && <Detail label={t("scan.workloads.instance")}>{s.instance}</Detail>}
      {s.apps && s.apps.length > 0 && <Detail label={t("scan.services.apps")}>{s.apps.join(", ")}</Detail>}
      {s.version && <Detail label={t("scan.packageVersion")}>{s.package ? `${s.package} ${s.version}` : s.version}</Detail>}
      {s.vendor && <Detail label={t("scan.workloads.vendor")} mono={false}>{s.vendor}</Detail>}
      {w.kind === "service" && s.container_image && <Detail label={t("scan.workloads.alsoIn")}>{s.container_image}</Detail>}
      {s.sources && s.sources.length > 0 && <Detail label={t("scan.workloads.signals")}>{s.sources.join(", ")}</Detail>}
    </>
  );
}

function ProcessDetails({ p, t, hostSlug }: { p: Partial<ProcessDetail>; t: T; hostSlug: string }) {
  return (
    <div className="space-y-2">
      <p className="font-mono text-[var(--text-secondary)] break-all"><SecretText text={p.command ?? ""} hostSlug={hostSlug} /></p>
      {(p.user || p.pid) && <Detail label={t("vm.user")}>{[p.user, p.pid && `PID ${p.pid}`].filter(Boolean).join(" · ")}</Detail>}
      {p.started_via && (
        <Detail label={t("scan.startedVia")} mono={false}>
          <span className={p.started_via === "manual" ? "text-[var(--warning)]" : p.started_via === "systemd" ? "text-[var(--success)]" : ""}>{p.started_via}</span>
          {p.unit ? ` · ${p.unit}` : ""}
        </Detail>
      )}
      {p.ports && <Detail label={t("scan.listeningPorts")}>{p.ports}</Detail>}
      {p.cwd && <Detail label={t("vm.cwd")}>{p.cwd}</Detail>}
      {p.venv && <Detail label={t("vm.venv")}>{p.venv}</Detail>}
    </div>
  );
}

/** Translation, or `fallback` when the catalogue has no such key. */
function tOr(t: T, key: string, fallback: string): string {
  const v = t(key);
  return v === key ? fallback : v;
}

type ContainerPortBinding = { hostPort: string; containerPort: string; proto: string };

/**
 * `docker ps` Ports column → deduplicated host→container bindings, e.g.
 * "0.0.0.0:8080->80/tcp, :::8080->80/tcp". Pure exposes are ignored.
 */
function parseContainerPortBindings(raw: string): ContainerPortBinding[] {
  if (!raw) return [];
  const seen = new Set<string>();
  const out: ContainerPortBinding[] = [];
  for (const part of raw.split(",")) {
    const m = part.trim().match(/(?:[\d.]+|\[?::\]?|\*):(\d+(?:-\d+)?)->(\d+(?:-\d+)?)\/(\w+)/);
    if (!m) continue;
    const [, hostPort, containerPort, proto] = m;
    const key = `${hostPort}|${containerPort}|${proto}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ hostPort, containerPort, proto });
  }
  return out;
}
