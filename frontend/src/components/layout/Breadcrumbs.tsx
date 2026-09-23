"use client";

import { useCallback, useSyncExternalStore } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { NAV_ITEMS } from "@/lib/constants";
import { buildCrumbs } from "@/lib/breadcrumbs";
import { ICON_PATHS } from "@/lib/icon-paths";
import Icon from "@/components/ui/Icon";

export default function Breadcrumbs() {
  const pathname = usePathname();
  const { t } = useLocale();
  const qc = useQueryClient();
  const crumbs = buildCrumbs(pathname ?? "/", NAV_ITEMS);
  const dyn = crumbs.find((c) => c.queryKey);

  // Read the detail page's own cache entry without creating a query observer.
  // useQuery here (even with skipToken / enabled:false) would overwrite the
  // shared Query's options on every Header re-render, and the page's next
  // invalidateQueries would then reject with "Missing queryFn".
  const name = useSyncExternalStore(
    useCallback((cb: () => void) => qc.getQueryCache().subscribe(cb), [qc]),
    () => (dyn ? dyn.pick?.(qc.getQueryData(dyn.queryKey!)) : undefined),
    () => undefined,
  );

  return (
    <nav aria-label={t("header.breadcrumbs")} className="hidden md:flex ml-2 min-w-0">
      <ol className="flex items-center gap-1 text-sm min-w-0 font-display">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          const unresolved = c.raw && !!c.queryKey && !name;
          const text = c.raw ? name ?? c.label : t(c.label);
          const mono = c.mono ? " font-mono" : "";
          return (
            <li key={c.href ?? `${i}-${c.label}`} className="flex items-center gap-1 min-w-0">
              {i > 0 && (
                <span aria-hidden className="flex shrink-0 text-[var(--text-faint)]">
                  <Icon path={ICON_PATHS.chevronRight} className="w-3 h-3" />
                </span>
              )}
              {last ? (
                <span
                  aria-current="page"
                  className={`font-semibold truncate max-w-[28ch] ${unresolved ? "text-[var(--text-faint)]" : "text-[var(--text-primary)]"}${mono}`}
                >
                  {text}
                </span>
              ) : c.href ? (
                <Link href={c.href} className={`text-[var(--text-muted)] hover:text-[var(--text-secondary)] transition-colors${mono}`}>
                  {text}
                </Link>
              ) : (
                <span className={`text-[var(--text-muted)]${mono}`}>{text}</span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
