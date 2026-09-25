// Pure path → crumbs. No React: unit-tested with `node --test src/lib/breadcrumbs.test.ts`.
// NAV_ITEMS is passed in rather than imported: Node's ESM loader won't probe a
// `./constants` import for `.ts`, and tsc forbids the explicit extension here.
// The type-only import below is erased at runtime, so it is safe.
import type { NavItem } from "./constants";

export type Crumb = {
  /** i18n key, or literal text when `raw` */
  label: string;
  raw?: boolean;
  /** absent = plain text (the last crumb, or a prefix with no page such as /atlas) */
  href?: string;
  /** DESIGN_SYSTEM §3: nicknames, slugs and domains render in the mono font */
  mono?: boolean;
  /** Only on the single dynamic crumb: the detail page's own react-query key */
  queryKey?: unknown[];
  pick?: (data: unknown) => string | undefined;
};

type Dyn = { key: string; coerce?: "number"; pick: Crumb["pick"]; mono?: boolean };

// Keyed by parent path. `key` and `coerce` must mirror what each detail page
// passes to useQuery: hosts keys by the slug string, the other five parseInt /
// Number the id. React Query compares keys structurally, so ["service", "12"]
// never hits ["service", 12].
// ponytail: `as any` per entry beats importing six api.ts return types into a pure module.
/* eslint-disable @typescript-eslint/no-explicit-any */
const DYNAMIC: Record<string, Dyn> = {
  "/hosts":      { key: "host",        pick: (d) => (d as any)?.host?.nickname, mono: true },
  "/services":   { key: "service",     coerce: "number", pick: (d) => (d as any)?.service?.nickname, mono: true },
  "/projects":   { key: "project",     coerce: "number", pick: (d) => (d as any)?.project?.name },
  "/dns":        { key: "dns",         coerce: "number", pick: (d) => (d as any)?.dns_record?.domain, mono: true },
  "/requests":   { key: "requests",    coerce: "number", pick: (d) => (d as any)?.request?.title },
  "/atlas/apis": { key: "api-catalog", coerce: "number", pick: (d) => (d as any)?.name },
};
/* eslint-enable @typescript-eslint/no-explicit-any */

// Routes with a page but no sidebar entry, plus the /atlas prefix (a section, not a page).
const EXTRA: Record<string, string> = {
  "/atlas": "nav.atlas",
  "/secrets/trash": "nav.trash",
  "/chamados/forms": "nav.chamadosForms",
};
const NO_PAGE = new Set(["/atlas"]);

export function buildCrumbs(pathname: string, nav: Pick<NavItem, "href" | "label">[]): Crumb[] {
  const segments = pathname.split(/[?#]/)[0].split("/").filter(Boolean);
  if (segments.length === 0) return [{ label: "nav.dashboard" }];

  const crumbs: Crumb[] = [];
  let prefix = "";
  segments.forEach((seg, i) => {
    const parent = prefix;
    prefix += `/${seg}`;
    const last = i === segments.length - 1;
    const crumb: Crumb = { label: seg, raw: true };

    const staticKey = EXTRA[prefix] ?? nav.find((n) => n.href === prefix)?.label;
    const dyn = DYNAMIC[parent];
    if (staticKey) {
      crumb.label = staticKey;
      delete crumb.raw;
    } else if (dyn) {
      crumb.mono = dyn.mono;
      crumb.queryKey = [dyn.key, dyn.coerce ? Number(seg) : seg];
      crumb.pick = dyn.pick;
    }
    if (!last && !NO_PAGE.has(prefix)) crumb.href = prefix;
    crumbs.push(crumb);
  });
  return crumbs;
}

/** Where "Voltar" goes: the nearest earlier crumb that is a page, or undefined
 *  on a top-level page (the button is then disabled). */
export function backTarget(crumbs: Crumb[]): string | undefined {
  for (let i = crumbs.length - 2; i >= 0; i--) if (crumbs[i].href) return crumbs[i].href;
  return undefined;
}
