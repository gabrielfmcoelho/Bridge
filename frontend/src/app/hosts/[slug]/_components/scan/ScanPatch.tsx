"use client";

import type { ReactNode } from "react";
import SectionCard from "@/components/ui/SectionCard";
import { useLocale } from "@/contexts/LocaleContext";
import type { VMInfoType } from "@/lib/api";
import { type T } from "./shared";

/**
 * A4 patch freshness: the last successful package-index refresh, the last
 * upgrade, whether automatic updates are on and the upgradable count — enough
 * to tell "patching is broken" (auto-updates on but the index is weeks stale,
 * host 50's case) from "merely outdated". Background scan only.
 */
export default function ScanPatch({ info, t }: { info: VMInfoType; t: T }) {
  const { formatDateTime } = useLocale();
  const p = info.deep_facts?.patch;
  const empty = info.deep_facts == null
    ? t("scan.deep.notRun")
    : !p || !p.manager
      ? t("scan.pane.empty.patch")
      : undefined;

  const date = (unix?: number) =>
    unix ? formatDateTime(new Date(unix * 1000)) : t("scan.deep.patch.unknown");

  return (
    <SectionCard as="h3" title={t("scan.pane.group.patch")} empty={empty}>
      {p && !empty && (
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <span className="rounded px-1.5 py-0.5 text-2xs font-mono bg-[var(--bg-elevated)] text-[var(--text-secondary)]">{p.manager}</span>
            {p.stale ? (
              <span className="rounded px-2 py-0.5 text-xs bg-[var(--danger)]/10 text-[var(--danger)]" title={t("scan.deep.patch.staleHelp")}>
                {t("scan.deep.patch.stale")}
              </span>
            ) : (
              <span className="rounded px-2 py-0.5 text-xs bg-[var(--success)]/10 text-[var(--success)]">
                {t("scan.deep.patch.ok")}
              </span>
            )}
            {p.upgradable != null && p.upgradable > 0 && (
              <span className="rounded px-2 py-0.5 text-xs bg-[var(--warning)]/10 text-[var(--warning)]">
                {t("scan.deep.patch.upgradable", { count: String(p.upgradable) })}
              </span>
            )}
          </div>
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
            <Field label={t("scan.deep.patch.lastIndex")}>{date(p.last_index_update)}</Field>
            <Field label={t("scan.deep.patch.lastUpgrade")}>{date(p.last_upgrade)}</Field>
            {p.auto_updates && (
              <Field label={t("scan.deep.patch.auto")}>
                <span className="font-mono">{p.auto_updates}</span>
                {" · "}
                <span className={p.auto_updates_on ? "text-[var(--success)]" : "text-[var(--text-muted)]"}>
                  {t(p.auto_updates_on ? "scan.deep.patch.on" : "scan.deep.patch.off")}
                </span>
              </Field>
            )}
          </dl>
        </div>
      )}
    </SectionCard>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-[var(--text-muted)]">{label}</dt>
      <dd className="text-[var(--text-secondary)]">{children}</dd>
    </>
  );
}
