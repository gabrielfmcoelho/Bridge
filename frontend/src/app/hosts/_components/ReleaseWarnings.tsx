"use client";

import StatusAlert from "@/components/ui/StatusAlert";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import type { ReleaseRisks, RiskWarning } from "@/lib/types";

/** Release warnings worded by code, each with its first few items. */
export function RiskList({ warnings }: { warnings: RiskWarning[] }) {
  const { t } = useLocale();
  return (
    <ul className="space-y-1.5">
      {warnings.map((w) => (
        <li key={w.code}>
          {t(`host.osRelease.risk.${w.code}`, { count: String(w.count) })}
          {w.detail && <span className="block text-xs font-mono break-all opacity-80">{w.detail}</span>}
        </li>
      ))}
    </ul>
  );
}

/** What the host's latest scan says a release would trip over, and how old
 *  that scan is. Warnings only: a release re-checks right before it starts,
 *  and they never stop it. */
export default function ReleaseWarnings({ risks, scannedAt }: { risks?: ReleaseRisks | null; scannedAt?: string }) {
  const { t, locale } = useLocale();
  const when = scannedAt ? getTimeAgo(scannedAt, locale) : "–";
  if (!risks) return <p className="text-xs text-[var(--text-muted)]">{t("host.osRelease.risk.notChecked")}</p>;
  const warnings = risks.warnings ?? [];
  if (warnings.length === 0) return <p className="text-xs text-[var(--text-muted)]">{t("host.osRelease.risk.none", { when })}</p>;
  return (
    <StatusAlert variant="warning">
      <p className="font-medium mb-1.5">{t("host.osRelease.risk.title", { when })}</p>
      <RiskList warnings={warnings} />
      <p className="text-xs mt-2 opacity-80">{t("host.osRelease.risk.hint")}</p>
    </StatusAlert>
  );
}
