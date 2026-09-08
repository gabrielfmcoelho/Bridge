"use client";

import Icon from "@/components/ui/Icon";
import { ICON_PATHS, REQUEST_TYPE_ICON } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import type { Offering } from "@/lib/types";

interface OfferingCardProps {
  offering: Offering;
  onRequest: (offering: Offering) => void;
  /** Position in the grid, drives the staggered reveal cascade. */
  index: number;
}

export default function OfferingCard({ offering, onRequest, index }: OfferingCardProps) {
  const { t } = useLocale();
  const useCases = offering.use_cases ?? [];

  return (
    <button
      type="button"
      onClick={() => onRequest(offering)}
      style={{ "--i": index } as React.CSSProperties}
      className="stagger-in group flex h-full flex-col items-stretch gap-4 rounded-[var(--radius-lg)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-5 text-left transition-[transform,border-color,box-shadow] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] hover:-translate-y-[3px] hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-md)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent-muted)] focus-visible:border-[var(--accent)] active:translate-y-0 active:scale-[0.99]"
    >
      <div className="flex items-start justify-between gap-3">
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] transition-colors duration-300 group-hover:border-[var(--accent)] group-hover:bg-[var(--accent-muted)] group-hover:text-[var(--accent)]">
          <Icon path={REQUEST_TYPE_ICON[offering.request_type]} className="h-5 w-5" strokeWidth={1.5} />
        </span>
        <span className="rounded-full border border-[var(--border-subtle)] px-2.5 py-1 text-[10px] uppercase tracking-wider text-[var(--text-muted)]">
          {t(`catalog.requestType.${offering.request_type}`)}
        </span>
      </div>

      <div className="space-y-1.5">
        <h3
          className="text-sm font-semibold leading-snug text-[var(--text-primary)]"
          style={{ fontFamily: "var(--font-display)" }}
        >
          {offering.name}
        </h3>
        <p className="line-clamp-2 text-xs leading-relaxed text-[var(--text-muted)]">{offering.description}</p>
      </div>

      {useCases.length > 0 && (
        <div className="space-y-2">
          <p className="text-[10px] uppercase tracking-wider text-[var(--text-faint)]">{t("catalog.useCases")}</p>
          <ul className="space-y-1.5">
            {useCases.slice(0, 3).map((uc) => (
              <li key={uc} className="flex gap-2 text-xs leading-snug text-[var(--text-secondary)]">
                <span aria-hidden className="mt-[7px] h-1 w-1 shrink-0 rounded-full bg-[var(--text-faint)] transition-colors duration-300 group-hover:bg-[var(--accent)]" />
                {uc}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* mt-auto pins the footer to the bottom so cards in a row share a
          baseline even when descriptions and use-case counts differ. */}
      <div className="mt-auto flex items-center justify-between gap-3 border-t border-[var(--border-subtle)] pt-3.5">
        <span className="truncate text-[11px] text-[var(--text-faint)]">{offering.category}</span>
        <span className="inline-flex shrink-0 items-center gap-1 rounded-[var(--radius-md)] border border-[var(--border-default)] px-3 py-1.5 text-xs font-medium text-[var(--text-secondary)] transition-colors duration-300 group-hover:border-[var(--accent)] group-hover:bg-[var(--accent-muted)] group-hover:text-[var(--accent)]">
          {t("catalog.request")}
          <Icon path={ICON_PATHS.chevronUp} className="h-3 w-3 rotate-90 transition-transform duration-300 group-hover:translate-x-0.5" strokeWidth={2} />
        </span>
      </div>
    </button>
  );
}
