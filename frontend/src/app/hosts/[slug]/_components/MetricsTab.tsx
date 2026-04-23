"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { grafanaAPI } from "@/lib/api";
import Card from "@/components/ui/Card";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import HostLiveKpis from "./HostLiveKpis";

interface Props {
  slug: string;
}

export default function HostMetricsTab({ slug }: Props) {
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
        <EmptyState
          icon="box"
          title="Grafana not available"
          description={error instanceof Error ? error.message : "Unknown error"}
          compact
        />
      </div>
    );
  }

  if (!data?.configured || !data.url) {
    return (
      <div className="space-y-3 animate-fade-in">
        <HostLiveKpis slug={slug} />
        <EmptyState
          icon="box"
          title="No dashboard configured"
          description="Set this host's Grafana dashboard UID in the edit drawer, or configure a default in Settings → Integrations → Grafana."
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
          <p className="text-xs text-[var(--text-muted)]">Dashboard</p>
          <p className="text-sm font-mono truncate text-[var(--text-primary)]">{data.dashboard_uid}</p>
        </div>
        <a
          href={data.url.replace("&kiosk", "")}
          target="_blank"
          rel="noopener noreferrer"
          className="text-xs text-[var(--accent)] hover:underline shrink-0 inline-flex items-center gap-1"
        >
          Open in Grafana
          <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
          </svg>
        </a>
      </Card>

      {iframeError && (
        <Card accent="amber" hover={false}>
          <p className="text-sm text-amber-300">
            The dashboard took too long to load or blocked the embed. Verify Grafana's
            <code className="mx-1 text-[var(--text-secondary)]">allow_embedding</code>
            setting and that the base URL is reachable from the browser.
          </p>
        </Card>
      )}

      <div className="relative w-full" style={{ aspectRatio: "16 / 10", minHeight: "600px" }}>
        {!iframeLoaded && !iframeError && (
          <div className="absolute inset-0 flex items-center justify-center bg-[var(--bg-elevated)] rounded-[var(--radius-md)]">
            <p className="text-xs text-[var(--text-muted)] animate-pulse">Loading dashboard…</p>
          </div>
        )}
        <iframe
          key={data.url}
          src={data.url}
          className="w-full h-full rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)]"
          sandbox="allow-scripts allow-same-origin allow-popups"
          onLoad={() => setIframeLoaded(true)}
          onError={() => setIframeError(true)}
          title={`Grafana dashboard for ${slug}`}
        />
      </div>
    </div>
  );
}
