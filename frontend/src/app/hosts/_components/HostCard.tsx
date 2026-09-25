"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import { situacaoAccent } from "@/lib/constants";
import SituacaoText from "@/components/ui/SituacaoText";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import ScanIndicator from "./ScanIndicator";
import { hasPermissionDeniedMessage } from "@/lib/utils";
import { formatSize } from "@/lib/units";
import Icon from "@/components/ui/Icon";
import IconButton from "@/components/ui/IconButton";
import { QuickLookOutlet } from "./HostQuickLook";
import { openQuickLook } from "./quickLookStore";
import type { Host } from "@/lib/types";

export default function HostCard({ host }: { host: Host }) {
  const { t } = useLocale();
  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const situacaoColor = situacoes.find((s) => s.value === host.situacao)?.color;
  const sr = host.scan_resources;
  // A scan that returned "permission denied" etc. counts as no data.
  const clean = !!(host.has_scan && sr &&
    !hasPermissionDeniedMessage(sr.cpu) &&
    !hasPermissionDeniedMessage(sr.ram) &&
    !hasPermissionDeniedMessage(sr.storage));

  const mainResp = host.responsaveis?.find((r) => r.is_main);

  const counts = [
    { icon: ICON_PATHS.globe, count: host.dns_count || 0, color: "success", label: "DNS" },
    { icon: ICON_PATHS.container, count: host.containers_count || 0, color: "info", label: t("host.containers") },
    { icon: ICON_PATHS.terminal, count: host.processes_count || 0, color: "accent", label: t("host.processes") },
    { icon: ICON_PATHS.gear, count: host.services_count || 0, color: "warning", label: t("host.services") },
    { icon: ICON_PATHS.folder, count: host.projects_count || 0, color: "accent", label: t("nav.projects") },
    { icon: ICON_PATHS.document, count: host.chamados_count || 0, color: "warning", label: t("nav.chamados") },
    { icon: ICON_PATHS.alert, count: host.alerts?.length || 0, color: "danger", label: t("host.alerts") },
    { icon: ICON_PATHS.clipboard, count: host.issues_count || 0, color: "accent", label: t("nav.issues") },
  ];

  return (
    <>
    {/* `group relative` anchors the quick-look button; the outlet sits outside
        the Link so clicks inside the (portalled) panel don't bubble into it. */}
    <Link href={`/hosts/${host.oficial_slug}`} className="group relative block h-full">
      <Card accent={situacaoAccent(host.situacao, situacaoColor)} className="h-full flex flex-col overflow-hidden">
        {/* Fixed anatomy (DS rule 18): every slot below always renders in the
            same place, "–"/0/dimmed when there is nothing to show. */}
        <CardHeader
          titleFont="display"
          title={host.nickname}
          subtitle={host.oficial_slug}
          status={<SituacaoText situacao={host.situacao} />}
          description={host.description}
          corner={
            <IconButton
              label={`${t("host.quickLook")}: ${host.nickname}`}
              variant="outline"
              className="shrink-0 -mr-1 -mt-1 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 group-focus-within:opacity-100 [@media(hover:none)]:opacity-100"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                openQuickLook(host);
              }}
            >
              <Icon path={ICON_PATHS.eye} />
            </IconButton>
          }
        />

        <CardMetadataGrid
          items={[
            { label: t("host.hostname"), value: host.hostname || "", mono: true },
            { label: t("host.hospedagem"), value: host.hospedagem || "" },
            { label: t("dns.responsavel"), value: mainResp?.name || host.main_responsavel_name || "" },
            { label: t("host.entity"), value: host.main_entidade || "" },
          ]}
        />

        <CardTagsSection tags={host.tags} />

        {/* Resources always show; without a (clean) scan they read "–". */}
        <div className="mt-3 pt-3 pb-1 border-t border-[var(--border-subtle)]">
          <div className="grid grid-cols-3 gap-3">
            <MiniResource label="CPU" value={clean ? sr?.cpu : undefined} usage={clean ? sr?.cpu_usage : undefined} />
            <MiniResource label="RAM" value={clean ? sr?.ram : undefined} usage={clean ? sr?.ram_percent : undefined} />
            <MiniResource label={t("vm.disk")} value={clean ? sr?.storage : undefined} usage={clean ? sr?.disk_percent : undefined} />
          </div>
        </div>

        {/* Indicators: a fixed 6×2 grid so every icon has the same cell on
            every card — links on the first row; alerts, issues, idle, scan,
            password, key on the second. All always present. */}
        <div className="grid grid-cols-6 gap-x-2 gap-y-2 mt-auto pt-4 border-t border-[var(--border-subtle)] mt-4 [&>*]:h-5 [&>*]:flex [&>*]:items-center">
          {counts.map((c) => (
            <CardIndicator key={c.label} icon={c.icon} count={c.count} color={c.color} title={`${c.count} ${c.label.toLowerCase()}`} />
          ))}
          <CardIndicator
            icon={ICON_PATHS.moon}
            count={host.idle ? 1 : 0}
            color="info"
            hideCount
            disabled={!host.has_scan}
            title={host.idle ? `${t("host.idle")}: ${(host.idle_reasons ?? []).join("; ") || t("host.idleNoWorkloads")}` : `${t("host.idle")}: ${t("host.idleNotFlaggedShort")}`}
          />
          <ScanIndicator hasScan={host.has_scan} lastScanAt={host.last_scan_at} />
          <AccessIcon icon={ICON_PATHS.lock} label={t("host.cardPasswordLabel")} has={host.has_password} none={t("host.noPassword")} status={host.password_test_status} t={t} />
          <AccessIcon icon={ICON_PATHS.key} label={t("host.cardKeyLabel")} has={host.has_key} none={t("host.noKey")} status={host.key_test_status} t={t} />
        </div>
      </Card>
    </Link>
    <QuickLookOutlet />
    </>
  );
}

function MiniResource({ label, value, usage }: { label: string; value?: string; usage?: string }) {
  const bad = (v?: string) => !v || !v.trim() || /bash|permission|\/dev\/null/i.test(v);
  const hasValue = !bad(value);
  const hasUsage = !bad(usage);
  const pct = hasUsage ? parseInt(usage!) || 0 : 0;
  const color = pct >= 80 ? "text-[var(--danger)]" : pct >= 50 ? "text-[var(--warning)]" : "text-[var(--success)]";
  const barColor = pct >= 80 ? "bg-[var(--danger)]" : pct >= 50 ? "bg-[var(--warning)]" : "bg-[var(--success)]";
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-[var(--text-muted)]">{label}</span>
        <span className={`text-xs font-semibold font-mono ${hasUsage ? color : "text-[var(--text-muted)]"}`}>
          {hasUsage ? (usage!.includes("%") ? usage : `${usage}%`) : "–"}
        </span>
      </div>
      <div className="h-1.5 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
        {hasUsage && <div className={`h-full rounded-full ${barColor} transition`} style={{ width: `${Math.min(pct, 100)}%` }} />}
      </div>
      <p className="text-xs text-[var(--text-muted)] mt-0.5 text-right font-mono">{hasValue ? formatSize(value!) : "–"}</p>
    </div>
  );
}

/** A credential slot, always shown: coloured by its last test (green ok, red
 *  failed, muted untested), or dimmed when none is stored. */
function AccessIcon({ icon, label, has, none, status, t }: { icon: string; label: string; has: boolean; none: string; status?: "success" | "failed" | null; t: (k: string) => string }) {
  const color = !has ? "text-[var(--text-faint)] opacity-40" : status === "success" ? "text-[var(--success)]" : status === "failed" ? "text-[var(--danger)]" : "text-[var(--text-muted)]";
  const text = has ? `${label} ${status || t("host.untested")}` : none;
  return (
    <span role="img" aria-label={text} title={text} className="inline-flex">
      <Icon path={icon} className={`w-3.5 h-3.5 ${color}`} />
    </span>
  );
}
