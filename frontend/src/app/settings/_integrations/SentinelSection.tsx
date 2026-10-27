"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { alertSettingsAPI, sentinelAPI } from "@/lib/api";
import type { AlertThresholds } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import Input from "@/components/ui/Input";
import Button from "@/components/ui/Button";
import StatusDot from "@/components/ui/StatusDot";
import StatusAlert from "@/components/ui/StatusAlert";
import SectionHeading from "@/components/ui/SectionHeading";
import { tableClasses } from "@/components/ui/Table";
import IntegrationCard from "./IntegrationCard";
import { useIntegrationForm } from "./useIntegrationForm";

type TrafficKey = "traffic_5xx_warning" | "traffic_5xx_critical" | "latency_p95_warning" | "latency_p95_critical";
const TRAFFIC_KEYS: TrafficKey[] = ["traffic_5xx_warning", "traffic_5xx_critical", "latency_p95_warning", "latency_p95_critical"];

/**
 * Coolify Sentinel: on/off and cache, the traffic alert thresholds, and every
 * Coolify-linked host with its last sync — plus the manual sync itself.
 * Tokens come from Coolify's API on each read, so there's nothing secret here.
 */
export default function SentinelSection() {
  const { t, locale } = useLocale();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const f = useIntegrationForm("sentinel");
  const enabled = f.form.sentinel_enabled === "true";

  const hosts = useQuery({ queryKey: ["sentinel-hosts"], queryFn: sentinelAPI.hosts, enabled });
  const sync = useMutation({
    mutationFn: sentinelAPI.sync,
    onSuccess: (rep) => {
      queryClient.invalidateQueries({ queryKey: ["sentinel-hosts"] });
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
      queryClient.invalidateQueries({ queryKey: ["services"] });
      flag({
        appearance: rep.ok === rep.hosts.length ? "success" : "warning",
        title: t("sentinel.syncDone", { ok: String(rep.ok), total: String(rep.hosts.length) }),
      });
    },
    onError: (e: Error) => flag({ appearance: "error", title: t("sentinel.syncFailed"), description: e.message }),
  });

  const thresholds = useQuery({ queryKey: ["alert-thresholds"], queryFn: alertSettingsAPI.get, enabled });
  const [edits, setEdits] = useState<Partial<Record<TrafficKey, string>>>({});
  const saveThresholds = useMutation({
    mutationFn: (body: Partial<AlertThresholds>) => alertSettingsAPI.update(body),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["alert-thresholds"] });
      setEdits({});
      flag({ appearance: "success", title: t("settings.integrations.saved") });
    },
  });
  const value = (k: TrafficKey) => edits[k] ?? String(thresholds.data?.[k] ?? "");
  const invalid = TRAFFIC_KEYS.some((k) => edits[k] !== undefined && !/^\d+$/.test(edits[k]!));

  const when = (iso?: string) =>
    iso ? new Date(iso).toLocaleString(locale, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" }) : "–";

  return (
    <IntegrationCard
      form={f}
      title="Coolify Sentinel"
      hint={t("settings.integrations.sentinel.hint")}
      enabledKey="sentinel_enabled"
      toggleLabel={t("settings.integrations.sentinel.ariaEnable")}
    >
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Input
          label={t("settings.integrations.sentinel.cacheSeconds")}
          type="number"
          min={0}
          value={f.form.sentinel_cache_seconds ?? ""}
          onChange={(e) => f.set("sentinel_cache_seconds", e.target.value)}
          placeholder="60"
          hint={t("settings.integrations.sentinel.cacheHint")}
        />
      </div>

      <div className="space-y-3">
        <SectionHeading variant="label" as="h3">{t("settings.integrations.sentinel.thresholds")}</SectionHeading>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {TRAFFIC_KEYS.map((k) => (
            <Input
              key={k}
              label={t(`settings.integrations.sentinel.${k}`)}
              type="number"
              min={0}
              value={value(k)}
              onChange={(e) => setEdits((s) => ({ ...s, [k]: e.target.value }))}
            />
          ))}
        </div>
        <div className="flex justify-end">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={Object.keys(edits).length === 0 || invalid}
            loading={saveThresholds.isPending}
            onClick={() => saveThresholds.mutate(Object.fromEntries(Object.entries(edits).map(([k, v]) => [k, Number(v)])))}
          >
            {t("settings.integrations.sentinel.saveThresholds")}
          </Button>
        </div>
        {saveThresholds.error && <StatusAlert variant="error">{saveThresholds.error.message}</StatusAlert>}
      </div>

      <div className="space-y-2">
        <SectionHeading
          variant="label"
          as="h3"
          actions={
            <Button type="button" size="sm" loading={sync.isPending} disabled={f.dirty} onClick={() => sync.mutate()}>
              {t("sentinel.syncNow")}
            </Button>
          }
        >
          {t("settings.integrations.sentinel.hosts")}
        </SectionHeading>
        {f.dirty && <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.sentinel.saveFirst")}</p>}
        <div className="overflow-x-auto">
          <table className={tableClasses.compact.table}>
            <thead>
              <tr className={tableClasses.compact.headRow}>
                <th className={tableClasses.compact.th}>{t("host.nickname")}</th>
                <th className={tableClasses.compact.th}>{t("sentinel.state")}</th>
                <th className={tableClasses.compact.th}>{t("sentinel.metricsTraffic")}</th>
                <th className={tableClasses.compact.th}>{t("sentinel.lastSyncShort")}</th>
              </tr>
            </thead>
            <tbody>
              {(hosts.data ?? []).map((h) => {
                const s = h.snapshot;
                return (
                  <tr key={h.id} className={tableClasses.compact.row}>
                    <td className={tableClasses.compact.td}>
                      <Link href={`/hosts/${h.slug}`} className="hover:underline">{h.nickname}</Link>
                      <span className="ml-2 font-mono text-xs text-[var(--text-muted)]">{h.hostname}</span>
                    </td>
                    <td className={tableClasses.compact.td} title={s?.detail}>
                      <span className="inline-flex items-center gap-1.5 text-xs">
                        <StatusDot color={!s ? "muted" : s.state === "ok" ? "success" : "warning"} size="xs" />
                        {s ? t(`sentinel.reason.${s.state}`) : t("sentinel.neverSynced")}
                      </span>
                    </td>
                    <td className={`${tableClasses.compact.td} text-xs`}>
                      {s ? `${s.metrics_enabled ? "✓" : "–"} / ${s.traffic_enabled ? "✓" : "–"}` : "–"}
                    </td>
                    <td className={`${tableClasses.compact.td} text-xs tabular-nums`}>{when(s?.synced_at)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </IntegrationCard>
  );
}
