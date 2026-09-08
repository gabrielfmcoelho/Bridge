"use client";

import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import { statusColor, statusLabelKey } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { RequestEvent, RequestStatus } from "@/lib/types";

// One chronological rail for comments *and* audit — the backend deliberately
// stores both in service_request_events with a `kind` discriminator, because
// the user reads them as a single history. Kept local to this route per the
// plan; promote to components/ui/ only when a second consumer appears.

// The dot colour carries the same meaning as the kind badge, so a scan down
// the rail reads without the labels.
const KIND_DOT: Record<RequestEvent["kind"], string> = {
  comment: "var(--text-faint)",
  status: "var(--accent)",
  assign: "var(--text-muted)",
  link: "var(--text-muted)",
  delivery: "var(--success)",
  integration: "var(--warning)",
};

export default function RequestTimeline({ events }: { events: RequestEvent[] }) {
  const { t, formatDateTime } = useLocale();

  if (events.length === 0) {
    return <EmptyState icon="folder" title={t("requests.timelineEmpty")} compact />;
  }

  return (
    <ol className="relative space-y-5 pl-6">
      {/* The rail itself. aria-hidden: it is decoration; the <ol> already
          conveys "ordered sequence" to a screen reader. */}
      <span
        aria-hidden
        className="absolute left-[3px] top-1.5 bottom-1.5 w-px bg-[var(--border-subtle)]"
      />

      {events.map((e) => (
        <li key={e.id} className="relative">
          <span
            aria-hidden
            className="absolute -left-6 top-1.5 w-[7px] h-[7px] rounded-full ring-2 ring-[var(--bg-surface)]"
            style={{ backgroundColor: KIND_DOT[e.kind] }}
          />

          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="text-sm font-medium text-[var(--text-primary)]">
              {e.user_name || t("requests.timelineSystem")}
            </span>

            {e.kind === "status" ? (
              <span className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
                {e.from_status ? (
                  <>
                    {t(statusLabelKey(e.from_status as RequestStatus))}
                    <span aria-hidden>→</span>
                  </>
                ) : (
                  t("requests.timelineOpened")
                )}
                <Badge color={statusColor(e.to_status as RequestStatus)}>
                  {t(statusLabelKey(e.to_status as RequestStatus))}
                </Badge>
              </span>
            ) : (
              <Badge color="gray">{t(`requests.event.${e.kind}`)}</Badge>
            )}

            <time dateTime={e.created_at} className="text-xs text-[var(--text-faint)] ml-auto">
              {formatDateTime(e.created_at)}
            </time>
          </div>

          {e.body && (
            <p className="mt-1.5 text-sm text-[var(--text-secondary)] whitespace-pre-wrap break-words">
              {e.body}
            </p>
          )}
        </li>
      ))}
    </ol>
  );
}
