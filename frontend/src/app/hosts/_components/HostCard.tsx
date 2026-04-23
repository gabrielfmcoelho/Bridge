"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import ScanIndicator from "./ScanIndicator";
import { hasPermissionDeniedMessage } from "@/lib/utils";
import type { Host } from "@/lib/types";

export default function HostCard({ host }: { host: Host }) {
  const { t } = useLocale();
  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const situacaoColor = situacoes.find((s) => s.value === host.situacao)?.color;
  const fallbackColor = host.situacao === "active" ? "#10b981" : host.situacao === "maintenance" ? "#f59e0b" : "#6b7280";
  const sr = host.scan_resources;
  const hasCleanResources = host.has_scan && sr &&
    !hasPermissionDeniedMessage(sr.cpu) &&
    !hasPermissionDeniedMessage(sr.ram) &&
    !hasPermissionDeniedMessage(sr.storage) &&
    (sr.cpu || sr.ram || sr.storage);

  const mainResp = host.responsaveis?.find((r) => r.is_main);

  const authIconColor = (has: boolean, status?: "success" | "failed" | null) => {
    if (!has) return "text-[var(--text-faint)]/30";
    if (status === "success") return "text-emerald-400";
    if (status === "failed") return "text-red-400";
    return "text-[var(--text-faint)]";
  };

  return (
    <Link href={`/hosts/${host.oficial_slug}`}>
      <Card className="h-full border-l-[3px] flex flex-col overflow-hidden" style={{ borderLeftColor: situacaoColor || fallbackColor }} clickIndicator="link">
        <CardHeader
          title={host.nickname}
          subtitle={host.oficial_slug}
          description={host.description || t("common.noDescription")}
          badge={
            <Badge variant="situacao" situacao={host.situacao} compact>
              {host.situacao}
            </Badge>
          }
        />

        <CardMetadataGrid
          items={[
            { label: t("host.hostname"), value: host.hostname || "-", mono: true },
            { label: t("host.hospedagem"), value: host.hospedagem || "-" },
            { label: t("dns.responsavel"), value: mainResp?.name || host.main_responsavel_name || "-" },
            { label: t("host.entity"), value: mainResp?.entity || "-" },
          ]}
        />

        <CardTagsSection tags={host.tags} />

        {/* Resources — unique to hosts */}
        <div className="mt-3 pt-3 pb-1 border-t border-[var(--border-subtle)]">
          {hasCleanResources && sr ? (
            <div className="grid grid-cols-3 gap-3">
              <MiniResource label="CPU" value={sr.cpu} usage={sr.cpu_usage} />
              <MiniResource label="RAM" value={sr.ram} usage={sr.ram_percent} />
              <MiniResource label="Disk" value={sr.storage} usage={sr.disk_percent} />
            </div>
          ) : (
            <div className="grid grid-cols-3 gap-3">
              {["CPU", "RAM", "Disk"].map((label) => (
                <div key={label}>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs text-[var(--text-faint)] uppercase tracking-wider">{label}</span>
                    <span className="text-xs text-[var(--text-faint)]">--</span>
                  </div>
                  <div className="h-1.5 rounded-full bg-[var(--bg-elevated)]" />
                  <p className="text-xs text-[var(--text-faint)] mt-0.5 text-right">&nbsp;</p>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Bottom indicators */}
        <div className="flex flex-col gap-2 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4">
          {/* Row 1: scan, dns, containers, processes, services, projects */}
          <div className="grid grid-cols-[repeat(6,2.25rem)] justify-start gap-x-2">
            <ScanIndicator hasScan={host.has_scan} lastScanAt={host.last_scan_at} />
            <CardIndicator icon={ICON_PATHS.globe} count={host.dns_count || 0} color="cyan" title={`${host.dns_count || 0} DNS`} />
            <CardIndicator icon={ICON_PATHS.container} count={host.containers_count || 0} color="sky" title={`${host.containers_count || 0} ${t("host.containers").toLowerCase()}`} />
            <CardIndicator icon={ICON_PATHS.terminal} count={host.processes_count || 0} color="violet" title={`${host.processes_count || 0} ${t("host.processes").toLowerCase()}`} />
            <CardIndicator icon={ICON_PATHS.gear} count={host.services_count || 0} color="amber" title={`${host.services_count || 0} ${t("host.services").toLowerCase()}`} />
            <CardIndicator icon={ICON_PATHS.folder} count={host.projects_count || 0} color="violet" title={`${host.projects_count || 0} projetos`} />
          </div>

          {/* Row 2: auth (pwd, key), chamados, alerts, issues */}
          <div className="grid grid-cols-[repeat(6,2.25rem)] justify-start gap-x-2">
            <div className="flex items-center" title={`Password: ${host.has_password ? (host.password_test_status || "untested") : "none"}`}>
              <svg className={`w-3.5 h-3.5 ${authIconColor(host.has_password, host.password_test_status)}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d={ICON_PATHS.lock} />
              </svg>
            </div>
            <div className="flex items-center" title={`Key: ${host.has_key ? (host.key_test_status || "untested") : "none"}`}>
              <svg className={`w-3.5 h-3.5 ${authIconColor(host.has_key, host.key_test_status)}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d={ICON_PATHS.key} />
              </svg>
            </div>
            <CardIndicator icon={ICON_PATHS.document} count={host.chamados_count || 0} color="orange" title={`${host.chamados_count || 0} chamados`} />
            <CardIndicator icon={ICON_PATHS.alert} count={host.alerts?.length || 0} color="amber" title={`${host.alerts?.length || 0} alerts`} />
            <CardIndicator icon={ICON_PATHS.clipboard} count={host.issues_count || 0} color="purple" title={`${host.issues_count || 0} issues`} />
          </div>
        </div>
      </Card>
    </Link>
  );
}

function MiniResource({ label, value, usage }: { label: string; value?: string; usage?: string }) {
  if (!value || typeof value !== 'string') return null;
  if (!value.trim() || value.toLowerCase().includes('bash') || value.toLowerCase().includes('permission') || value.toLowerCase().includes('/dev/null')) return null;
  if (usage && (usage.toLowerCase().includes('bash') || usage.toLowerCase().includes('permission') || usage.toLowerCase().includes('/dev/null'))) return null;

  const pct = parseInt(usage || "0") || 0;
  const color = pct >= 80 ? "text-red-400" : pct >= 50 ? "text-amber-400" : "text-emerald-400";
  const barColor = pct >= 80 ? "bg-red-500" : pct >= 50 ? "bg-amber-500" : "bg-emerald-500";
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-[var(--text-faint)] uppercase tracking-wider">{label}</span>
        {usage && <span className={`text-xs font-semibold ${color}`} style={{ fontFamily: "var(--font-mono)" }}>{usage.includes("%") ? usage : `${usage}%`}</span>}
      </div>
      <div className="h-1.5 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
        <div className={`h-full rounded-full ${barColor} transition-all`} style={{ width: `${Math.min(pct, 100)}%` }} />
      </div>
      <p className="text-xs text-[var(--text-muted)] mt-0.5 text-right" style={{ fontFamily: "var(--font-mono)" }}>{value}</p>
    </div>
  );
}
