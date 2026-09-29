"use client";


import IconButton from "@/components/ui/IconButton";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import SectionCard from "@/components/ui/SectionCard";
import type { HostAlert } from "@/lib/types";
import { ALERT_DOT_COLOR, LEVEL_ORDER } from "@/lib/alert-colors";
import { useLocale } from "@/contexts/LocaleContext";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

/* ─── Alerts Section ─── */

export function AlertsSection({ alerts, onAlertClick, addButton, showResolved, onToggleResolved, hasResolved }: {
  alerts: HostAlert[];
  onAlertClick: (alert: HostAlert) => void;
  addButton?: React.ReactNode;
  showResolved?: boolean;
  onToggleResolved?: () => void;
  hasResolved?: boolean;
}) {
  const { t } = useLocale();
  // ponytail: fixed order (most severe first) — a host has a handful of alerts, no sort/view controls needed.
  const sorted = [...alerts].sort((a, b) => (LEVEL_ORDER[a.level] ?? 2) - (LEVEL_ORDER[b.level] ?? 2));

  return (
    <SectionCard as="h3" title={t("alert.title")} count={alerts.length} body="flush" empty={alerts.length === 0 ? t("alert.noAlertsDesc") : undefined} controls={
        <>
          {hasResolved && onToggleResolved && (
            <IconButton
              variant={showResolved ? "active" : "default"}
              onClick={onToggleResolved}
              label={showResolved ? t("alert.hideResolved") : t("alert.showResolved")}
            >
              <Icon path={showResolved ? ICON_PATHS.eyeOff : ICON_PATHS.eye} />
            </IconButton>
          )}
          {addButton}
        </>
      }>
      <RowList>
        {sorted.map((alert, i) => {
          const resolved = alert.status === "resolved";
          return (
            <ListRow key={alert.id ?? `auto-${i}`} onClick={() => onAlertClick(alert)}>
              {resolved
                ? <Icon path={ICON_PATHS.checkCircle} className="w-3.5 h-3.5 shrink-0 text-[var(--success)]" />
                : <span className={`w-2 h-2 rounded-full shrink-0 ${ALERT_DOT_COLOR[alert.level]}`} title={alert.level} />}
              <RowText
                title={<span className={resolved ? "line-through" : ""}>{alert.message}</span>}
                muted={resolved}
                meta={<>
                  <span className="font-mono truncate">{alert.type}</span>
                  <span>{alert.source === "grafana" ? "Grafana" : alert.source === "manual" ? t("alert.manual") : t("alert.auto")}</span>
                </>}
              />
              <span title={alert.linked_issue_id ? t("issue.number", { id: String(alert.linked_issue_id) }) : t("alert.noIssueLinked")}>
                <Icon path={ICON_PATHS.clipboard} className={`w-3.5 h-3.5 shrink-0 ${alert.linked_issue_id ? "text-[var(--accent)]" : "text-[var(--text-faint)]/40"}`} />
              </span>
            </ListRow>
          );
        })}
      </RowList>
    </SectionCard>
  );
}
