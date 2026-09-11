"use client";

import Card, { CardIcon } from "@/components/ui/Card";
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
    <Card
      as="button"
      onClick={() => onRequest(offering)}
      style={{ "--i": index } as React.CSSProperties}
      className="stagger-in flex h-full flex-col items-stretch gap-4"
    >
      <div className="flex items-start justify-between gap-3">
        <CardIcon path={REQUEST_TYPE_ICON[offering.request_type]} />
        <span className="rounded-full border border-[var(--border-subtle)] px-2.5 py-1 text-2xs text-[var(--text-muted)]">
          {t(`catalog.requestType.${offering.request_type}`)}
        </span>
      </div>

      <div className="space-y-1.5">
        <h3 className="text-sm font-semibold leading-snug text-[var(--text-primary)] font-display">{offering.name}</h3>
        <p className="line-clamp-2 text-xs leading-relaxed text-[var(--text-muted)]">{offering.description}</p>
      </div>

      {useCases.length > 0 && (
        <div className="space-y-2">
          <p className="text-2xs text-[var(--text-faint)]">{t("catalog.useCases")}</p>
          <ul className="space-y-1.5">
            {useCases.slice(0, 3).map((uc) => (
              <li key={uc} className="flex gap-2 text-xs leading-snug text-[var(--text-secondary)]">
                <span
                  aria-hidden
                  className="mt-[7px] h-1 w-1 shrink-0 rounded-full bg-[var(--text-faint)] transition-colors duration-200 group-hover/card:bg-[var(--card-accent)]"
                />
                {uc}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* mt-auto pins the footer to the bottom so cards in a row share a
          baseline even when descriptions and use-case counts differ. */}
      <div className="mt-auto flex items-center justify-between gap-3 border-t border-[var(--border-subtle)] pt-3.5">
        <span className="truncate text-xs text-[var(--text-faint)]">{offering.category}</span>
        <span className="inline-flex shrink-0 items-center gap-1 rounded-[var(--radius-md)] border border-[var(--border-default)] px-3 py-1.5 text-xs font-medium text-[var(--text-secondary)] transition-colors duration-200 group-hover/card:border-[var(--card-accent)] group-hover/card:bg-[var(--card-accent)]/10 group-hover/card:text-[var(--card-accent)]">
          {t("catalog.request")}
          <Icon
            path={ICON_PATHS.chevronUp}
            className="h-3 w-3 rotate-90 transition-transform duration-200 group-hover/card:translate-x-0.5"
            strokeWidth={2}
          />
        </span>
      </div>
    </Card>
  );
}
