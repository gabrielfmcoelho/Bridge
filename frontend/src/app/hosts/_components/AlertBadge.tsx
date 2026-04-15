"use client";

import Tooltip from "@/components/ui/Tooltip";
import { ALERT_DOT_COLOR, ALERT_TEXT_COLOR } from "./alert-colors";
import type { HostAlert, AlertLevel } from "@/lib/types";

export default function AlertBadge({ alerts }: { alerts?: HostAlert[] }) {
  if (!alerts || alerts.length === 0) return null;

  const worstLevel: AlertLevel = alerts.some(a => a.level === "critical")
    ? "critical"
    : alerts.some(a => a.level === "warning")
    ? "warning"
    : "info";

  return (
    <Tooltip
      content={
        <div className="space-y-1.5">
          {alerts.map((a, i) => (
            <div key={i} className="flex items-start gap-2">
              <span className={`w-2 h-2 rounded-full mt-1 shrink-0 ${ALERT_DOT_COLOR[a.level]}`} />
              <div>
                <span className={`text-[10px] font-semibold uppercase ${ALERT_TEXT_COLOR[a.level]}`}>{a.level}</span>
                <p className="text-xs text-[var(--text-secondary)]">{a.message}</p>
              </div>
            </div>
          ))}
        </div>
      }
      side="bottom"
    >
      <span className={`inline-flex items-center gap-0.5 text-[10px] font-semibold cursor-help ${ALERT_TEXT_COLOR[worstLevel]}`}>
        <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
        </svg>
        {alerts.length}
      </span>
    </Tooltip>
  );
}
