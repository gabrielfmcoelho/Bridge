"use client";

import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { sshAPI, type VMInfoType } from "@/lib/api";
import { getTimeAgo, resolveAuthMethod } from "@/lib/utils";
import type { Host } from "@/lib/types";
import VMInfoDisplay, { SCAN_GROUPS, type ScanGroup } from "./VMInfoDisplay";

const ANCHOR = "scan-";

/**
 * Right column of the host overview: what the last scan observed. Header
 * (when, and "Executar scan" for editors), a sticky jump bar over the eight
 * groups, then the groups in one continuous scroll.
 */
export default function ScanPane({ slug, host, lastScan, canEdit }: {
  slug: string;
  host: Host;
  lastScan: { data: string; scanned_at: string } | null | undefined;
  canEdit: boolean;
}) {
  const { t, locale, formatDateTime } = useLocale();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const method = resolveAuthMethod(host.has_password, host.has_key, host.preferred_auth);

  const scan = useMutation({
    mutationFn: () => sshAPI.testConnection(slug, method!, true),
    onMutate: () => flag({ appearance: "info", title: t("scan.pane.running") }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["host", slug] });
      flag(res.success
        ? { appearance: "success", title: t("scan.pane.done") }
        : { appearance: "error", title: t("scan.pane.failed"), description: res.error });
    },
    onError: (err: Error) => flag({ appearance: "error", title: t("scan.pane.failed"), description: err.message }),
  });

  const parsed = ((): { info?: VMInfoType; error?: boolean } => {
    if (!lastScan?.data) return {};
    try {
      return { info: JSON.parse(lastScan.data) };
    } catch {
      return { error: true };
    }
  })();

  const runButton = canEdit && (
    <Button
      size="sm"
      variant="secondary"
      onClick={() => scan.mutate()}
      loading={scan.isPending}
      disabled={!method}
      title={!method ? t("scan.pane.noCredentials") : undefined}
    >
      {t("scan.pane.run")}
    </Button>
  );

  if (!lastScan?.data) {
    return (
      <section aria-label={t("scan.pane.title")} className="min-w-0">
        <EmptyState compact icon="search" title={t("scan.pane.noneTitle")} description={t("scan.pane.noneDescription")} action={runButton || undefined} />
      </section>
    );
  }

  return (
    <section aria-label={t("scan.pane.title")} className="min-w-0">
      <div className="flex items-center justify-between gap-3 mb-2">
        <p className="text-xs text-[var(--text-muted)]" title={formatDateTime(lastScan.scanned_at)}>
          {t("scan.pane.ago", { ago: getTimeAgo(lastScan.scanned_at, locale) })}
        </p>
        {runButton}
      </div>
      {parsed.error || !parsed.info ? (
        <StatusAlert variant="error">{t("scan.pane.parseError")}</StatusAlert>
      ) : (
        <>
          <JumpBar t={t} />
          <VMInfoDisplay info={parsed.info} locale={locale} anchorPrefix={ANCHOR} />
        </>
      )}
    </section>
  );
}

/** Sticky chips, one per group; the group in view is marked `aria-current`. */
function JumpBar({ t }: { t: (k: string) => string }) {
  const [active, setActive] = useState<ScanGroup>(SCAN_GROUPS[0]);

  useEffect(() => {
    // A group counts as "in view" while it crosses the top 40% of the viewport;
    // the first such group (in page order) is the active one.
    const visible = new Set<string>();
    const obs = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          const g = e.target.id.slice(ANCHOR.length);
          if (e.isIntersecting) visible.add(g);
          else visible.delete(g);
        }
        const first = SCAN_GROUPS.find((g) => visible.has(g));
        if (first) setActive(first);
      },
      { rootMargin: "-64px 0px -60% 0px" },
    );
    for (const g of SCAN_GROUPS) {
      const el = document.getElementById(ANCHOR + g);
      if (el) obs.observe(el);
    }
    return () => obs.disconnect();
  }, []);

  return (
    <nav aria-label={t("scan.pane.sections")} className="sticky top-0 z-10 -mx-1 px-1 py-2 mb-3 bg-[var(--bg-base)]">
      <div className="flex gap-1.5 overflow-x-auto [scrollbar-width:none]">
        {SCAN_GROUPS.map((g) => {
          const on = g === active;
          return (
            <a
              key={g}
              href={`#${ANCHOR}${g}`}
              aria-current={on ? "true" : undefined}
              onClick={(e) => {
                e.preventDefault();
                setActive(g);
                document.getElementById(ANCHOR + g)?.scrollIntoView({
                  behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth",
                  block: "start",
                });
              }}
              className={`shrink-0 whitespace-nowrap rounded-full border px-3 py-1 text-xs transition-colors ${
                on
                  ? "border-[var(--accent)]/40 bg-[var(--accent)]/10 text-[var(--accent)]"
                  : "border-[var(--border-subtle)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:border-[var(--border-default)]"
              }`}
            >
              {t(`scan.pane.group.${g}`)}
            </a>
          );
        })}
      </div>
    </nav>
  );
}
