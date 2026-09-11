"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { grafanaAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface Props {
  serviceId: number;
  nickname?: string;
}

export default function ServiceMetricsTab({ serviceId, nickname }: Props) {
  const { t } = useLocale();
  const [iframeError, setIframeError] = useState(false);
  const [iframeLoaded, setIframeLoaded] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ["grafana-embed-url", "service", serviceId],
    queryFn: () => grafanaAPI.embedURL("service", serviceId),
    retry: false,
  });

  if (isLoading) {
    return <Skeleton className="h-[600px] w-full rounded-[var(--radius-md)]" />;
  }

  if (error) {
    return (
      <EmptyState
        icon="box"
        title={t("service.grafanaNotAvailable")}
        description={error instanceof Error ? error.message : t("service.unknownError")}
        compact
      />
    );
  }

  if (!data?.configured || !data.url) {
    return (
      <EmptyState
        icon="box"
        title={t("service.noDashboardConfigured")}
        description={t("service.noDashboardConfiguredDesc")}
        compact
      />
    );
  }

  return (
    <div className="space-y-3 animate-fade-in">
      <Card hover={false} className="!p-3 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs text-[var(--text-muted)]">{t("service.dashboardFieldLabel")}</p>
          <p className="text-sm font-mono truncate text-[var(--text-primary)]">{data.dashboard_uid}</p>
        </div>
        <a
          href={data.url.replace("&kiosk", "")}
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs text-[var(--accent)] hover:underline shrink-0 inline-flex items-center gap-1"
        >
          {t("service.openInGrafana")}
          <Icon path={ICON_PATHS.externalLink} className="w-3 h-3" />
        </a>
      </Card>

      {iframeError && (
        <Card accent="amber" hover={false}>
          <p className="text-sm text-[var(--warning)]">
            {t("service.embedBlockedBefore")}
            <code className="mx-1 text-[var(--text-secondary)]">allow_embedding</code>
            {t("service.embedBlockedAfter")}
          </p>
        </Card>
      )}

      <div className="relative w-full" style={{ aspectRatio: "16 / 10", minHeight: "600px" }}>
        {!iframeLoaded && !iframeError && (
          <div className="absolute inset-0 flex items-center justify-center bg-[var(--bg-elevated)] rounded-[var(--radius-md)]">
            <p className="text-xs text-[var(--text-muted)] animate-pulse">{t("service.loadingDashboard")}</p>
          </div>
        )}
        <iframe
          key={data.url}
          src={data.url}
          className="w-full h-full rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)]"
          sandbox="allow-scripts allow-same-origin allow-popups"
          onLoad={() => setIframeLoaded(true)}
          onError={() => setIframeError(true)}
          title={t("service.grafanaDashboardForTitle", { name: nickname ?? "service" })}
        />
      </div>
    </div>
  );
}
