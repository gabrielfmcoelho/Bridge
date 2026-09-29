"use client";

import { RowGroupTitle } from "@/components/ui/RowList";
import { splitReleases, isOverdue, RELEASE_TONE } from "@/lib/releases";
import type { Release } from "@/lib/types";

type R = Release & { issue_ids?: number[] };

/**
 * A project's releases as a timeline: what's planned (nearest first, overdue
 * flagged), then what was reached (latest first), then the canceled ones.
 * One vertical rail per lane; the dot carries the status color.
 */
export default function ReleaseTimeline({ releases, onSelect, t, locale }: {
  releases: R[];
  onSelect?: (r: R) => void;
  t: (key: string, vars?: Record<string, string>) => string;
  locale: string;
}) {
  const { planned, achieved, canceled } = splitReleases(releases);
  const fmt = (d: string) => (d ? new Date(`${d}T00:00:00`).toLocaleDateString(locale, { day: "2-digit", month: "short", year: "numeric" }) : "");

  const lane = (title: string, rows: R[]) => rows.length > 0 && (
    <section>
      <RowGroupTitle>{title}</RowGroupTitle>
      <ol className="relative mx-5 mb-3 border-l border-[var(--border-subtle)]">
        {rows.map((r) => {
          const late = isOverdue(r);
          const tone = late ? "var(--warning)" : RELEASE_TONE[r.status] ?? "var(--text-muted)";
          const date = r.status === "live" ? r.live_date : r.target_date;
          const Body = onSelect ? "button" : "div";
          return (
            <li key={r.id} className="relative pl-5">
              <span aria-hidden className="absolute -left-[5px] top-3.5 w-[9px] h-[9px] rounded-full ring-2 ring-[var(--bg-surface)]" style={{ background: tone }} />
              <Body
                {...(onSelect ? { type: "button" as const, onClick: () => onSelect(r) } : {})}
                className={`block w-full text-left py-2.5 px-2 -mx-2 rounded-[var(--radius-md)] ${onSelect ? "hover:bg-[var(--bg-elevated)] transition-colors duration-100" : ""} ${r.status === "canceled" ? "opacity-60" : ""}`}
              >
                <span className="flex items-baseline justify-between gap-3">
                  <span className="text-sm font-medium text-[var(--text-primary)] truncate">{r.title}</span>
                  <span className="text-xs tabular-nums shrink-0 text-[var(--text-muted)]">{fmt(date) || t("release.noDate")}</span>
                </span>
                <span className="flex gap-3 text-2xs mt-0.5">
                  <span style={{ color: tone }}>{late ? t("release.overdue") : t(`release.${r.status}`)}</span>
                  {!!r.issue_ids?.length && (
                    <span className="text-[var(--text-muted)]">{r.issue_ids.length} {r.issue_ids.length === 1 ? t("release.issueSingular") : t("release.issuePlural")}</span>
                  )}
                </span>
                {r.description && (
                  // Plain excerpt: the row is a button, so no block markdown inside.
                  <span className="block mt-1 line-clamp-2 text-xs text-[var(--text-secondary)]">{r.description.replace(/[#*_`>[\]]/g, "")}</span>
                )}
              </Body>
            </li>
          );
        })}
      </ol>
    </section>
  );

  return (
    <div className="pb-1">
      {lane(t("release.planned"), planned)}
      {lane(t("release.achieved"), achieved)}
      {lane(t("release.canceledGroup"), canceled)}
    </div>
  );
}
