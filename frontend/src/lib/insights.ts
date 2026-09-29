// Shapes shared by every inventory's KPI catalog (hostInsights, dnsInsights,
// serviceInsights) and dashboard breakdowns. Type-only on purpose: the
// insight modules import it with `import type`, so `node --test` can run them
// without resolving this file.
import type { ICON_PATHS } from "@/lib/icon-paths";

export interface SortConfig {
  field: string;
  direction: "asc" | "desc";
}

/** One KPI: counts something in the current listing; `filter`/`sort` is what
 *  a click applies (a second click clears it). */
export interface Insight<F> {
  key: string;
  label: string;
  /** Name in ICON_PATHS; resolved by the caller, so insight modules stay runtime-import free. */
  icon: keyof typeof ICON_PATHS;
  color: string;
  value: number;
  hint?: string;
  filter?: Partial<F>;
  sort?: SortConfig;
}

/** One bar of a dashboard breakdown. */
export interface BreakdownRow<F> {
  key: string;
  /** Display label; the caller translates `labelKey` rows. */
  label: string;
  labelKey?: boolean;
  count: number;
  filter?: Partial<F>;
}
