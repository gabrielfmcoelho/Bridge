"use client";

import { useCallback, useSyncExternalStore } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { NAV_ITEMS } from "@/lib/constants";
import { buildCrumbs, backTarget } from "@/lib/breadcrumbs";
import { ICON_PATHS } from "@/lib/icon-paths";
import Icon from "@/components/ui/Icon";
import Button from "@/components/ui/Button";
import Divider from "@/components/ui/Divider";

export default function Breadcrumbs() {
  const pathname = usePathname();
  const router = useRouter();
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

  // "Voltar" is always here, one step up the crumb trail; on a top-level page
  // there is nowhere to go, so it stays visible but disabled. A single crumb
  // only repeats the page title below it, so the trail needs two.
  const back = backTarget(crumbs);

  return (
    <div className="flex items-center gap-1 min-w-0">
    <Button
      variant="ghost"
      size="sm"
      disabled={!back}
      onClick={() => back && router.push(back)}
      aria-label={t("common.back")}
      title={t("common.back")}
    >
      <Icon path={ICON_PATHS.back} className="w-3.5 h-3.5" />
      <span className="max-md:hidden">{t("common.back")}</span>
    </Button>
    {crumbs.length > 1 && <Divider vertical className="max-md:hidden my-2 mx-1" />}
    {crumbs.length > 1 && (
    <nav aria-label={t("header.breadcrumbs")} className="hidden md:flex min-w-0">
      <ol className="flex items-center gap-1 text-xs min-w-0 overflow-hidden font-display">
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
    )}
    </div>
  );
}
