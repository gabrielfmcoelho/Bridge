"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { grafanaAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { Skeleton } from "@/components/ui/Skeleton";
import HostLiveKpis from "./HostLiveKpis";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface Props {
  slug: string;
}

export default function HostMetricsTab({ slug }: Props) {
  const { t } = useLocale();
  const [iframeError, setIframeError] = useState(false);
  const [iframeLoaded, setIframeLoaded] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ["grafana-embed-url", "host", slug],
    queryFn: () => grafanaAPI.embedURL("host", slug),
    retry: false,
  });

  if (isLoading) {
    return (
      <div className="space-y-3 animate-fade-in">
        <HostLiveKpis slug={slug} />
        <Skeleton className="h-[600px] w-full rounded-[var(--radius-md)]" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-3 animate-fade-in">
        <HostLiveKpis slug={slug} />
        <StatusAlert variant="error">
          <p className="font-medium">{t("host.metrics.grafanaUnavailableTitle")}</p>
          <p className="text-xs">{error instanceof Error ? error.message : t("host.metrics.unknownError")}</p>
        </StatusAlert>
      </div>
    );
  }

  if (!data?.configured || !data.url) {
    return (
      <div className="space-y-3 animate-fade-in">
        <HostLiveKpis slug={slug} />
        <EmptyState
          icon="box"
          title={t("host.metrics.noDashboardTitle")}
          description={t("host.metrics.noDashboardDesc")}
          compact
        />
      </div>
    );
  }

  return (
    <div className="space-y-3 animate-fade-in">
      <HostLiveKpis slug={slug} />
      <Card hover={false} className="!p-3 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs text-[var(--text-muted)]">{t("host.metrics.dashboardLabel")}</p>
          <p className="text-sm font-mono truncate text-[var(--text-primary)]">{data.dashboard_uid}</p>
        </div>
        <a
          href={data.url.replace("&kiosk", "")}
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs text-[var(--accent)] hover:underline shrink-0 inline-flex items-center gap-1"
        >
          {t("host.metrics.openInGrafana")}
          <Icon path={ICON_PATHS.externalLink} className="w-3 h-3" />
        </a>
      </Card>

      {iframeError && (
        <StatusAlert variant="warning">
          {t("host.metrics.embedBlockedBefore")}
          <code className="mx-1 text-[var(--text-secondary)]">allow_embedding</code>
          {t("host.metrics.embedBlockedAfter")}
        </StatusAlert>
      )}

      <div className="relative w-full" style={{ aspectRatio: "16 / 10", minHeight: "600px" }}>
        {!iframeLoaded && !iframeError && (
          <div className="absolute inset-0 flex items-center justify-center bg-[var(--bg-elevated)] rounded-[var(--radius-md)]">
            <p className="text-xs text-[var(--text-muted)] animate-pulse">{t("host.metrics.loadingDashboard")}</p>
          </div>
        )}
        <iframe
          key={data.url}
          src={data.url}
          className="w-full h-full rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)]"
          sandbox="allow-scripts allow-same-origin allow-popups"
          onLoad={() => setIframeLoaded(true)}
          onError={() => setIframeError(true)}
          title={t("host.metrics.dashboardIframeTitle", { slug })}
        />
      </div>
    </div>
  );
}
